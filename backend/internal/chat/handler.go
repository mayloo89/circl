package chat

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/token"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	maxMsgSize = 4096
)

// upgrader accepts WebSocket connections from any origin.
// Origin validation is handled at the CORS middleware level.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(_ *http.Request) bool { return true },
}

// Client represents a single WebSocket connection from an authenticated user.
type Client struct {
	hub         *Hub
	conn        *websocket.Conn
	send        chan []byte
	userID      string
	roomID      string
	displayName string
}

// typingFrame is the WS frame broadcast to room members when a user is typing.
type typingFrame struct {
	Event       string `json:"event"`
	UserID      string `json:"user_id"`
	RoomID      string `json:"room_id"`
	DisplayName string `json:"display_name"`
}

// serverMessage is the JSON envelope sent from the server to connected clients.
// event is "new_message" for incoming messages or "message_deleted" for deletions.
type serverMessage struct {
	Event           string     `json:"event"`
	Type            string     `json:"type,omitempty"`
	ID              string     `json:"id"`
	RoomID          string     `json:"room_id"`
	SenderID        string     `json:"sender_id,omitempty"`
	SenderName      string     `json:"sender_name,omitempty"`
	SenderAvatarURL string     `json:"sender_avatar_url,omitempty"`
	Content         string     `json:"content,omitempty"`
	ThumbnailURL    string     `json:"thumbnail_url,omitempty"`
	ViewOnce        bool       `json:"view_once,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at,omitempty"`
}

// clientMessage is the JSON envelope received from a connected client.
// Type must be "message" (plain text) or "attachment" (uploaded file URL).
// MimeType is required when Type is "attachment".
// ViewOnce and TTL are mutually exclusive ephemeral modes.
type clientMessage struct {
	Type     string `json:"type"`
	Content  string `json:"content"`
	MimeType string `json:"mime_type,omitempty"`
	UploadID string `json:"upload_id,omitempty"`
	ViewOnce bool   `json:"view_once,omitempty"`
	TTL      string `json:"ttl,omitempty"` // "1h" | "24h" | "7d"
}

// HandlerConfig holds optional callbacks for the REST chat handler.
type HandlerConfig struct {
	// NotifyMessageDeleted is called after a message is physically deleted
	// so the hub can broadcast a message_deleted event to all room members.
	NotifyMessageDeleted func(roomID, messageID string)
	// NotifyRoomRead is called after a user marks a room as read so the hub
	// can broadcast a read_receipt event to all room members.
	NotifyRoomRead func(roomID, userID string, readAt time.Time)
	// DeleteFiles is called with storage keys to remove from object storage
	// after a message's attached files have been unlinked from the database.
	DeleteFiles func(ctx context.Context, keys []string)
	// ReadFile opens a storage object by key for reading. Used by the view-once
	// handler to stream the file content to the client before deleting it,
	// eliminating the race between file serving and file deletion.
	ReadFile func(ctx context.Context, key string) (io.ReadCloser, error)
}

// resolveMessageType maps a client-supplied frame type and MIME type to the
// internal MessageType* constant. It returns false for unknown frame types.
func resolveMessageType(clientType, mimeType string) (string, bool) {
	switch clientType {
	case "message":
		return MessageTypeText, true
	case "attachment":
		switch {
		case strings.HasPrefix(mimeType, "image/"):
			return MessageTypeImage, true
		case strings.HasPrefix(mimeType, "video/"):
			return MessageTypeVideo, true
		default:
			return MessageTypeFile, true
		}
	default:
		return "", false
	}
}

// NewHandler returns a chi router with the REST chat routes.
// It must be mounted behind the requireAuth middleware so that
// middleware.UserIDFromContext is available in every handler.
// An optional HandlerConfig may be provided to wire deletion notifications
// and object storage cleanup.
func NewHandler(svc Manager, cfg ...HandlerConfig) http.Handler {
	var c HandlerConfig
	if len(cfg) > 0 {
		c = cfg[0]
	}

	r := chi.NewRouter()
	r.Post("/rooms/dm", getDMHandler(svc))
	r.Post("/rooms", createGroupHandler(svc))
	r.Get("/rooms", listRoomsHandler(svc))
	r.Get("/rooms/{id}/messages", listMessagesHandler(svc))
	r.Put("/rooms/{id}/read", markReadHandler(svc, c))
	r.Post("/rooms/{id}/messages/{msgID}/view", viewMessageHandler(svc, c))

	return r
}

// NewWSHandler returns the WebSocket handler for a single room.
// It must be registered outside the requireAuth middleware group because the
// browser WebSocket API does not support custom request headers; the JWT is
// passed as a ?token= query parameter instead and validated here.
//
// notifyNewMessage, if non-nil, is called for each non-sender room member
// after a message is saved, allowing callers to push real-time SSE badges.
func NewWSHandler(svc Manager, hub *Hub, jwtSecret string, notifyNewMessage func(recipientID, roomID string)) http.HandlerFunc {
	return wsHandler(svc, hub, jwtSecret, notifyNewMessage)
}

