package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/mayloo89/circl/backend/internal/middleware"
)

// handler handles admin HTTP routes.
type handler struct {
	svc *Service
}

// NewHandler returns an http.Handler covering all admin routes.
// All routes require the caller to be an admin (checked via RequireAdmin middleware).
// Must be mounted behind RequireAuth so the admin flag is already in context.
func NewHandler(svc *Service) http.Handler {
	h := &handler{svc: svc}
	r := chi.NewRouter()
	r.Use(middleware.RequireAdmin)
	r.Get("/stats", h.getStats)
	r.Get("/users", h.listUsers)
	r.Put("/users/{id}/status", h.updateUserStatus)
	return r
}

// getStats handles GET /admin/stats.
func (h *handler) getStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.GetStats(r.Context())
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats) //nolint:errcheck
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
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
		"users": users,
		"total": total,
	})
}

// updateUserStatus handles PUT /admin/users/{id}/status.
// Body: {"action":"suspend"|"ban"|"reactivate","reason":"...","duration_days":7}
func (h *handler) updateUserStatus(w http.ResponseWriter, r *http.Request) {
	adminID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID := chi.URLParam(r, "id")
	if userID == "" {
		http.Error(w, "Missing user ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Action       string `json:"action"`
		Reason       string `json:"reason"`
		DurationDays int    `json:"duration_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	switch req.Action {
	case "suspend":
		err := h.svc.SuspendUser(r.Context(), userID, req.Reason, req.DurationDays, adminID)
		if errors.Is(err, ErrUserNotFound) {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrAlreadySuspended) {
			http.Error(w, "User already suspended or banned", http.StatusConflict)
			return
		}
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
	case "ban":
		err := h.svc.BanUser(r.Context(), userID, req.Reason, adminID)
		if errors.Is(err, ErrUserNotFound) {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
	case "reactivate":
		err := h.svc.ReactivateUser(r.Context(), userID)
		if errors.Is(err, ErrUserNotFound) {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "Invalid action: must be suspend, ban, or reactivate", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
