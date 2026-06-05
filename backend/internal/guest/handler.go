package guest

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mayloo89/circl/backend/internal/apierror"
	"github.com/mayloo89/circl/backend/internal/chat"
	"github.com/mayloo89/circl/backend/internal/ratelimit"
)

// NicknameChecker returns true when the nickname is already taken by a
// registered user (matches username or display_name). Implemented by
// chat.Store.NicknameTaken so this package stays decoupled from chat.
type NicknameChecker func(ctx context.Context, nickname string) (bool, error)

// SessionCreator creates and retrieves guest sessions.
type SessionCreator interface {
	Create(ctx context.Context, nickname string) (*Session, error)
	Get(ctx context.Context, sessionID string) (*Session, error)
}

// HandlerConfig holds dependencies for the guest HTTP handler.
type HandlerConfig struct {
	Sessions      SessionCreator
	NicknameTaken NicknameChecker
	RoomLister    chat.Manager
	// Participants returns the live WebSocket participants for a room. Used to
	// build the guest-facing roster without exposing registered handles.
	Participants func(ctx context.Context, roomID string) []chat.ClientInfo
	// Captcha, when set, verifies the anti-bot token submitted with a guest
	// session. When nil (no provider configured), the check is skipped.
	Captcha       func(ctx context.Context, token, remoteIP string) (bool, error)
	Limiter       *ratelimit.RedisLimiter
	GuestIPRate   int
	GuestIPWindow time.Duration
}

// NewHandler returns a handler for guest-facing routes (mount at /guest):
//
// POST /guest/session — create a guest session (nickname + age attestation)
// GET /guest/rooms — list public rooms (unauthenticated)
// GET /guest/rooms/{id}/participants — live roster for a public room
func NewHandler(cfg HandlerConfig) http.Handler {
	r := chi.NewRouter()
	r.Post("/session", createGuestSession(cfg))
	r.Get("/rooms", listPublicRooms(cfg))
	r.Get("/rooms/{id}/participants", listRoomParticipants(cfg))
	return r
}

// guestParticipant is the privacy-reduced view of a public-room participant:
// display name + badge only, never the registered @handle.
type guestParticipant struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	IsGuest     bool   `json:"is_guest"`
}

// listRoomParticipants handles GET /guest/rooms/{id}/participants.
// Returns the live participants of a public room (display name + badge only).
func listRoomParticipants(cfg HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID := chi.URLParam(r, "id")
		room, err := cfg.RoomLister.GetRoom(r.Context(), roomID)
		if err != nil || room.Type != chat.RoomTypePublic {
			apierror.Write(w, http.StatusNotFound, apierror.CodeRoomNotFound, "room not found")
			return
		}
		out := []guestParticipant{}
		if cfg.Participants != nil {
			for _, p := range cfg.Participants(r.Context(), roomID) {
				out = append(out, guestParticipant{
					UserID:      p.UserID,
					DisplayName: p.DisplayName,
					AvatarURL:   p.AvatarURL,
					IsGuest:     p.IsGuest,
				})
			}
		}
		apierror.WriteJSON(w, http.StatusOK, out)
	}
}

// createGuestSession handles POST /guest/session.
// Body: {"nickname": "...", "age_attestation": true}
func createGuestSession(cfg HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr

		if cfg.Limiter != nil && cfg.GuestIPRate > 0 {
			ok, err := cfg.Limiter.Allow(r.Context(), "guest:ip:"+ip, cfg.GuestIPRate, cfg.GuestIPWindow)
			if err != nil || !ok {
				apierror.Write(w, http.StatusTooManyRequests, apierror.CodeGuestRateLimited, "rate limit exceeded")
				return
			}
		}

		var body struct {
			Nickname       string `json:"nickname"`
			AgeAttestation bool   `json:"age_attestation"`
			CaptchaToken   string `json:"captcha_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
			return
		}
		if body.Nickname == "" || len(body.Nickname) > 30 {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "nickname is required (max 30 chars)")
			return
		}
		if !body.AgeAttestation {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeAgeAttestRequired, "age attestation is required")
			return
		}

		// Anti-bot gate. Skipped entirely when no provider is configured (dev).
		if cfg.Captcha != nil {
			ok, err := cfg.Captcha(r.Context(), body.CaptchaToken, ip)
			if err != nil || !ok {
				apierror.Write(w, http.StatusForbidden, apierror.CodeCaptchaFailed, "captcha verification failed")
				return
			}
		}

		if cfg.NicknameTaken != nil {
			taken, err := cfg.NicknameTaken(r.Context(), body.Nickname)
			if err != nil {
				apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
				return
			}
			if taken {
				apierror.Write(w, http.StatusConflict, apierror.CodeNicknameTaken, "nickname is taken by a registered user")
				return
			}
		}

		sess, err := cfg.Sessions.Create(r.Context(), body.Nickname)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}

		apierror.WriteJSON(w, http.StatusCreated, map[string]string{
			"session_id": sess.ID,
			"nickname":   sess.Nickname,
		})
	}
}

// listPublicRooms handles GET /guest/rooms.
// Returns the public room directory — unauthenticated.
func listPublicRooms(cfg HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rooms, err := cfg.RoomLister.ListPublicRooms(r.Context())
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		if rooms == nil {
			rooms = []chat.PublicRoomSummary{}
		}
		apierror.WriteJSON(w, http.StatusOK, rooms)
	}
}
