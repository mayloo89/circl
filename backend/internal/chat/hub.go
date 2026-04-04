package chat

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

const redisChannelPrefix = "chat:room:"

// ClientInfo holds the participant details exposed via RoomParticipants.
type ClientInfo struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
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
}

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
			infos := make([]ClientInfo, 0, len(clients))
			for c := range clients {
				infos = append(infos, ClientInfo{
					UserID:      c.userID,
					DisplayName: c.displayName,
					AvatarURL:   c.avatarURL,
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
		h.pubsubs[client.roomID] = ps
		go h.listenRedis(ctx, client.roomID, ps)
	}
	h.rooms[client.roomID][client] = struct{}{}

	if client.isChannel {
		data, _ := json.Marshal(map[string]any{
			"event":        "participant_join",
			"user_id":      client.userID,
			"display_name": client.displayName,
			"avatar_url":   client.avatarURL,
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

	if client.isChannel {
		data, _ := json.Marshal(map[string]any{
			"event":   "participant_leave",
			"user_id": client.userID,
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
func (h *Hub) deliver(roomID string, data []byte) {
	clients, ok := h.rooms[roomID]
	if !ok {
		return
	}
	for c := range clients {
		select {
		case c.send <- data:
		default:
			// Client is not reading fast enough; disconnect it.
			delete(clients, c)
			close(c.send)
		}
	}
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
