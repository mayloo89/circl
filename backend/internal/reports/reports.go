package reports

import (
	"context"
	"errors"
	"time"
)

// AdminModerator applies moderation actions to users.
type AdminModerator interface {
	SuspendUser(ctx context.Context, userID, reason string, durationDays int, adminID string) error
	BanUser(ctx context.Context, userID, reason, adminID string) error
}

// RateLimiter enforces call-rate limits keyed by an arbitrary string.
type RateLimiter interface {
	// Allow returns true if the action is within the allowed rate.
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

// Valid report reasons. NCII and gender-violence categories were added in the
// trust-and-safety pass to back the safety page's promises (Ley 27.736
// "Ley Olimpia" in AR; StopNCII partner stack). CSAM is included so admin can
// triage out-of-band escalations even though direct user reporting is not the
// primary discovery channel.
const (
	ReasonHarassment           = "harassment"
	ReasonSpam                 = "spam"
	ReasonInappropriateContent = "inappropriate_content"
	ReasonFakeProfile          = "fake_profile"
	ReasonNCII                 = "non_consensual_intimate_images"
	ReasonGenderViolence       = "digital_gender_violence"
	ReasonCSAM                 = "csam"
	ReasonOther                = "other"
)

// Valid report statuses.
const (
	StatusPending   = "pending"
	StatusReviewed  = "reviewed"
	StatusDismissed = "dismissed"
)

// Triage priorities. CSAM and NCII go straight to the critical queue;
// gender-violence reports surface above ordinary harassment without competing
// with imminent-harm categories for attention.
const (
	PriorityNormal   = "normal"
	PriorityHigh     = "high"
	PriorityCritical = "critical"
)

// PriorityFor returns the queue priority a report should land in based on its
// reason. The store also writes this column at insert time so admin lists can
// sort on it without recomputing.
func PriorityFor(reason string) string {
	switch reason {
	case ReasonCSAM, ReasonNCII:
		return PriorityCritical
	case ReasonGenderViolence:
		return PriorityHigh
	default:
		return PriorityNormal
	}
}

// Sentinel errors returned by the service and store layers.
var (
	ErrNotFound        = errors.New("report not found")
	ErrSelfReport      = errors.New("cannot report yourself")
	ErrInvalidReason   = errors.New("invalid report reason")
	ErrInvalidStatus   = errors.New("invalid report status")
	ErrInvalidPriority = errors.New("invalid report priority")
)

// Report represents a user report.
type Report struct {
	ID             string     `json:"id"`
	ReporterID     string     `json:"reporter_id"`
	ReportedUserID string     `json:"reported_user_id"`
	Reason         string     `json:"reason"`
	Priority       string     `json:"priority"`
	Description    string     `json:"description"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	ReviewedAt     *time.Time `json:"reviewed_at,omitzero"`
	ReviewedBy     *string    `json:"reviewed_by,omitempty"`
}

// ReportWithUserInfo includes reported user details for display.
type ReportWithUserInfo struct {
	ID             string     `json:"id"`
	ReporterID     string     `json:"reporter_id"`
	ReportedUserID string     `json:"reported_user_id"`
	ReportedEmail  string     `json:"reported_email"`
	ReportedName   string     `json:"reported_name"`
	ReportedAvatar string     `json:"reported_avatar"`
	Reason         string     `json:"reason"`
	Priority       string     `json:"priority"`
	Description    string     `json:"description"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	ReviewedAt     *time.Time `json:"reviewed_at,omitzero"`
	ReviewedBy     *string    `json:"reviewed_by,omitempty"`
}

// ListFilter narrows a report list query. Zero-valued fields are ignored.
type ListFilter struct {
	Status   string
	Priority string
}

// Store is the persistence interface required by the service.
type Store interface {
	// Create creates a new report with the given priority.
	Create(ctx context.Context, reporterID, reportedUserID, reason, priority, description string) (*Report, error)
	// GetByID retrieves a report by ID.
	GetByID(ctx context.Context, id string) (*Report, error)
	// List returns reports matching the filter. Results are ordered with
	// critical first, then high, then normal, and newest-first within each
	// priority bucket so admin sees the worst items at the top of the table.
	List(ctx context.Context, f ListFilter) ([]ReportWithUserInfo, error)
	// UpdateStatus updates a report's status and records who reviewed it.
	UpdateStatus(ctx context.Context, id, status, reviewedBy string) (*Report, error)
}

// Service implements the reports business logic.
type Service struct {
	store Store
}

// NewService creates a Service backed by the given store.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// Create creates a new report. Priority is derived from the reason so callers
// can't downgrade a CSAM or NCII report to the normal queue.
func (s *Service) Create(ctx context.Context, reporterID, reportedUserID, reason, description string) (*Report, error) {
	if reporterID == reportedUserID {
		return nil, ErrSelfReport
	}
	if !isValidReason(reason) {
		return nil, ErrInvalidReason
	}
	return s.store.Create(ctx, reporterID, reportedUserID, reason, PriorityFor(reason), description)
}

// GetByID retrieves a report by ID.
func (s *Service) GetByID(ctx context.Context, id string) (*Report, error) {
	return s.store.GetByID(ctx, id)
}

// List returns reports matching the given filter.
func (s *Service) List(ctx context.Context, f ListFilter) ([]ReportWithUserInfo, error) {
	if f.Status != "" && !isValidStatus(f.Status) {
		return nil, ErrInvalidStatus
	}
	if f.Priority != "" && !isValidPriority(f.Priority) {
		return nil, ErrInvalidPriority
	}
	return s.store.List(ctx, f)
}

// UpdateStatus updates a report's status.
func (s *Service) UpdateStatus(ctx context.Context, id, status, reviewedBy string) (*Report, error) {
	if !isValidStatus(status) {
		return nil, ErrInvalidStatus
	}
	return s.store.UpdateStatus(ctx, id, status, reviewedBy)
}

func isValidReason(reason string) bool {
	switch reason {
	case ReasonHarassment, ReasonSpam, ReasonInappropriateContent, ReasonFakeProfile,
		ReasonNCII, ReasonGenderViolence, ReasonCSAM, ReasonOther:
		return true
	}
	return false
}

func isValidStatus(status string) bool {
	return status == StatusPending ||
		status == StatusReviewed ||
		status == StatusDismissed
}

func isValidPriority(p string) bool {
	return p == PriorityNormal || p == PriorityHigh || p == PriorityCritical
}
