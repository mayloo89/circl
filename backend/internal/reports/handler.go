package reports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/apierror"
	"github.com/mayloo89/circl/backend/internal/middleware"
)

// Manager wraps the service with HTTP handlers.
type Manager struct {
	svc       *Service
	moderator AdminModerator
	limiter   RateLimiter
	log       zerolog.Logger
}

// WithModerator sets the admin moderator on the Manager.
func WithModerator(m AdminModerator) func(*Manager) {
	return func(mgr *Manager) { mgr.moderator = m }
}

// WithLimiter sets the rate limiter on the Manager.
func WithLimiter(l RateLimiter) func(*Manager) {
	return func(mgr *Manager) { mgr.limiter = l }
}

// WithLogger sets the logger on the Manager.
func WithLogger(log zerolog.Logger) func(*Manager) {
	return func(mgr *Manager) {
		mgr.log = log.With().Str("component", "reports").Logger()
	}
}

// NewManager creates a Manager backed by the given service.
func NewManager(svc *Service, opts ...func(*Manager)) *Manager {
	mgr := &Manager{svc: svc, log: zerolog.Nop()}
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
		apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
		return
	}

	if m.limiter != nil {
		key := fmt.Sprintf("reports:%s:%s", reporterID, time.Now().UTC().Format("2006-01-02"))
		allowed, err := m.limiter.Allow(r.Context(), key, 10, 24*time.Hour)
		if err != nil {
			zerolog.Ctx(r.Context()).Warn().Err(err).Msg("reports: rate limiter error")
		} else if !allowed {
			apierror.Write(w, http.StatusTooManyRequests, apierror.CodeRateLimited, "too many reports")
			return
		}
	}

	var req struct {
		ReportedUserID string `json:"reported_user_id"`
		MessageID      string `json:"message_id"`
		Reason         string `json:"reason"`
		Description    string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
		return
	}

	if req.Reason == "" {
		apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "reason is required")
		return
	}

	target := Target{UserID: req.ReportedUserID, MessageID: req.MessageID}
	report, err := m.svc.Create(r.Context(), reporterID, target, req.Reason, req.Description)
	if err != nil {
		switch {
		case errors.Is(err, ErrMissingTarget):
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "exactly one of reported_user_id or message_id is required")
		case errors.Is(err, ErrSelfReport):
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "cannot report yourself")
		case errors.Is(err, ErrInvalidReason):
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid reason")
		case errors.Is(err, ErrUserNotFound):
			apierror.Write(w, http.StatusNotFound, apierror.CodeUserNotFound, "reported user not found")
		case errors.Is(err, ErrMessageNotFound):
			apierror.Write(w, http.StatusNotFound, apierror.CodeMessageNotFound, "message not found")
		case errors.Is(err, ErrDuplicateReport):
			apierror.Write(w, http.StatusConflict, apierror.CodeAlreadyReported, "message already reported")
		default:
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
		}
		return
	}

	apierror.WriteJSON(w, http.StatusCreated, report)
}

// ListReports handles GET /reports?status=&priority= (admin only).
func (m *Manager) ListReports(w http.ResponseWriter, r *http.Request) {
	reports, err := m.svc.List(r.Context(), ListFilter{
		Status:   r.URL.Query().Get("status"),
		Priority: r.URL.Query().Get("priority"),
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidStatus):
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid status")
		case errors.Is(err, ErrInvalidPriority):
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid priority")
		default:
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
		}
		return
	}

	apierror.WriteJSON(w, http.StatusOK, reports)
}

// UpdateReportStatus handles PUT /reports/{id}/status (admin only).
// An optional action field ("suspend" or "ban") triggers a moderation action
// on the reported user after the report status is updated.
func (m *Manager) UpdateReportStatus(w http.ResponseWriter, r *http.Request) {
	reviewerID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
		return
	}

	reportID := chi.URLParam(r, "id")
	if reportID == "" {
		apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "missing report id")
		return
	}

	var req struct {
		Status       string `json:"status"`
		Action       string `json:"action"`        // "suspend", "ban", or ""
		DurationDays int    `json:"duration_days"` // for suspend
		Reason       string `json:"reason"`        // for suspend/ban
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
		return
	}

	if req.Status == "" {
		apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "status is required")
		return
	}

	if req.Action != "" && req.Action != "suspend" && req.Action != "ban" {
		apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid action")
		return
	}

	if req.Action != "" && req.Status != StatusReviewed {
		apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, ErrActionNeedsReview.Error())
		return
	}

	// The action runs before the status change, so a failed action never
	// leaves a report marked reviewed with nothing done. If the status write
	// fails afterwards, the action stands and the report stays pending for the
	// moderator to resolve again.
	if req.Action != "" && m.moderator != nil {
		existing, err := m.svc.GetByID(r.Context(), reportID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apierror.Write(w, http.StatusNotFound, apierror.CodeNotFound, "report not found")
				return
			}
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		if err := m.moderate(r.Context(), existing, req.Action, req.Reason, req.DurationDays, reviewerID); err != nil {
			switch {
			case errors.Is(err, ErrReportedUserGone):
				apierror.Write(w, http.StatusConflict, apierror.CodeReportedAccountGone, ErrReportedUserGone.Error())
			case errors.Is(err, ErrAlreadyModerated):
				apierror.Write(w, http.StatusConflict, apierror.CodeAlreadyModerated, ErrAlreadyModerated.Error())
			default:
				zerolog.Ctx(r.Context()).Error().Err(err).Str("report_id", reportID).Msg("reports: moderation action failed")
				apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			}
			return
		}
	}

	report, err := m.svc.UpdateStatus(r.Context(), reportID, req.Status, reviewerID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			apierror.Write(w, http.StatusNotFound, apierror.CodeNotFound, "report not found")
		case errors.Is(err, ErrInvalidStatus):
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid status")
		default:
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
		}
		return
	}

	apierror.WriteJSON(w, http.StatusOK, report)
}

// moderate applies a report's moderation action to the reported account.
func (m *Manager) moderate(ctx context.Context, report *Report, action, reason string, durationDays int, reviewerID string) error {
	if report.ReportedUserID == nil {
		return ErrReportedUserGone
	}
	target := *report.ReportedUserID
	if action == "suspend" {
		return m.moderator.SuspendUser(ctx, target, reason, durationDays, reviewerID)
	}
	return m.moderator.BanUser(ctx, target, reason, reviewerID)
}
