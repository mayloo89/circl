package presence

import (
	"context"
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

// PresenceStore is satisfied by *Store.
type PresenceStore interface {
	Heartbeat(ctx context.Context, userID string) (bool, error)
	Offline(ctx context.Context, userID string) error
	GetPresence(ctx context.Context, userIDs []string) ([]Info, error)
	ContactIDs(ctx context.Context, userID string) ([]string, error)
}

// PrivacyLookup returns the hide_presence flag for each requested user.
// Users without a preferences row are absent from the map and must be
// treated as not hiding (the zero bool).
type PrivacyLookup interface {
	HidePresenceByIDs(ctx context.Context, userIDs []string) (map[string]bool, error)
}

// noopPrivacy is a PrivacyLookup that always returns no hidden users.
// Used in tests and callers that have not opted into the symmetric
// hide_presence gate.
type noopPrivacy struct{}

func (noopPrivacy) HidePresenceByIDs(_ context.Context, _ []string) (map[string]bool, error) {
	return map[string]bool{}, nil
}

// NewHandler returns a chi router with the presence endpoints.
// Must be mounted behind requireAuth.
//
// privacy may be nil; in that case the handler behaves as if no user has
// hide_presence enabled (legacy behavior).
func NewHandler(store PresenceStore, notifier Notifier, privacy PrivacyLookup) http.Handler {
	if privacy == nil {
		privacy = noopPrivacy{}
	}
	r := chi.NewRouter()
	r.Post("/heartbeat", heartbeatHandler(store, notifier, privacy))
	r.Delete("/heartbeat", offlineHandler(store, notifier, privacy))
	r.Get("/", getPresenceHandler(store, privacy))
	return r
}

// heartbeatHandler marks the authenticated user as online.
//
// POST /presence/heartbeat
func heartbeatHandler(store PresenceStore, notifier Notifier, privacy PrivacyLookup) http.HandlerFunc {
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
		// Symmetric: if either the transitioning user or a recipient has
		// hide_presence enabled, the event is suppressed for that pair.
		if justOnline {
			go fanoutPresence(r.Context(), "presence_online", userID, store, notifier, privacy)
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// offlineHandler immediately marks the authenticated user as offline.
//
// DELETE /presence/heartbeat
func offlineHandler(store PresenceStore, notifier Notifier, privacy PrivacyLookup) http.HandlerFunc {
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
		go fanoutPresence(r.Context(), "presence_offline", userID, store, notifier, privacy)
		w.WriteHeader(http.StatusNoContent)
	}
}

// fanoutPresence emits a presence_online/offline SSE event to the user's
// accepted contacts, applying the symmetric hide_presence rule: if the
// transitioning user has hide_presence the event is dropped entirely;
// individual recipients with hide_presence are skipped.
func fanoutPresence(ctx context.Context, eventType, userID string, store PresenceStore, notifier Notifier, privacy PrivacyLookup) {
	contacts, err := store.ContactIDs(ctx, userID)
	if err != nil {
		return
	}
	all := append([]string{userID}, contacts...)
	hide, err := privacy.HidePresenceByIDs(ctx, all)
	if err != nil {
		return
	}
	if hide[userID] {
		return
	}
	event := notifications.Event{
		Type:    eventType,
		Payload: map[string]string{"user_id": userID},
	}
	for _, cid := range contacts {
		if hide[cid] {
			continue
		}
		notifier.Notify(cid, event)
	}
}

// getPresenceHandler returns presence info for a list of user IDs.
// Real presence is only returned for the caller and their accepted contacts;
// all other IDs receive an offline/unknown entry.
//
// Symmetric hide_presence: if the caller has hide_presence enabled, every
// returned entry is forced to offline / no last_seen_at. If a queried
// user has hide_presence enabled, that single entry is forced to offline /
// no last_seen_at regardless of contact status.
//
// GET /presence?ids=id1,id2,...
func getPresenceHandler(store PresenceStore, privacy PrivacyLookup) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		callerID, ok := middleware.UserIDFromContext(r.Context())
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

		// Any authenticated user can see online/offline status.
		// Only accepted contacts (and self) can see last_seen_at.
		contactIDs, err := store.ContactIDs(r.Context(), callerID)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		canSeeLastSeen := make(map[string]bool, len(contactIDs)+1)
		canSeeLastSeen[callerID] = true
		for _, id := range contactIDs {
			canSeeLastSeen[id] = true
		}

		info, err := store.GetPresence(r.Context(), ids)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}

		hide, err := privacy.HidePresenceByIDs(r.Context(), append([]string{callerID}, ids...))
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		callerHides := hide[callerID]

		for i := range info {
			if callerHides || hide[info[i].UserID] {
				info[i].Online = false
				info[i].LastSeenAt = nil
				continue
			}
			if !canSeeLastSeen[info[i].UserID] {
				info[i].LastSeenAt = nil
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info) //nolint:errcheck
	}
}
