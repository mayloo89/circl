package reports

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mayloo89/circl/backend/internal/middleware"
)

// NewHandler returns an http.Handler with all report routes.
func NewHandler(m *Manager) http.Handler {
	r := chi.NewRouter()
	r.Post("/", m.CreateReport)
	return r
}

// Manager wraps the service with HTTP handlers.
type Manager struct {
	svc *Service
}

// NewManager creates a Manager backed by the given service.
func NewManager(svc *Service) *Manager {
	return &Manager{svc: svc}
}

// CreateReport handles POST /reports.
func (m *Manager) CreateReport(w http.ResponseWriter, r *http.Request) {
	reporterID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
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
	json.NewEncoder(w).Encode(report)
}

// ListReports is reserved for admin use and will be wired up when a role model is in place.
func (m *Manager) ListReports(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

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
	json.NewEncoder(w).Encode(reports)
}

// UpdateReportStatus is reserved for admin use and will be wired up when a role model is in place.
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
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Status == "" {
		http.Error(w, "status is required", http.StatusBadRequest)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}