// getDMHandler returns (or creates) the direct-message room between the
// authenticated user and a specified peer.
//
// POST /chat/rooms/dm
// Body: {"peer_id": "<uuid>"}
func getDMHandler(svc Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		var body struct {
			PeerID string `json:"peer_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PeerID == "" {
			http.Error(w, `{"error":"peer_id is required"}`, http.StatusBadRequest)
			return
		}

		room, err := svc.GetOrCreateDM(r.Context(), userID, body.PeerID)
		if err != nil {
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(room) //nolint:errcheck
	}
}

// createGroupHandler creates a named group room.
//
// POST /chat/rooms
// Body: {"name": "...", "member_ids": ["<uuid>", ...]}
func createGroupHandler(svc Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		var body struct {
			Name      string   `json:"name"`
			MemberIDs []string `json:"member_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
			return
		}

		room, err := svc.CreateGroup(r.Context(), userID, body.Name, body.MemberIDs)
		if err != nil {
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(room) //nolint:errcheck
	}
}

// listRoomsHandler returns all rooms the authenticated user belongs to.
//
// GET /chat/rooms
func listRoomsHandler(svc Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		rooms, err := svc.ListRooms(r.Context(), userID)
		if err != nil {
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rooms) //nolint:errcheck
	}
}

// listMessagesHandler returns paginated messages for a room.
// View-once messages not sent by the requester have their content masked.
//
// GET /chat/rooms/{id}/messages?before=<RFC3339>&limit=<int>
func listMessagesHandler(svc Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		roomID := chi.URLParam(r, "id")

		member, err := svc.IsMember(r.Context(), roomID, userID)
		if err != nil || !member {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}

		var before *time.Time
		if raw := r.URL.Query().Get("before"); raw != "" {
			t, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				http.Error(w, `{"error":"invalid before timestamp"}`, http.StatusBadRequest)
				return
			}
			before = &t
		}

		limit := 50
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 {
				limit = n
			}
		}

		msgs, err := svc.ListMessages(r.Context(), roomID, before, limit)
		if err != nil {
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		// Mask view_once content for non-senders: the recipient must call
		// POST /rooms/{id}/messages/{msgID}/view to fetch the real content.
		for i := range msgs {
			if msgs[i].ViewOnce && msgs[i].SenderID != userID {
				msgs[i].Content = ""
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(msgs) //nolint:errcheck
	}
}

// markReadHandler marks all messages in a room as read for the authenticated
// user and broadcasts a read_receipt WS event to all room members.
//
// PUT /chat/rooms/{id}/read
func markReadHandler(svc Manager, cfg HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		roomID := chi.URLParam(r, "id")

		readAt, err := svc.MarkRead(r.Context(), roomID, userID)
		if err != nil {
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		if cfg.NotifyRoomRead != nil {
			cfg.NotifyRoomRead(roomID, userID, readAt)
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// viewMessageHandler delivers the content of a view-once message to the
// requester and, once all non-sender members have viewed it, physically
// deletes the message and its attached files.
//
// POST /chat/rooms/{id}/messages/{msgID}/view
func viewMessageHandler(svc Manager, cfg HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		roomID := chi.URLParam(r, "id")
		msgID := chi.URLParam(r, "msgID")

		member, err := svc.IsMember(r.Context(), roomID, userID)
		if err != nil || !member {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}

		msg, keys, err := svc.ViewOnceMessage(r.Context(), msgID, roomID, userID)
		if errors.Is(err, ErrNotFound) {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrForbidden) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		if err != nil {
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		// For image/video view-once messages, stream the file binary directly
		// to the client so the browser has the data in hand before we delete
		// the file from storage. This eliminates the race between the client
		// loading the media and the goroutine removing it from the bucket.
		if len(keys) > 0 && cfg.ReadFile != nil && (msg.Type == MessageTypeImage || msg.Type == MessageTypeVideo) {
			rc, readErr := cfg.ReadFile(r.Context(), keys[0])
			if readErr == nil {
				defer rc.Close() //nolint:errcheck
				ct := mime.TypeByExtension(filepath.Ext(keys[0]))
				if ct == "" {
					ct = "application/octet-stream"
				}
				w.Header().Set("Content-Type", ct)
				io.Copy(w, rc) //nolint:errcheck
				go func() {
					if cfg.DeleteFiles != nil {
						cfg.DeleteFiles(context.Background(), keys)
					}
					if cfg.NotifyMessageDeleted != nil {
						cfg.NotifyMessageDeleted(roomID, msgID)
					}
				}()
				return
			}
		}

		// For text messages (or if ReadFile is unavailable), return JSON.
		// No streaming race exists here, so cleanup runs synchronously.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(msg) //nolint:errcheck

		if len(keys) > 0 {
			if cfg.DeleteFiles != nil {
				cfg.DeleteFiles(r.Context(), keys)
			}
			if cfg.NotifyMessageDeleted != nil {
				cfg.NotifyMessageDeleted(roomID, msgID)
			}
		}
	}
}

// wsHandler upgrades the connection to WebSocket and starts the client pumps.
// Auth is performed via a ?token= query parameter because the browser
// WebSocket API does not support custom headers.
//
// GET /chat/rooms/{id}/ws?token=<jwt>
func wsHandler(svc Manager, hub *Hub, jwtSecret string, notifyNewMessage func(recipientID, roomID string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := r.URL.Query().Get("token")
		if tok == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		claims, err := token.Validate(tok, jwtSecret)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		userID := claims.Subject

		roomID := chi.URLParam(r, "id")

		member, err := svc.IsMember(r.Context(), roomID, userID)
		if err != nil || !member {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		displayName, _ := svc.GetDisplayName(r.Context(), userID)

		client := &Client{
			hub:         hub,
			conn:        conn,
			send:        make(chan []byte, 256),
			userID:      userID,
			roomID:      roomID,
			displayName: displayName,
		}

		hub.register <- client

		go client.writePump()
		go client.readPump(svc, notifyNewMessage)
	}
}

// readPump pumps messages from the WebSocket connection to the hub.
// It runs in a dedicated goroutine per connection.
func (c *Client) readPump(svc Manager, notifyNewMessage func(recipientID, roomID string)) {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMsgSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait)) //nolint:errcheck
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait)) //nolint:errcheck
		return nil
	})

	var lastTypingBroadcast time.Time

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var in clientMessage
		if err := json.Unmarshal(raw, &in); err != nil {
			continue
		}

		if in.Type == "typing" {
			if time.Since(lastTypingBroadcast) >= 2*time.Second {
				lastTypingBroadcast = time.Now()
				if data, err := json.Marshal(typingFrame{
					Event:       "typing",
					UserID:      c.userID,
					RoomID:      c.roomID,
					DisplayName: c.displayName,
				}); err == nil {
					_ = c.hub.Publish(context.Background(), c.roomID, data)
				}
			}
			continue
		}

		msgType, ok := resolveMessageType(in.Type, in.MimeType)
		if !ok || in.Content == "" {
			continue
		}
		// Attachment messages must carry an upload_id so the file can be
		// linked to the message record and cleaned up on deletion.
		if in.Type == "attachment" && in.UploadID == "" {
			continue
		}

		params := SaveMessageParams{
			RoomID:   c.roomID,
			SenderID: c.userID,
			Type:     msgType,
			Content:  in.Content,
			UploadID: in.UploadID,
			ViewOnce: in.ViewOnce,
		}
		if in.TTL != "" {
			d, err := ParseTTL(in.TTL)
			if err != nil {
				continue // reject unknown TTL values
			}
			t := time.Now().Add(d)
			params.ExpiresAt = &t
		}

		ctx := context.Background()
		msg, err := svc.SaveMessage(ctx, params)
		if err != nil {
			continue
		}

		// View-once messages are broadcast with masked content; the recipient
		// must call POST /rooms/{id}/messages/{msgID}/view to read them.
		content := msg.Content
		if msg.ViewOnce {
			content = ""
		}

		data, err := json.Marshal(serverMessage{
			Event:           "new_message",
			Type:            msg.Type,
			ID:              msg.ID,
			RoomID:          msg.RoomID,
			SenderID:        msg.SenderID,
			SenderName:      msg.SenderName,
			SenderAvatarURL: msg.SenderAvatarURL,
			Content:         content,
			ThumbnailURL:    msg.ThumbnailURL,
			ViewOnce:        msg.ViewOnce,
			ExpiresAt:       msg.ExpiresAt,
			CreatedAt:       msg.CreatedAt,
		})
		if err != nil {
			continue
		}

		_ = c.hub.Publish(ctx, c.roomID, data)

		// Notify non-sender members so they can show an unread badge.
		if notifyNewMessage != nil {
			if members, err := svc.ListMembers(ctx, c.roomID); err == nil {
				for _, uid := range members {
					if uid != c.userID {
						notifyNewMessage(uid, c.roomID)
					}
				}
			}
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection.
// It also sends periodic ping frames to keep the connection alive.
// It runs in a dedicated goroutine per connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait)) //nolint:errcheck
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{}) //nolint:errcheck
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait)) //nolint:errcheck
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
