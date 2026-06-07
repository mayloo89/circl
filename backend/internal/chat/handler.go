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
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/mayloo89/circl/backend/internal/apierror"
	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/ratelimit"
	"github.com/mayloo89/circl/backend/internal/wsticket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	maxMsgSize = 4096
)

// Client represents a single WebSocket connection from an authenticated user.
type Client struct {
	hub               *Hub
	conn              *websocket.Conn
	send              chan []byte
	userID            string
	roomID            string
	username          string
	displayName       string
	avatarURL         string
	isChannel         bool
	isPublic          bool
	isGuest           bool
	isServiceProvider func(ctx context.Context, userID string) bool
	guestMsgLimiter   *ratelimit.RedisLimiter
	guestMsgRate      int
	guestMsgWindow    time.Duration
	isBlockedInRoom   func(ctx context.Context, senderID, roomID string) bool
	isMutedInRoom     func(ctx context.Context, roomID, userID string) bool
	// hideReadReceipts and hideTyping are populated at WS connect from the
	// user's profile preferences. They are used both as the emit gate (for
	// frames originating from this client) and the receive gate (in
	// hub.deliver). Pref changes take effect on the next reconnect.
	hideReadReceipts bool
	hideTyping       bool
	// ctx carries the OpenTelemetry session span for this connection.
	// The span is ended in readPump's defer when the connection closes.
	ctx context.Context
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
	Redacted        bool       `json:"redacted,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitzero"`
	CreatedAt       time.Time  `json:"created_at,omitzero"`
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

// HandlerConfig holds optional callbacks and dependencies for the REST chat handler.
type HandlerConfig struct {
	// Hub, if set, is used to query active WebSocket participants for channel rooms.
	Hub *Hub
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
	// IsBlocked, if set, is checked before creating a DM room and before
	// delivering WebSocket messages. Returns true if either user has blocked
	// the other, suppressing the operation.
	IsBlocked func(ctx context.Context, userA, userB string) (bool, error)
	// IsBlockedInRoom, if set, is checked before saving a WebSocket message.
	// Returns true if the sender is blocked by any room member, suppressing
	// the message silently.
	IsBlockedInRoom func(ctx context.Context, senderID, roomID string) bool
	// AreContacts, if set, returns true if the two users have a mutual
	// accepted contact relationship. Used to decide whether DM messages
	// should be scanned for external contact info.
	AreContacts func(ctx context.Context, userA, userB string) (bool, error)
	// IsExemptSender, if set, returns true if the sender should bypass
	// contact-info redaction (e.g. verified service accounts). Always
	// returns false when nil.
	IsExemptSender func(ctx context.Context, senderID string) bool
	// IsServiceProvider, if set, returns true if the user is a verified
	// service-provider profile. Such users are blocked from posting in
	// public rooms (marketplace integrity). Always returns false when nil.
	IsServiceProvider func(ctx context.Context, userID string) bool
	// GuestMsgLimiter, if set, rate-limits guest message sends in public rooms.
	// Keyed by guest session ID + client IP for stricter per-guest enforcement.
	GuestMsgLimiter *ratelimit.RedisLimiter
	// GuestMsgRate is the maximum number of messages a guest may send per window.
	GuestMsgRate int
	// GuestMsgWindow is the sliding window duration for guest message rate limiting.
	GuestMsgWindow time.Duration
	// AllowedOrigins is the list of origins permitted to open WebSocket
	// connections. When empty, all origins are allowed (development only).
	AllowedOrigins []string
	// PrivacyResolver, if set, returns the user's typing-indicator and
	// read-receipt opt-out flags. Called once at WS connect and on each
	// REST-initiated read_receipt to decide whether to publish. When nil
	// the gate is open (legacy behavior).
	PrivacyResolver func(ctx context.Context, userID string) (PrivacyFlags, error)

	// --- In-room moderation (public rooms only) ---

	// KickFromRoom, if set, disconnects a participant from a room and sends
	// them a "kicked" WS event. Called by the admin kick endpoint.
	KickFromRoom func(roomID, targetID string)
	// GetGuestSession, if set, retrieves a guest session by ID so the kick
	// handler can look up the IP hash for follow-up IP banning.
	// Returns the session's IP hash; empty string if unknown.
	GetGuestSession func(ctx context.Context, sessionID string) (ipHash string, err error)
	// BanRoomIP, if set, records a per-room IP ban in Redis.
	BanRoomIP func(ctx context.Context, roomID, ipHash string, ttl time.Duration) error
	// IsMutedInRoom, if set, returns true when the user is currently muted in
	// the given room. Checked in readPump before saving each message.
	IsMutedInRoom func(ctx context.Context, roomID, userID string) bool
	// MuteInRoom, if set, stores a per-room mute in Redis with the given TTL.
	MuteInRoom func(ctx context.Context, roomID, userID string, ttl time.Duration) error
	// UnmuteInRoom, if set, removes a per-room mute.
	UnmuteInRoom func(ctx context.Context, roomID, userID string) error
	// ScheduleUnmute, if set, arranges for a you_are_unmuted frame to be
	// delivered to targetID in roomID after ttl elapses. Used to clear the
	// client-side muted state without requiring a reconnect.
	ScheduleUnmute func(roomID, targetID string, ttl time.Duration)
}

// PrivacyFlags are the chat-relevant subset of a user's privacy preferences.
// All fields default to false (no hiding).
type PrivacyFlags struct {
	HideReadReceipts bool
	HideTyping       bool
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
	r.Post("/rooms/dm", getDMHandler(svc, c))
	r.Post("/rooms", createGroupHandler(svc))
	r.Get("/rooms", listRoomsHandler(svc))
	r.Get("/rooms/{id}", getRoomHandler(svc, c))
	r.Put("/rooms/{id}", updateGroupHandler(svc))
	r.Get("/rooms/{id}/members", listGroupMembersHandler(svc, c))
	r.Post("/rooms/{id}/members", addGroupMemberHandler(svc))
	r.Delete("/rooms/{id}/members/{userID}", removeGroupMemberHandler(svc))
	r.Get("/rooms/{id}/messages", listMessagesHandler(svc))
	r.Put("/rooms/{id}/read", markReadHandler(svc, c))
	r.Post("/rooms/{id}/messages/{msgID}/view", viewMessageHandler(svc, c))

	// Public channel routes (admin only)
	r.Get("/channels", listChannelsHandler(svc, c))
	r.Post("/channels", func(w http.ResponseWriter, r *http.Request) {
		middleware.RequireAdmin(createChannelHandler(svc)).ServeHTTP(w, r)
	})

	// Public room routes
	r.Get("/public-rooms", listPublicRoomsHandler(svc, c))

	// In-room moderation — admin only
	r.Post("/rooms/{id}/mod/kick", func(w http.ResponseWriter, r *http.Request) {
		middleware.RequireAdmin(modKickHandler(svc, c)).ServeHTTP(w, r)
	})
	r.Post("/rooms/{id}/mod/mute", func(w http.ResponseWriter, r *http.Request) {
		middleware.RequireAdmin(modMuteHandler(c)).ServeHTTP(w, r)
	})
	r.Delete("/rooms/{id}/mod/mute/{targetID}", func(w http.ResponseWriter, r *http.Request) {
		middleware.RequireAdmin(modUnmuteHandler(c)).ServeHTTP(w, r)
	})

	return r
}

// NewWSHandler returns the WebSocket handler for a single room.
// It must be registered outside the requireAuth middleware group because the
// browser WebSocket API does not support custom request headers; a short-lived
// ticket (obtained via POST /ws-ticket) is passed as a ?ticket= query param.
//
// notifyNewMessage, if non-nil, is called for each non-sender room member
// after a message is saved, allowing callers to push real-time SSE badges.
// An optional HandlerConfig may be supplied to wire block-checking callbacks.
func NewWSHandler(svc Manager, hub *Hub, tickets wsticket.Redeemer, notifyNewMessage func(recipientID, roomID string), cfg ...HandlerConfig) http.HandlerFunc {
	var c HandlerConfig
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return wsHandler(svc, hub, tickets, notifyNewMessage, c)
}

// getDMHandler returns (or creates) the direct-message room between the
// authenticated user and a specified peer.
//
// POST /chat/rooms/dm
// Body: {"peer_id": "<uuid>"}
func getDMHandler(svc Manager, cfg HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}

		var body struct {
			PeerID string `json:"peer_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PeerID == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "peer_id is required")
			return
		}

		if cfg.IsBlocked != nil {
			blocked, err := cfg.IsBlocked(r.Context(), userID, body.PeerID)
			if err != nil {
				apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
				return
			}
			if blocked {
				apierror.Write(w, http.StatusForbidden, apierror.CodeForbidden, "forbidden")
				return
			}
		}

		room, err := svc.GetOrCreateDM(r.Context(), userID, body.PeerID)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}

		resp := struct {
			*Room
			AreAcceptedContacts bool `json:"are_accepted_contacts"`
		}{Room: room}

		if cfg.AreContacts != nil {
			accepted, accErr := cfg.AreContacts(r.Context(), userID, body.PeerID)
			if accErr == nil {
				resp.AreAcceptedContacts = accepted
			}
		}

		apierror.WriteJSON(w, http.StatusOK, resp)
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
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}

		var body struct {
			Name      string   `json:"name"`
			MemberIDs []string `json:"member_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeNameRequired, "name is required")
			return
		}

		room, err := svc.CreateGroup(r.Context(), userID, body.Name, body.MemberIDs)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}

		apierror.WriteJSON(w, http.StatusCreated, room)
	}
}

