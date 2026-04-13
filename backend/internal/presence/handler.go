package presence

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mayloo89/circl/backend/internal/apierror"
	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/notifications"
)

// Notifier is satisfied by *notifications.Hub.
type Notifier interface {
	Notify(userID string, e notifications.Event)
}

// NewHandler returns a chi router with the presence endpoints.
// Must be mounted behind requireAuth.
func NewHandler(store *Store, notifier Notifier) http.Handler {
	r := chi.NewRouter()
	r.Post("/heartbeat", heartbeatHandler(store, notifier))
	r.Delete("/heartbeat", offlineHandler(store, notifier))
	r.Get("/", getPresenceHandler(store))
	return r
}

// heartbeatHandler marks the authenticated user as online.
//
// POST /presence/heartbeat
func heartbeatHandler(store *Store, notifier Notifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}

		justOnline, err := store.Heartbeat(r.Context(), userID)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}

		// If the user just came online, notify their contacts via SSE.
		if justOnline {
			go func() {
				contacts, err := store.ContactIDs(r.Context(), userID)
				if err != nil {
					return
				}
				event := notifications.Event{
					Type:    "presence_online",
					Payload: map[string]string{"user_id": userID},
				}
				for _, cid := range contacts {
					notifier.Notify(cid, event)
				}
			}()
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// offlineHandler immediately marks the authenticated user as offline.
//
// DELETE /presence/heartbeat
func offlineHandler(store *Store, notifier Notifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		if err := store.Offline(r.Context(), userID); err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		// Notify contacts so they can update presence immediately.
		go func() {
			contacts, err := store.ContactIDs(r.Context(), userID)
			if err != nil {
				return
			}
			event := notifications.Event{
				Type:    "presence_offline",
				Payload: map[string]string{"user_id": userID},
			}
			for _, cid := range contacts {
				notifier.Notify(cid, event)
			}
		}()
		w.WriteHeader(http.StatusNoContent)
	}
}

// getPresenceHandler returns presence info for a list of user IDs.
//
// GET /presence?ids=id1,id2,...
func getPresenceHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}

		raw := r.URL.Query().Get("ids")
		if raw == "" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("[]")) //nolint:errcheck
			return
		}

		ids := strings.Split(raw, ",")
		// Clamp to 100 IDs per request to avoid abuse.
		if len(ids) > 100 {
			ids = ids[:100]
		}

		info, err := store.GetPresence(r.Context(), ids)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info) //nolint:errcheck
	}
}
