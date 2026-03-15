package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
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
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	userID string
	roomID string
}

// serverMessage is the JSON envelope sent from the server to connected clients.
type serverMessage struct {
	Type       string    `json:"type"`
	ID         string    `json:"id"`
	RoomID     string    `json:"room_id"`
	SenderID   string    `json:"sender_id"`
	SenderName string    `json:"sender_name"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

// clientMessage is the JSON envelope received from a connected client.
type clientMessage struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// NewHandler returns a chi router with the REST chat routes.
// It must be mounted behind the requireAuth middleware so that
// middleware.UserIDFromContext is available in every handler.
func NewHandler(svc Manager) http.Handler {
	r := chi.NewRouter()

	r.Post("/rooms/dm", getDMHandler(svc))
	r.Post("/rooms", createGroupHandler(svc))
	r.Get("/rooms", listRoomsHandler(svc))
	r.Get("/rooms/{id}/messages", listMessagesHandler(svc))
	r.Put("/rooms/{id}/read", markReadHandler(svc))

	return r
}

// NewWSHandler returns the WebSocket handler for a single room.
// It must be registered outside the requireAuth middleware group because the
// browser WebSocket API does not support custom request headers; the JWT is
// passed as a ?token= query parameter instead and validated here.
func NewWSHandler(svc Manager, hub *Hub, jwtSecret string) http.HandlerFunc {
	return wsHandler(svc, hub, jwtSecret)
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

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(msgs) //nolint:errcheck
	}
}

// markReadHandler marks all messages in a room as read for the authenticated user.
//
// PUT /chat/rooms/{id}/read
func markReadHandler(svc Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		roomID := chi.URLParam(r, "id")

		if err := svc.MarkRead(r.Context(), roomID, userID); err != nil {
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// wsHandler upgrades the connection to WebSocket and starts the client pumps.
// Auth is performed via a ?token= query parameter because the browser
// WebSocket API does not support custom headers.
//
// GET /chat/rooms/{id}/ws?token=<jwt>
func wsHandler(svc Manager, hub *Hub, jwtSecret string) http.HandlerFunc {
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

		client := &Client{
			hub:    hub,
			conn:   conn,
			send:   make(chan []byte, 256),
			userID: userID,
			roomID: roomID,
		}

		hub.register <- client

		go client.writePump()
		go client.readPump(svc)
	}
}

// readPump pumps messages from the WebSocket connection to the hub.
// It runs in a dedicated goroutine per connection.
func (c *Client) readPump(svc Manager) {
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

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var in clientMessage
		if err := json.Unmarshal(raw, &in); err != nil {
			continue
		}

		if in.Type == "message" && in.Content != "" {
			msg, err := svc.SaveMessage(context.Background(), c.roomID, c.userID, MessageTypeText, in.Content)
			if err != nil {
				continue
			}

			data, err := json.Marshal(serverMessage{
				Type:       "message",
				ID:         msg.ID,
				RoomID:     msg.RoomID,
				SenderID:   msg.SenderID,
				SenderName: msg.SenderName,
				Content:    msg.Content,
				CreatedAt:  msg.CreatedAt,
			})
			if err != nil {
				continue
			}

			_ = c.hub.Publish(context.Background(), c.roomID, data)
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