// listRoomsHandler returns all rooms the authenticated user belongs to.
//
// getRoomHandler returns the room record for a given ID.
// Channels are accessible to any authenticated user; DM and group rooms
// require the caller to be a member.
//
// GET /chat/rooms/{id}
func getRoomHandler(svc Manager, cfg HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		roomID := chi.URLParam(r, "id")
		room, err := svc.GetRoom(r.Context(), roomID)
		if err != nil {
			apierror.Write(w, http.StatusNotFound, apierror.CodeRoomNotFound, "room not found")
			return
		}
		if room.Type != RoomTypeChannel && room.Type != RoomTypePublic {
			member, err := svc.IsMember(r.Context(), roomID, userID)
			if err != nil || !member {
				apierror.Write(w, http.StatusForbidden, apierror.CodeForbidden, "forbidden")
				return
			}
		}

		if room.Type == RoomTypeDM && cfg.AreContacts != nil {
			peerID, peerErr := svc.GetDMPeerID(r.Context(), roomID, userID)
			if peerErr == nil {
				accepted, accErr := cfg.AreContacts(r.Context(), userID, peerID)
				if accErr == nil {
					resp := struct {
						*Room
						AreAcceptedContacts bool `json:"are_accepted_contacts"`
					}{Room: room, AreAcceptedContacts: accepted}
					apierror.WriteJSON(w, http.StatusOK, resp)
					return
				}
			}
		}

		apierror.WriteJSON(w, http.StatusOK, room)
	}
}

