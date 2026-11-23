package reports

import (
	"context"
	"errors"
	"time"
)

// AdminModerator applies moderation actions to users.
// Implementations return an error wrapping ErrReportedUserGone when the account
// was purged or deleted, and ErrAlreadyModerated when it is already suspended
// or banned.
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
	// ErrMessageNotFound covers both a missing message and one the reporter
	// cannot see, so the API never confirms a message exists to a non-member.
	ErrMessageNotFound = errors.New("message not found")
	ErrUserNotFound    = errors.New("reported user not found")
	ErrDuplicateReport = errors.New("message already reported")
	ErrMissingTarget   = errors.New("report needs exactly one of reported_user_id or message_id")
	// ErrReportedUserGone is returned when an action targets an account that
	// was hard-deleted after the report was filed.
	ErrReportedUserGone = errors.New("reported account no longer exists")
	ErrAlreadyModerated = errors.New("reported account already suspended or banned")
	// ErrActionNeedsReview rejects a moderation action on a dismissal, which
	// would act on the account while wiping the evidence behind it.
	ErrActionNeedsReview = errors.New("moderation actions require status reviewed")
)

// Report represents a user report. ReporterID and ReportedUserID are nil only
// when the account was hard-deleted after the report was filed.
type Report struct {
	ID             string     `json:"id"`
	ReporterID     *string    `json:"reporter_id"`
	ReportedUserID *string    `json:"reported_user_id"`
	MessageID      *string    `json:"message_id,omitempty"`
	Reason         string     `json:"reason"`
	Priority       string     `json:"priority"`
	Description    string     `json:"description"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	ReviewedAt     *time.Time `json:"reviewed_at,omitzero"`
	ReviewedBy     *string    `json:"reviewed_by,omitempty"`
}

// ReportedMessage is the copy of a reported message taken when the report was
// filed. Content is nil once a dismissal has purged it.
type ReportedMessage struct {
	ID       string     `json:"id"`
	RoomID   string     `json:"room_id"`
	Type     string     `json:"type"`
	Content  *string    `json:"content"`
	SentAt   time.Time  `json:"sent_at"`
	PurgedAt *time.Time `json:"purged_at,omitzero"`
}

// ReportWithUserInfo includes reported user details for display. Identity
// fields prefer the live account and fall back to the snapshot taken at report
// time, so a purged account still shows who was reported. ReportedAccountGone
// tells the admin UI not to link to a profile that no longer exists.
type ReportWithUserInfo struct {
	ID                  string           `json:"id"`
	ReporterID          *string          `json:"reporter_id"`
	ReportedUserID      *string          `json:"reported_user_id"`
	ReportedEmail       string           `json:"reported_email"`
	ReportedName        string           `json:"reported_name"`
	ReportedUsername    string           `json:"reported_username"`
	ReportedAvatar      string           `json:"reported_avatar"`
	ReportedAccountGone bool             `json:"reported_account_gone"`
	Message             *ReportedMessage `json:"message,omitempty"`
	Reason              string           `json:"reason"`
	Priority            string           `json:"priority"`
	Description         string           `json:"description"`
	Status              string           `json:"status"`
	CreatedAt           time.Time        `json:"created_at"`
	ReviewedAt          *time.Time       `json:"reviewed_at,omitzero"`
	ReviewedBy          *string          `json:"reviewed_by,omitempty"`
}

// ListFilter narrows a report list query. Zero-valued fields are ignored.
type ListFilter struct {
	Status   string
	Priority string
}

// Store is the persistence interface required by the service.
type Store interface {
	// Create files a report against a user, snapshotting their identity.
	Create(ctx context.Context, reporterID, reportedUserID, reason, priority, description string) (*Report, error)
	// CreateForMessage files a report against a message. The reported user is
	// the message's sender; the message and sender identity are snapshotted.
	// Returns ErrMessageNotFound when the message is missing, tombstoned, or in
	// a room the reporter is not a member of.
	CreateForMessage(ctx context.Context, reporterID, messageID, reason, priority, description string) (*Report, error)
	// GetByID retrieves a report by ID.
	GetByID(ctx context.Context, id string) (*Report, error)
	// List returns reports matching the filter. Results are ordered with
	// critical first, then high, then normal, and newest-first within each
	// priority bucket so admin sees the worst items at the top of the table.
	List(ctx context.Context, f ListFilter) ([]ReportWithUserInfo, error)
	// UpdateStatus updates a report's status and records who reviewed it.
	// Dismissing a report purges its message copy.
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

// Target is what a report is filed against: exactly one of a user or a message.
type Target struct {
	UserID    string
	MessageID string
}

// Create files a report. Priority is derived from the reason so callers can't
// downgrade a CSAM or NCII report to the normal queue.
func (s *Service) Create(ctx context.Context, reporterID string, target Target, reason, description string) (*Report, error) {
	if (target.UserID == "") == (target.MessageID == "") {
		return nil, ErrMissingTarget
	}
	if !isValidReason(reason) {
		return nil, ErrInvalidReason
	}
	if target.MessageID != "" {
		return s.store.CreateForMessage(ctx, reporterID, target.MessageID, reason, PriorityFor(reason), description)
	}
	if reporterID == target.UserID {
		return nil, ErrSelfReport
	}
	return s.store.Create(ctx, reporterID, target.UserID, reason, PriorityFor(reason), description)
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
