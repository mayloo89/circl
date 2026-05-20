package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mayloo89/circl/backend/internal/apierror"
	"github.com/mayloo89/circl/backend/internal/middleware"
)

// handler handles admin HTTP routes.
type handler struct {
	svc      *Service
	presence PresenceLookupFunc
}

// HandlerOption configures optional admin sub-routers.
type HandlerOption func(*handlerConfig)

type handlerConfig struct {
	appeals    http.Handler
	moderation http.Handler
}

// WithAppealsHandler mounts an admin-only sub-router at /admin/appeals. The
// caller supplies the handler so this package doesn't import appeals.
func WithAppealsHandler(h http.Handler) HandlerOption {
	return func(c *handlerConfig) { c.appeals = h }
}

// WithModerationHandler mounts an admin-only sub-router at /admin/moderation.
// The caller supplies the handler so this package doesn't import moderation.
func WithModerationHandler(h http.Handler) HandlerOption {
	return func(c *handlerConfig) { c.moderation = h }
}

// NewHandler returns an http.Handler covering all admin routes.
// All routes require the caller to be an admin (checked via RequireAdmin middleware).
// Must be mounted behind RequireAuth so the admin flag is already in context.
//
// presence may be nil; in that case the user list omits live presence
// (legacy behavior) and existing tests don't need to wire it up.
func NewHandler(svc *Service, presence PresenceLookupFunc, opts ...HandlerOption) http.Handler {
	cfg := &handlerConfig{}
	for _, o := range opts {
		o(cfg)
	}
	h := &handler{svc: svc, presence: presence}
	r := chi.NewRouter()
	r.Use(middleware.RequireAdmin)
	r.Get("/stats", h.getStats)
	r.Get("/users", h.listUsers)
	r.Put("/users/{id}/status", h.updateUserStatus)
	r.Get("/channels", h.listChannels)
	r.Post("/channels", h.createChannel)
	r.Put("/channels/{id}", h.updateChannel)
	r.Delete("/channels/{id}", h.deleteChannel)

	if cfg.appeals != nil {
		r.Mount("/appeals", cfg.appeals)
	}
	if cfg.moderation != nil {
		r.Mount("/moderation", cfg.moderation)
	}
	// Super-admin-only routes.
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireSuperAdmin)
		r.Delete("/users/{id}", h.hardDeleteUser)
		r.Put("/users/{id}/role", h.setUserRole)
	})

	return r
}

// getStats handles GET /admin/stats.
func (h *handler) getStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.GetStats(r.Context())
	if err != nil {
		apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
		return
	}
	apierror.WriteJSON(w, http.StatusOK, stats)
}

// listUsers handles GET /admin/users?q=&status=&limit=&offset=.
func (h *handler) listUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	status := r.URL.Query().Get("status")

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	users, total, err := h.svc.ListUsers(r.Context(), q, status, limit, offset)
	if err != nil {
		apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
		return
	}

	// Overlay live presence onto each user record. We do this at the
	// handler layer rather than in the service so admin stays decoupled
	// from the presence package; the lookup is injected via NewHandler.
	var pres map[string]UserPresence
	if h.presence != nil && len(users) > 0 {
		ids := make([]string, len(users))
		for i, u := range users {
			ids[i] = u.ID
		}
		pres, err = h.presence(r.Context(), ids)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
	}

	type listItem struct {
		*UserRecord
		Online     bool       `json:"online"`
		LastSeenAt *time.Time `json:"last_seen_at,omitzero"`
	}
	enriched := make([]listItem, len(users))
	for i, u := range users {
		p := pres[u.ID]
		enriched[i] = listItem{UserRecord: u, Online: p.Online, LastSeenAt: p.LastSeenAt}
	}

	apierror.WriteJSON(w, http.StatusOK, map[string]any{
		"users": enriched,
		"total": total,
	})
}

// listChannels handles GET /admin/channels.
func (h *handler) listChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := h.svc.ListChannels(r.Context())
	if err != nil {
		apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
		return
	}
	apierror.WriteJSON(w, http.StatusOK, channels)
}