// GET /chat/rooms
func listRoomsHandler(svc Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}

		rooms, err := svc.ListRooms(r.Context(), userID)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}

		apierror.WriteJSON(w, http.StatusOK, rooms)
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
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}

		roomID := chi.URLParam(r, "id")

		room, err := svc.GetRoom(r.Context(), roomID)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		if room.Type == RoomTypeChannel {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("[]")) //nolint:errcheck
			return
		}

		// Public rooms are readable by any authenticated user; other rooms require membership.
		if room.Type != RoomTypePublic {
			member, err := svc.IsMember(r.Context(), roomID, userID)
			if err != nil || !member {
				apierror.Write(w, http.StatusForbidden, apierror.CodeForbidden, "forbidden")
				return
			}
		}

		var before *time.Time
		if raw := r.URL.Query().Get("before"); raw != "" {
			t, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid before timestamp")
				return
			}
			before = &t
		}

		limit := 50
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 200 {
				limit = n
			}
		}

		msgs, err := svc.ListMessages(r.Context(), roomID, before, limit)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}

		// Mask view_once content for non-senders: the recipient must call
		// POST /rooms/{id}/messages/{msgID}/view to fetch the real content.
		for i := range msgs {
			if msgs[i].ViewOnce && msgs[i].SenderID != userID {
				msgs[i].Content = ""
			}
		}

		apierror.WriteJSON(w, http.StatusOK, msgs)
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
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}

		roomID := chi.URLParam(r, "id")

		readAt, err := svc.MarkRead(r.Context(), roomID, userID)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
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
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}

		roomID := chi.URLParam(r, "id")
		msgID := chi.URLParam(r, "msgID")

		member, err := svc.IsMember(r.Context(), roomID, userID)
		if err != nil || !member {
			apierror.Write(w, http.StatusForbidden, apierror.CodeForbidden, "forbidden")
			return
		}

		msg, keys, err := svc.ViewOnceMessage(r.Context(), msgID, roomID, userID)
		if errors.Is(err, ErrNotFound) {
			apierror.Write(w, http.StatusNotFound, apierror.CodeNotFound, "message not found")
			return
		}
		if errors.Is(err, ErrForbidden) {
			apierror.Write(w, http.StatusForbidden, apierror.CodeForbidden, "forbidden")
			return
		}
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
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
		apierror.WriteJSON(w, http.StatusOK, msg)

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

