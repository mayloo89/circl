package chat

import (
	"context"
	"encoding/json"
	"sync/atomic"

	"github.com/redis/go-redis/v9"
)

const redisChannelPrefix = "chat:room:"

// ClientInfo holds the participant details exposed via RoomParticipants.
type ClientInfo struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	IsGuest     bool   `json:"is_guest,omitempty"`
}

// participantsReq is a synchronous query sent on Hub.participantsQ.
type participantsReq struct {
	roomID string
	reply  chan []ClientInfo
}

// broadcastMsg carries a message payload destined for all local clients of
// a given room.  Sent to Hub.broadcast by the Redis listener goroutines so
// that all channel writes happen through the single Run loop.
type broadcastMsg struct {
	roomID string
	data   []byte
}

// Hub manages WebSocket client connections and fans out messages via Redis
// Pub/Sub so that the system scales horizontally across multiple server
// instances.
//
// All mutations to the rooms and pubsubs maps are serialised through the Run
// goroutine, which reads from three channels: register, unregister, broadcast.
// This design avoids mutexes on the hot path and mirrors the well-known
// gorilla/websocket chat example pattern.
type Hub struct {
	rdb   *redis.Client
	bgCtx context.Context // background context used for fire-and-forget publishes

	register      chan *Client
	unregister    chan *Client
	broadcast     chan broadcastMsg
	participantsQ chan participantsReq

	// These fields are only accessed from the Run goroutine.
	rooms   map[string]map[*Client]struct{}
	pubsubs map[string]*redis.PubSub

	activeConns atomic.Int64
}

// ActiveConns returns the number of currently connected WebSocket clients.
// Safe to call from any goroutine; used by the Prometheus metrics collector.
func (h *Hub) ActiveConns() int64 { return h.activeConns.Load() }

// NewHub creates a Hub backed by the given Redis client.
func NewHub(rdb *redis.Client) *Hub {
	return &Hub{
		rdb:           rdb,
		bgCtx:         context.Background(),
		register:      make(chan *Client, 16),
		unregister:    make(chan *Client, 16),
		broadcast:     make(chan broadcastMsg, 256),
		participantsQ: make(chan participantsReq, 4),
		rooms:         make(map[string]map[*Client]struct{}),
		pubsubs:       make(map[string]*redis.PubSub),
	}
}

// Run processes register, unregister, broadcast, and participants-query events.
// It must be started in a goroutine and runs until ctx is cancelled.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case client := <-h.register:
			h.addClient(ctx, client)

		case client := <-h.unregister:
			h.removeClient(client)

		case msg := <-h.broadcast:
			h.deliver(msg.roomID, msg.data)

		case req := <-h.participantsQ:
			clients := h.rooms[req.roomID]
			seen := make(map[string]struct{}, len(clients))
			infos := make([]ClientInfo, 0, len(clients))
			for c := range clients {
				if _, ok := seen[c.userID]; ok {
					continue
				}
				seen[c.userID] = struct{}{}
				infos = append(infos, ClientInfo{
					UserID:      c.userID,
					Username:    c.username,
					DisplayName: c.displayName,
					AvatarURL:   c.avatarURL,
					IsGuest:     c.isGuest,
				})
			}
			req.reply <- infos

		case <-ctx.Done():
			for _, clients := range h.rooms {
				for c := range clients {
					close(c.send)
				}
			}
			for _, ps := range h.pubsubs {
				_ = ps.Close()
			}
			return
		}
	}
}

// Publish sends data to all subscribers of roomID across all server instances
// by publishing to the corresponding Redis channel.
func (h *Hub) Publish(ctx context.Context, roomID string, data []byte) error {
	return h.rdb.Publish(ctx, redisChannelPrefix+roomID, data).Err()
}

// RoomParticipants returns the ClientInfo for every locally connected client in
// roomID. The call blocks until the Run goroutine responds or ctx is cancelled.
func (h *Hub) RoomParticipants(ctx context.Context, roomID string) []ClientInfo {
	reply := make(chan []ClientInfo, 1)
	select {
	case h.participantsQ <- participantsReq{roomID: roomID, reply: reply}:
	case <-ctx.Done():
		return nil
	}
	select {
	case infos := <-reply:
		return infos
	case <-ctx.Done():
		return nil
	}
}

