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

// Valid report reasons.
const (
	ReasonHarassment           = "harassment"
	ReasonSpam                 = "spam"
	ReasonInappropriateContent = "inappropriate_content"
	ReasonFakeProfile          = "fake_profile"
	ReasonOther                = "other"
)

// Valid report statuses.
const (
	StatusPending   = "pending"
	StatusReviewed  = "reviewed"
	StatusDismissed = "dismissed"
)

// Sentinel errors returned by the service and store layers.
var (
	ErrNotFound      = errors.New("report not found")
	ErrSelfReport    = errors.New("cannot report yourself")
	ErrInvalidReason = errors.New("invalid report reason")
	ErrInvalidStatus = errors.New("invalid report status")
)

// Report represents a user report.
type Report struct {
	ID             string     `json:"id"`
	ReporterID     string     `json:"reporter_id"`
	ReportedUserID string     `json:"reported_user_id"`
	Reason         string     `json:"reason"`
	Description    string     `json:"description"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	ReviewedAt     *time.Time `json:"reviewed_at,omitempty"`
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
	Description    string     `json:"description"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	ReviewedAt     *time.Time `json:"reviewed_at,omitempty"`
	ReviewedBy     *string    `json:"reviewed_by,omitempty"`
}

// Store is the persistence interface required by the service.
type Store interface {
	// Create creates a new report.
	Create(ctx context.Context, reporterID, reportedUserID, reason, description string) (*Report, error)
	// GetByID retrieves a report by ID.
	GetByID(ctx context.Context, id string) (*Report, error)
	// List returns all reports with optional status filter, newest first.
	List(ctx context.Context, status string) ([]ReportWithUserInfo, error)
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

// Create creates a new report.
func (s *Service) Create(ctx context.Context, reporterID, reportedUserID, reason, description string) (*Report, error) {
	if reporterID == reportedUserID {
		return nil, ErrSelfReport
	}
	if !isValidReason(reason) {
		return nil, ErrInvalidReason
	}
	return s.store.Create(ctx, reporterID, reportedUserID, reason, description)
}

// GetByID retrieves a report by ID.
func (s *Service) GetByID(ctx context.Context, id string) (*Report, error) {
	return s.store.GetByID(ctx, id)
}

// List returns all reports with optional status filter.
func (s *Service) List(ctx context.Context, status string) ([]ReportWithUserInfo, error) {
	if status != "" && !isValidStatus(status) {
		return nil, ErrInvalidStatus
	}
	return s.store.List(ctx, status)
}

// UpdateStatus updates a report's status.
func (s *Service) UpdateStatus(ctx context.Context, id, status, reviewedBy string) (*Report, error) {
	if !isValidStatus(status) {
		return nil, ErrInvalidStatus
	}
	return s.store.UpdateStatus(ctx, id, status, reviewedBy)
}

func isValidReason(reason string) bool {
	return reason == ReasonHarassment ||
		reason == ReasonSpam ||
		reason == ReasonInappropriateContent ||
		reason == ReasonFakeProfile ||
		reason == ReasonOther
}

func isValidStatus(status string) bool {
	return status == StatusPending ||
		status == StatusReviewed ||
		status == StatusDismissed
}