// updateGroupHandler renames a group room. Only the creator (admin) may rename.
//
// PUT /chat/rooms/{id}
// Body: {"name": "..."}
func updateGroupHandler(svc Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		roomID := chi.URLParam(r, "id")
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeNameRequired, "name is required")
			return
		}
		err := svc.UpdateGroupName(r.Context(), roomID, userID, body.Name)
		if errors.Is(err, ErrNotFound) {
			apierror.Write(w, http.StatusNotFound, apierror.CodeRoomNotFound, "room not found")
			return
		}
		if errors.Is(err, ErrForbidden) {
			apierror.Write(w, http.StatusForbidden, apierror.CodeForbidden, "forbidden")
			return
		}
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// listGroupMembersHandler returns the member profiles for a group room, or the
// active WebSocket participants for a channel room (ephemeral membership).
//
// GET /chat/rooms/{id}/members
func listGroupMembersHandler(svc Manager, cfg HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		roomID := chi.URLParam(r, "id")
		room, err := svc.GetRoom(r.Context(), roomID)
		if errors.Is(err, ErrNotFound) {
			apierror.Write(w, http.StatusNotFound, apierror.CodeRoomNotFound, "room not found")
			return
		}
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}

		if room.Type == RoomTypeChannel || room.Type == RoomTypePublic {
			// For channels and public rooms, return currently connected participants from the hub.
			var participants []ClientInfo
			if cfg.Hub != nil {
				participants = cfg.Hub.RoomParticipants(r.Context(), roomID)
			}
			if participants == nil {
				participants = []ClientInfo{}
			}
			apierror.WriteJSON(w, http.StatusOK, participants)
			return
		}

		// For group/DM rooms, require the caller to be a member.
		member, err := svc.IsMember(r.Context(), roomID, userID)
		if err != nil || !member {
			apierror.Write(w, http.StatusForbidden, apierror.CodeForbidden, "forbidden")
			return
		}

		profiles, err := svc.ListMemberProfiles(r.Context(), roomID)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		apierror.WriteJSON(w, http.StatusOK, profiles)
	}
}