// addClient registers a client and subscribes to its room's Redis channel if
// this is the first local client for that room.
func (h *Hub) addClient(ctx context.Context, client *Client) {
	if h.rooms[client.roomID] == nil {
		h.rooms[client.roomID] = make(map[*Client]struct{})
		ps := h.rdb.Subscribe(ctx, redisChannelPrefix+client.roomID)
		// Wait for the SUBSCRIBE acknowledgment from Redis before storing the
		// pubsub and returning.  Without this there is a race: mustRegister
		// (in tests) or a caller can publish before listenRedis has called
		// ps.Channel(), dropping the message.  After Receive returns the
		// subscription is live in Redis; go-redis buffers any messages that
		// arrive before Channel() is called, so nothing is lost.
		if _, err := ps.Receive(ctx); err != nil {
			_ = ps.Close()
			return
		}
		h.pubsubs[client.roomID] = ps
		go h.listenRedis(ctx, client.roomID, ps)
	}
	h.rooms[client.roomID][client] = struct{}{}
	h.activeConns.Add(1)

	if client.isChannel || client.isPublic {
		// Public rooms expose registered users by display name + badge only,
		// never their @handle — guests must not be able to harvest usernames.
		username := client.username
		if client.isPublic {
			username = ""
		}
		data, _ := json.Marshal(map[string]any{
			"event":        "participant_join",
			"user_id":      client.userID,
			"username":     username,
			"display_name": client.displayName,
			"avatar_url":   client.avatarURL,
			"is_guest":     client.isGuest,
		})
		go h.rdb.Publish(h.bgCtx, redisChannelPrefix+client.roomID, data) //nolint:errcheck
	}
}

// removeClient unregisters a client and tears down the Redis subscription
// when the last local client leaves a room.
func (h *Hub) removeClient(client *Client) {
	clients, ok := h.rooms[client.roomID]
	if !ok {
		return
	}
	if _, exists := clients[client]; !exists {
		return
	}
	delete(clients, client)
	close(client.send)
	h.activeConns.Add(-1)

	if client.isChannel || client.isPublic {
		data, _ := json.Marshal(map[string]any{
			"event":    "participant_leave",
			"user_id":  client.userID,
			"is_guest": client.isGuest,
		})
		go h.rdb.Publish(h.bgCtx, redisChannelPrefix+client.roomID, data) //nolint:errcheck
	}

	if len(clients) == 0 {
		if ps, ok := h.pubsubs[client.roomID]; ok {
			_ = ps.Close()
			delete(h.pubsubs, client.roomID)
		}
		delete(h.rooms, client.roomID)
	}
}

// deliver sends data to all locally connected clients for roomID.
// Slow clients (full send buffer) are removed.
//
// Privacy gate: when the frame's "event" field is "typing" or "read_receipt",
// individual clients with the matching opt-out flag are skipped (symmetric
// hide_typing_indicator / hide_read_receipts).
func (h *Hub) deliver(roomID string, data []byte) {
	clients, ok := h.rooms[roomID]
	if !ok {
		return
	}
	event := frameEvent(data)
	for c := range clients {
		switch event {
		case "typing":
			if c.hideTyping {
				continue
			}
		case "read_receipt":
			if c.hideReadReceipts {
				continue
			}
		}
		select {
		case c.send <- data:
		default:
			// Client is not reading fast enough; disconnect it.
			delete(clients, c)
			close(c.send)
		}
	}
}

// frameEvent extracts the "event" field from a JSON frame without parsing
// the full payload. Returns an empty string if the frame is not valid JSON
// or has no "event" key.
func frameEvent(data []byte) string {
	var meta struct {
		Event string `json:"event"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return ""
	}
	return meta.Event
}

// listenRedis forwards messages from the Redis channel into the broadcast
// channel so they are delivered via the single Run goroutine.
func (h *Hub) listenRedis(ctx context.Context, roomID string, ps *redis.PubSub) {
	ch := ps.Channel()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			select {
			case h.broadcast <- broadcastMsg{roomID: roomID, data: []byte(msg.Payload)}:
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