// deleteChannel handles DELETE /admin/channels/{id}.
func (h *handler) deleteChannel(w http.ResponseWriter, r *http.Request) {
	channelID := chi.URLParam(r, "id")
	if channelID == "" {
		apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "missing channel id")
		return
	}
	err := h.svc.DeleteChannel(r.Context(), channelID)
	if errors.Is(err, ErrChannelNotFound) {
		apierror.Write(w, http.StatusNotFound, apierror.CodeChannelNotFound, "channel not found")
		return
	}
	if err != nil {
		apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// createChannel handles POST /admin/channels.
// Body: {"name":"...","description":"..."}
func (h *handler) createChannel(w http.ResponseWriter, r *http.Request) {
	adminID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		apierror.Write(w, http.StatusBadRequest, apierror.CodeNameRequired, "name is required")
		return
	}

	ch, err := h.svc.CreateChannel(r.Context(), adminID, req.Name, req.Description)
	if errors.Is(err, ErrChannelNameTaken) {
		apierror.Write(w, http.StatusConflict, apierror.CodeChannelNameTaken, "channel name already taken")
		return
	}
	if err != nil {
		apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
		return
	}
	apierror.WriteJSON(w, http.StatusCreated, ch)
}

// updateChannel handles PUT /admin/channels/{id}.
// Body: {"name":"...","description":"..."}
func (h *handler) updateChannel(w http.ResponseWriter, r *http.Request) {
	channelID := chi.URLParam(r, "id")
	if channelID == "" {
		apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "missing channel id")
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		apierror.Write(w, http.StatusBadRequest, apierror.CodeNameRequired, "name is required")
		return
	}

	err := h.svc.UpdateChannel(r.Context(), channelID, req.Name, req.Description)
	if errors.Is(err, ErrChannelNotFound) {
		apierror.Write(w, http.StatusNotFound, apierror.CodeChannelNotFound, "channel not found")
		return
	}
	if errors.Is(err, ErrChannelNameTaken) {
		apierror.Write(w, http.StatusConflict, apierror.CodeChannelNameTaken, "channel name already taken")
		return
	}
	if err != nil {
		apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// hardDeleteUser handles DELETE /admin/users/{id}.
// Immediately purges all user data with no grace period. Super-admin only.
func (h *handler) hardDeleteUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "missing user id")
		return
	}
	err := h.svc.HardDeleteUser(r.Context(), userID)
	if errors.Is(err, ErrUserNotFound) {
		apierror.Write(w, http.StatusNotFound, apierror.CodeUserNotFound, "user not found")
		return
	}
	if err != nil {
		apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// setUserRole handles PUT /admin/users/{id}/role.
// Body: {"role":"user"|"admin"|"super_admin"}. Super-admin only.
func (h *handler) setUserRole(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "missing user id")
		return
	}

	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
		return
	}

	err := h.svc.SetUserRole(r.Context(), userID, req.Role)
	if errors.Is(err, ErrInvalidRole) {
		apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid role: must be user, admin, or super_admin")
		return
	}
	if errors.Is(err, ErrUserNotFound) {
		apierror.Write(w, http.StatusNotFound, apierror.CodeUserNotFound, "user not found")
		return
	}
	if err != nil {
		apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// updateUserStatus handles PUT /admin/users/{id}/status.
// Body: {"action":"suspend"|"ban"|"reactivate","reason":"...","duration_days":7}
func (h *handler) updateUserStatus(w http.ResponseWriter, r *http.Request) {
	adminID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
		return
	}

	userID := chi.URLParam(r, "id")
	if userID == "" {
		apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "missing user id")
		return
	}

	var req struct {
		Action       string `json:"action"`
		Reason       string `json:"reason"`
		DurationDays int    `json:"duration_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
		return
	}

	switch req.Action {
	case "suspend":
		err := h.svc.SuspendUser(r.Context(), userID, req.Reason, req.DurationDays, adminID)
		if errors.Is(err, ErrUserNotFound) {
			apierror.Write(w, http.StatusNotFound, apierror.CodeUserNotFound, "user not found")
			return
		}
		if errors.Is(err, ErrAlreadySuspended) {
			apierror.Write(w, http.StatusConflict, apierror.CodeInvalidRequest, "user already suspended or banned")
			return
		}
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
	case "ban":
		err := h.svc.BanUser(r.Context(), userID, req.Reason, adminID)
		if errors.Is(err, ErrUserNotFound) {
			apierror.Write(w, http.StatusNotFound, apierror.CodeUserNotFound, "user not found")
			return
		}
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
	case "reactivate":
		err := h.svc.ReactivateUser(r.Context(), userID)
		if errors.Is(err, ErrUserNotFound) {
			apierror.Write(w, http.StatusNotFound, apierror.CodeUserNotFound, "user not found")
			return
		}
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
	default:
		apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid action: must be suspend, ban, or reactivate")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