// addGroupMemberHandler adds a user to a group room. Only the creator may add.
//
// POST /chat/rooms/{id}/members
// Body: {"user_id": "<uuid>"}
func addGroupMemberHandler(svc Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		roomID := chi.URLParam(r, "id")
		var body struct {
			UserID string `json:"user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.UserID == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "user_id is required")
			return
		}
		err := svc.AddGroupMember(r.Context(), roomID, userID, body.UserID)
		if errors.Is(err, ErrNotFound) {
			apierror.Write(w, http.StatusNotFound, apierror.CodeRoomNotFound, "room not found")
			return
		}
		if errors.Is(err, ErrForbidden) {
			apierror.Write(w, http.StatusForbidden, apierror.CodeForbidden, "forbidden")
			return
		}
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// removeGroupMemberHandler removes a member from a group room.
// The creator (admin) may remove any member; any member may remove themselves (leave).
// The creator cannot be removed.
//
// DELETE /chat/rooms/{id}/members/{userID}
func removeGroupMemberHandler(svc Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		roomID := chi.URLParam(r, "id")
		targetID := chi.URLParam(r, "userID")
		err := svc.RemoveGroupMember(r.Context(), roomID, actorID, targetID)
		if errors.Is(err, ErrNotFound) {
			apierror.Write(w, http.StatusNotFound, apierror.CodeRoomNotFound, "room not found")
			return
		}
		if errors.Is(err, ErrForbidden) {
			apierror.Write(w, http.StatusForbidden, apierror.CodeForbidden, "forbidden")
			return
		}
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// listChannelsHandler returns all public channel rooms.
// ActiveCount is populated from the Hub when available.
//
// GET /chat/channels
func listChannelsHandler(svc Manager, cfg HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		channels, err := svc.ListChannels(r.Context())
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		if cfg.Hub != nil {
			for i := range channels {
				channels[i].ActiveCount = len(cfg.Hub.RoomParticipants(r.Context(), channels[i].ID))
			}
		}
		apierror.WriteJSON(w, http.StatusOK, channels)
	}
}

// createChannelHandler creates a public channel room.
//
// POST /chat/channels
// Body: {"name": "...", "description": "..."}
func createChannelHandler(svc Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		var body struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeNameRequired, "name is required")
			return
		}
		room, err := svc.CreateChannel(r.Context(), userID, body.Name, body.Description)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		apierror.WriteJSON(w, http.StatusCreated, room)
	}
}

// listPublicRoomsHandler returns all public guest-accessible rooms.
// ActiveCount is populated from the Hub when available.
//
// GET /chat/public-rooms
func listPublicRoomsHandler(svc Manager, cfg HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rooms, err := svc.ListPublicRooms(r.Context())
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		if cfg.Hub != nil {
			for i := range rooms {
				rooms[i].ActiveCount = len(cfg.Hub.RoomParticipants(r.Context(), rooms[i].ID))
			}
		}
		if rooms == nil {
			rooms = []PublicRoomSummary{}
		}
		apierror.WriteJSON(w, http.StatusOK, rooms)
	}
}

// wsHandler upgrades the connection to WebSocket and starts the client pumps.
// Auth is performed via a single-use ?ticket= query parameter (obtained from
// POST /ws-ticket) because the browser WebSocket API does not support custom
// headers. The ticket is consumed on first use; reuse returns 401.
//
// GET /chat/rooms/{id}/ws?ticket=<uuid>
func wsHandler(svc Manager, hub *Hub, tickets wsticket.Redeemer, notifyNewMessage func(recipientID, roomID string), cfg HandlerConfig) http.HandlerFunc {
	allowed := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		allowed[o] = struct{}{}
	}
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			if len(allowed) == 0 {
				return true
			}
			_, ok := allowed[r.Header.Get("Origin")]
			return ok
		},
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ticket := r.URL.Query().Get("ticket")
		if ticket == "" {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		ticketData, err := tickets.Redeem(r.Context(), ticket)
		if err != nil {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}

		roomID := chi.URLParam(r, "id")

		room, err := svc.GetRoom(r.Context(), roomID)
		if errors.Is(err, ErrNotFound) {
			apierror.Write(w, http.StatusNotFound, apierror.CodeRoomNotFound, "room not found")
			return
		}
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}

		isChannel := room.Type == RoomTypeChannel
		isPublic := room.Type == RoomTypePublic
		isGuest := ticketData.IsGuest

		// Guests may only connect to public rooms.
		if isGuest && !isPublic {
			apierror.Write(w, http.StatusForbidden, apierror.CodeForbidden, "forbidden")
			return
		}

		// Registered users must be a member of non-channel/non-public rooms.
		if !isChannel && !isPublic {
			member, err := svc.IsMember(r.Context(), roomID, ticketData.UserID)
			if err != nil || !member {
				apierror.Write(w, http.StatusForbidden, apierror.CodeForbidden, "forbidden")
				return
			}
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		var displayName, avatarURL, username string
		if isGuest {
			displayName = ticketData.GuestNickname
		} else {
			displayName, _ = svc.GetDisplayName(r.Context(), ticketData.UserID)
			avatarURL, _ = svc.GetAvatarURL(r.Context(), ticketData.UserID)
			username, _ = svc.GetUsername(r.Context(), ticketData.UserID)
		}

		// Start a session span that lives for the lifetime of this WebSocket
		// connection. Use a detached context (not r.Context()) so the span is
		// not cancelled when the HTTP handler returns after launching goroutines.
		// Link it to the HTTP request's trace via a remote span context so the
		// WS session appears as a child of the HTTP upgrade request in Tempo.
		sc := trace.SpanContextFromContext(r.Context())
		sessCtx := trace.ContextWithRemoteSpanContext(context.Background(), sc)
		sessCtx, _ = otel.Tracer("circl/chat").Start(sessCtx, "ws.session",
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("user.id", ticketData.UserID),
				attribute.String("room.id", roomID),
				attribute.Bool("chat.is_channel", isChannel),
				attribute.Bool("chat.is_public", isPublic),
				attribute.Bool("chat.is_guest", isGuest),
			),
		)

		var privacy PrivacyFlags
		if !isGuest && cfg.PrivacyResolver != nil {
			privacy, _ = cfg.PrivacyResolver(r.Context(), ticketData.UserID)
		}

		client := &Client{
			hub:               hub,
			conn:              conn,
			send:              make(chan []byte, 256),
			userID:            ticketData.UserID,
			roomID:            roomID,
			username:          username,
			displayName:       displayName,
			avatarURL:         avatarURL,
			isChannel:         isChannel,
			isPublic:          isPublic,
			isGuest:           isGuest,
			isServiceProvider: cfg.IsServiceProvider,
			guestMsgLimiter:   cfg.GuestMsgLimiter,
			guestMsgRate:      cfg.GuestMsgRate,
			guestMsgWindow:    cfg.GuestMsgWindow,
			isBlockedInRoom:   cfg.IsBlockedInRoom,
			isMutedInRoom:     cfg.IsMutedInRoom,
			hideReadReceipts:  privacy.HideReadReceipts,
			hideTyping:        privacy.HideTyping,
			ctx:               sessCtx,
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
		trace.SpanFromContext(c.ctx).End()
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
			if c.hideTyping {
				continue
			}
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

		// Channels are broadcast-only: their messages are never persisted (see
		// RetentionDurations in chat.go). They are text-only and ignore
		// attachment, TTL and view-once inputs. The frame is minted in-process
		// from the connection's identity rather than a stored row, so there is
		// no DB write and nothing to retain or sweep.
		if c.isChannel {
			if msgType != MessageTypeText {
				continue
			}
			frame, err := json.Marshal(serverMessage{
				Event:           "new_message",
				Type:            MessageTypeText,
				ID:              uuid.NewString(),
				RoomID:          c.roomID,
				SenderID:        c.userID,
				SenderName:      c.displayName,
				SenderAvatarURL: c.avatarURL,
				Content:         in.Content,
				CreatedAt:       time.Now().UTC(),
			})
			if err != nil {
				continue
			}
			_ = c.hub.Publish(c.ctx, c.roomID, frame)
			continue
		}

		// Guests in public rooms are text-only — reject attachments.
		if c.isGuest && msgType != MessageTypeText {
			continue
		}

		// Mute check: send a feedback event and drop the message.
		if c.isMutedInRoom != nil && c.isMutedInRoom(c.ctx, c.roomID, c.userID) {
			if frame, err := json.Marshal(map[string]any{
				"event":   "you_are_muted",
				"room_id": c.roomID,
			}); err == nil {
				select {
				case c.send <- frame:
				default:
				}
			}
			continue
		}

		// Guest message rate limiting (stricter than registered users).
		if c.isGuest && c.guestMsgLimiter != nil && c.guestMsgRate > 0 {
			ok, err := c.guestMsgLimiter.Allow(c.ctx, "guest:msg:"+c.userID, c.guestMsgRate, c.guestMsgWindow)
			if err != nil || !ok {
				continue
			}
		}

		// Service-provider profiles are not allowed to post in public rooms.
		if c.isPublic && c.isServiceProvider != nil && c.isServiceProvider(c.ctx, c.userID) {
			continue
		}

		// Attachment messages must carry an upload_id so the file can be
		// linked to the message record and cleaned up on deletion.
		if in.Type == "attachment" && in.UploadID == "" {
			continue
		}

		params := SaveMessageParams{
			RoomID:        c.roomID,
			SenderID:      c.userID,
			Type:          msgType,
			Content:       in.Content,
			UploadID:      in.UploadID,
			ViewOnce:      in.ViewOnce,
			SenderIsGuest: c.isGuest,
			SenderName:    c.displayName,
		}
		if c.isGuest {
			// Guests cannot send ephemeral messages or attachments.
			params.UploadID = ""
			params.ViewOnce = false
			params.ExpiresAt = nil
		} else if in.TTL != "" {
			d, err := ParseTTL(in.TTL)
			if err != nil {
				continue // reject unknown TTL values
			}
			t := time.Now().Add(d)
			params.ExpiresAt = &t
		}

		// Closure lets us defer span.End() and exit early with return instead of
		// continue, ensuring every code path ends the span correctly.
		func() {
			msgCtx, span := otel.Tracer("circl/chat").Start(c.ctx, "ws.message",
				trace.WithAttributes(
					attribute.String("room.id", c.roomID),
					attribute.String("message.type", in.Type),
				),
			)
			defer span.End()

			if c.isBlockedInRoom != nil && c.isBlockedInRoom(msgCtx, c.userID, c.roomID) {
				return
			}

			msg, err := svc.SaveMessage(msgCtx, params)
			if err != nil {
				span.RecordError(err)
				return
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
				Redacted:        msg.Redacted,
				ExpiresAt:       msg.ExpiresAt,
				CreatedAt:       msg.CreatedAt,
			})
			if err != nil {
				return
			}

			_ = c.hub.Publish(msgCtx, c.roomID, data)

			if msg.Redacted {
				noteData, noteErr := json.Marshal(serverMessage{
					Event:     "new_message",
					Type:      MessageTypeSystem,
					RoomID:    c.roomID,
					Content:   "Contact information was removed from the previous message to protect privacy.",
					CreatedAt: msg.CreatedAt,
				})
				if noteErr == nil {
					_ = c.hub.Publish(msgCtx, c.roomID, noteData)
				}
			}

			// Notify non-sender members so they can show an unread badge.
			if notifyNewMessage != nil {
				if members, err := svc.ListMembers(msgCtx, c.roomID); err == nil {
					for _, uid := range members {
						if uid != c.userID {
							notifyNewMessage(uid, c.roomID)
						}
					}
				}
			}
		}()
	}
}

// modKickHandler handles POST /chat/rooms/{id}/mod/kick (admin only).
// Body: {"target_id": "..."}
// Disconnects the target participant and IP-bans them if they are a guest.
func modKickHandler(svc Manager, cfg HandlerConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roomID := chi.URLParam(r, "id")
		room, err := svc.GetRoom(r.Context(), roomID)
		if err != nil || room.Type != RoomTypePublic {
			apierror.Write(w, http.StatusNotFound, apierror.CodeRoomNotFound, "public room not found")
			return
		}

		var body struct {
			TargetID string `json:"target_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TargetID == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "target_id is required")
			return
		}

		if cfg.KickFromRoom != nil {
			cfg.KickFromRoom(roomID, body.TargetID)
		}

		// IP-ban guests (target_id starts with "guest:").
		if strings.HasPrefix(body.TargetID, "guest:") && cfg.GetGuestSession != nil && cfg.BanRoomIP != nil {
			if ipHash, err := cfg.GetGuestSession(r.Context(), body.TargetID); err == nil && ipHash != "" {
				_ = cfg.BanRoomIP(r.Context(), roomID, ipHash, 24*time.Hour)
			}
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// modMuteHandler handles POST /chat/rooms/{id}/mod/mute (admin only).
// Body: {"target_id": "...", "duration": "15m"|"1h"|"24h"}
func modMuteHandler(cfg HandlerConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roomID := chi.URLParam(r, "id")

		var body struct {
			TargetID string `json:"target_id"`
			Duration string `json:"duration"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TargetID == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "target_id is required")
			return
		}

		durations := map[string]time.Duration{
			"15m": 15 * time.Minute,
			"1h":  time.Hour,
			"24h": 24 * time.Hour,
		}
		ttl, ok := durations[body.Duration]
		if !ok {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "duration must be 15m, 1h, or 24h")
			return
		}

		if cfg.MuteInRoom != nil {
			if err := cfg.MuteInRoom(r.Context(), roomID, body.TargetID, ttl); err != nil {
				apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
				return
			}
		}

		if cfg.ScheduleUnmute != nil {
			cfg.ScheduleUnmute(roomID, body.TargetID, ttl)
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// modUnmuteHandler handles DELETE /chat/rooms/{id}/mod/mute/{targetID} (admin only).
func modUnmuteHandler(cfg HandlerConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roomID := chi.URLParam(r, "id")
		targetID := chi.URLParam(r, "targetID")

		if cfg.UnmuteInRoom != nil {
			if err := cfg.UnmuteInRoom(r.Context(), roomID, targetID); err != nil {
				apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
				return
			}
		}

		w.WriteHeader(http.StatusNoContent)
	})
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
