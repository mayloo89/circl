package reports

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mayloo89/circl/backend/internal/middleware"
)

// Manager wraps the service with HTTP handlers.
type Manager struct {
	svc       *Service
	moderator AdminModerator
	limiter   RateLimiter
}

// WithModerator sets the admin moderator on the Manager.
func WithModerator(m AdminModerator) func(*Manager) {
	return func(mgr *Manager) { mgr.moderator = m }
}

// WithLimiter sets the rate limiter on the Manager.
func WithLimiter(l RateLimiter) func(*Manager) {
	return func(mgr *Manager) { mgr.limiter = l }
}

// NewManager creates a Manager backed by the given service.
func NewManager(svc *Service, opts ...func(*Manager)) *Manager {
	mgr := &Manager{svc: svc}
	for _, o := range opts {
		o(mgr)
	}
	return mgr
}

// NewHandler returns an http.Handler with all report routes.
// Admin-only routes (GET / and PUT /{id}/status) are protected by RequireAdmin.
func NewHandler(m *Manager) http.Handler {
	r := chi.NewRouter()
	r.Post("/", m.CreateReport)
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAdmin)
		r.Get("/", m.ListReports)
		r.Put("/{id}/status", m.UpdateReportStatus)
	})
	return r
}

// CreateReport handles POST /reports.
func (m *Manager) CreateReport(w http.ResponseWriter, r *http.Request) {
	reporterID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if m.limiter != nil {
		key := fmt.Sprintf("reports:%s:%s", reporterID, time.Now().UTC().Format("2006-01-02"))
		allowed, err := m.limiter.Allow(r.Context(), key, 10, 24*time.Hour)
		if err != nil {
			log.Printf("reports: rate limiter error: %v", err)
		} else if !allowed {
			http.Error(w, "Too many reports", http.StatusTooManyRequests)
			return
		}
	}

	var req struct {
		ReportedUserID string `json:"reported_user_id"`
		Reason         string `json:"reason"`
		Description    string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.ReportedUserID == "" {
		http.Error(w, "reported_user_id is required", http.StatusBadRequest)
		return
	}
	if req.Reason == "" {
		http.Error(w, "reason is required", http.StatusBadRequest)
		return
	}

	report, err := m.svc.Create(r.Context(), reporterID, req.ReportedUserID, req.Reason, req.Description)
	if err != nil {
		if errors.Is(err, ErrSelfReport) {
			http.Error(w, "Cannot report yourself", http.StatusBadRequest)
			return
		}
		if errors.Is(err, ErrInvalidReason) {
			http.Error(w, "Invalid reason", http.StatusBadRequest)
			return
		}
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(report) //nolint:errcheck
}

// ListReports handles GET /reports (admin only).
func (m *Manager) ListReports(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")

	reports, err := m.svc.List(r.Context(), status)
	if err != nil {
		if errors.Is(err, ErrInvalidStatus) {
			http.Error(w, "Invalid status", http.StatusBadRequest)
			return
		}
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reports) //nolint:errcheck
}

// UpdateReportStatus handles PUT /reports/{id}/status (admin only).
// An optional action field ("suspend" or "ban") triggers a moderation action
// on the reported user after the report status is updated.
func (m *Manager) UpdateReportStatus(w http.ResponseWriter, r *http.Request) {
	reviewerID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	reportID := chi.URLParam(r, "id")
	if reportID == "" {
		http.Error(w, "Missing report ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Status       string `json:"status"`
		Action       string `json:"action"`        // "suspend", "ban", or ""
		DurationDays int    `json:"duration_days"` // for suspend
		Reason       string `json:"reason"`        // for suspend/ban
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Status == "" {
		http.Error(w, "status is required", http.StatusBadRequest)
		return
	}

	if req.Action != "" && req.Action != "suspend" && req.Action != "ban" {
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}

	report, err := m.svc.UpdateStatus(r.Context(), reportID, req.Status, reviewerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "Report not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrInvalidStatus) {
			http.Error(w, "Invalid status", http.StatusBadRequest)
			return
		}
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	if m.moderator != nil {
		switch req.Action {
		case "suspend":
			if err := m.moderator.SuspendUser(r.Context(), report.ReportedUserID, req.Reason, req.DurationDays, reviewerID); err != nil {
				log.Printf("reports: suspend user %s: %v", report.ReportedUserID, err)
			}
		case "ban":
			if err := m.moderator.BanUser(r.Context(), report.ReportedUserID, req.Reason, reviewerID); err != nil {
				log.Printf("reports: ban user %s: %v", report.ReportedUserID, err)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report) //nolint:errcheck
}
