package wsticket

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/mayloo89/circl/backend/internal/apierror"
	"github.com/mayloo89/circl/backend/internal/middleware"
)

// SessionValidator validates a guest session ID and returns the nickname.
// Implemented by guest.SessionStore so this package stays decoupled from guest.
type SessionValidator interface {
	Get(ctx context.Context, sessionID string) (*SessionData, error)
}

// SessionData is a minimal guest session representation that wsticket uses
// to avoid importing the guest package.
type SessionData struct {
	ID       string
	Nickname string
}

// NewHandler returns a handler for POST /ws-ticket.
// Must be mounted behind RequireAuth.
func NewHandler(issuer Issuer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		ticket, err := issuer.Issue(r.Context(), userID)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"ticket": ticket}) //nolint:errcheck
	}
}

// NewGuestHandler returns a handler for POST /guest/ws-ticket.
// The caller supplies a SessionValidator so this package doesn't import guest.
func NewGuestHandler(issuer GuestIssuer, sessions SessionValidator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			SessionID string `json:"session_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SessionID == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "session_id is required")
			return
		}

		sess, err := sessions.Get(r.Context(), body.SessionID)
		if err != nil {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "invalid guest session")
			return
		}

		ticket, err := issuer.IssueGuest(r.Context(), sess.ID, sess.Nickname)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"ticket": ticket}) //nolint:errcheck
	}
}
