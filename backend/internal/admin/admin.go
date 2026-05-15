package admin

import (
	"context"
	"errors"
	"time"

	"github.com/mayloo89/circl/backend/internal/token"
)

var (
	// ErrUserNotFound is returned when the target user does not exist.
	ErrUserNotFound = errors.New("user not found")
	// ErrAlreadySuspended is returned when the user is already suspended or banned.
	ErrAlreadySuspended = errors.New("user already suspended or banned")
	// ErrChannelNotFound is returned when the target channel does not exist.
	ErrChannelNotFound = errors.New("channel not found")
	// ErrChannelNameTaken is returned when a channel with the same name already exists.
	ErrChannelNameTaken = errors.New("channel name already taken")
	// ErrInvalidRole is returned when the given role string is not valid.
	ErrInvalidRole = errors.New("invalid role")
)

// UserRecord holds the admin view of a user.
type UserRecord struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Status      string    `json:"status"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

// UserPresence is the live activity snapshot the admin user list overlays
// on top of each UserRecord. `Online` reflects the Redis presence keyset
// (TTL'd by the heartbeat) and `LastSeenAt` mirrors `users.last_seen_at`.
// Both fields default to their zero values for users with no presence data.
type UserPresence struct {
	Online     bool       `json:"online"`
	LastSeenAt *time.Time `json:"last_seen_at,omitzero"`
}

// PresenceLookupFunc returns live presence info keyed by user ID. Users
// without a presence row are absent from the map; callers treat absence
// as offline / no last-seen. Implemented in main.go on top of the
// presence package's Store.GetPresence so admin doesn't take a direct
// dependency on the presence package.
type PresenceLookupFunc func(ctx context.Context, userIDs []string) (map[string]UserPresence, error)

// Suspension records a moderation action against a user.
type Suspension struct {
	ID             string
	UserID         string
	SuspendedUntil *time.Time
	Reason         string
	CreatedBy      string
	CreatedAt      time.Time
}

// ChannelRecord holds the admin view of a public channel.
type ChannelRecord struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatorID   string    `json:"creator_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// Stats holds aggregate counts for the admin dashboard.
type Stats struct {
	TotalUsers     int `json:"total_users"`
	ActiveUsers    int `json:"active_users"`
	SuspendedUsers int `json:"suspended_users"`
	BannedUsers    int `json:"banned_users"`
	DeletedUsers   int `json:"deleted_users"`
	TotalReports   int `json:"total_reports"`
	PendingReports int `json:"pending_reports"`
	TotalRooms     int `json:"total_rooms"`
}

// Store is the data-access interface for admin/moderation operations.
type Store interface {
	GetUserByID(ctx context.Context, userID string) (*UserRecord, error)
	SetUserStatus(ctx context.Context, userID, status string) error
	CreateSuspension(ctx context.Context, userID string, suspendedUntil *time.Time, reason, createdBy string) (*Suspension, error)
	// IsActiveUser satisfies middleware.UserStatusChecker.
	IsActiveUser(ctx context.Context, userID string) (bool, error)
	GetStats(ctx context.Context) (*Stats, error)
	ListUsers(ctx context.Context, query, status string, limit, offset int) ([]*UserRecord, int, error)
	ReactivateUser(ctx context.Context, userID string) error
	ListChannels(ctx context.Context) ([]ChannelRecord, error)
	DeleteChannel(ctx context.Context, channelID string) error
	CreateChannel(ctx context.Context, adminID, name, description string) (*ChannelRecord, error)
	UpdateChannel(ctx context.Context, channelID, name, description string) error
	// HardDeleteUser immediately purges all user data and anonymizes the users row.
	HardDeleteUser(ctx context.Context, userID string) error
	// SetUserRole updates the role of an existing user.
	SetUserRole(ctx context.Context, userID, role string) error
}

// SuspensionNotifier is the optional hook called after a successful suspension
// or ban. The notifier is responsible for creating the appeal record and
// emailing the user the appeal link. Failures are logged by the notifier and
// must not block the suspension — admin already saw the action succeed.
type SuspensionNotifier interface {
	NotifyOfSuspension(ctx context.Context, user UserRecord, suspension Suspension)
}

// Service wraps the admin Store with business logic.
type Service struct {
	store    Store
	notifier SuspensionNotifier
}

// NewService creates a Service backed by the given Store.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// SetSuspensionNotifier wires the appeal hook used by SuspendUser and BanUser.
// Done as a setter rather than a constructor option because the notifier
// depends on services initialised after the admin store.
func (s *Service) SetSuspensionNotifier(n SuspensionNotifier) { s.notifier = n }

// IsActiveUser implements middleware.UserStatusChecker.
func (s *Service) IsActiveUser(ctx context.Context, userID string) (bool, error) {
	return s.store.IsActiveUser(ctx, userID)
}

// SuspendUser sets the user's status to 'suspended' and records the suspension.
// durationDays == 0 means permanent (suspended_until is NULL).
func (s *Service) SuspendUser(ctx context.Context, userID, reason string, durationDays int, adminID string) error {
	u, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if u.Status == "banned" || u.Status == "suspended" {
		return ErrAlreadySuspended
	}
	var until *time.Time
	if durationDays > 0 {
		t := time.Now().UTC().AddDate(0, 0, durationDays)
		until = &t
	}
	if err := s.store.SetUserStatus(ctx, userID, "suspended"); err != nil {
		return err
	}
	susp, err := s.store.CreateSuspension(ctx, userID, until, reason, adminID)
	if err != nil {
		return err
	}
	s.notifySuspension(ctx, *u, *susp)
	return nil
}

// BanUser permanently sets the user's status to 'banned' and records the action.
func (s *Service) BanUser(ctx context.Context, userID, reason, adminID string) error {
	u, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := s.store.SetUserStatus(ctx, userID, "banned"); err != nil {
		return err
	}
	susp, err := s.store.CreateSuspension(ctx, userID, nil, reason, adminID)
	if err != nil {
		return err
	}
	s.notifySuspension(ctx, *u, *susp)
	return nil
}

func (s *Service) notifySuspension(ctx context.Context, user UserRecord, susp Suspension) {
	if s.notifier != nil {
		s.notifier.NotifyOfSuspension(ctx, user, susp)
	}
}

// GetStats returns aggregate counts for the admin dashboard.
func (s *Service) GetStats(ctx context.Context) (*Stats, error) {
	return s.store.GetStats(ctx)
}

// ListUsers returns a paginated list of users with optional search and status filter.
func (s *Service) ListUsers(ctx context.Context, query, status string, limit, offset int) ([]*UserRecord, int, error) {
	return s.store.ListUsers(ctx, query, status, limit, offset)
}

// ReactivateUser sets the user's status back to 'active'.
func (s *Service) ReactivateUser(ctx context.Context, userID string) error {
	return s.store.ReactivateUser(ctx, userID)
}

// ListChannels returns all public channel rooms.
func (s *Service) ListChannels(ctx context.Context) ([]ChannelRecord, error) {
	return s.store.ListChannels(ctx)
}

// DeleteChannel removes a channel room and all its messages.
func (s *Service) DeleteChannel(ctx context.Context, channelID string) error {
	return s.store.DeleteChannel(ctx, channelID)
}

// CreateChannel creates a new public channel room owned by the admin.
func (s *Service) CreateChannel(ctx context.Context, adminID, name, description string) (*ChannelRecord, error) {
	return s.store.CreateChannel(ctx, adminID, name, description)
}

// UpdateChannel changes the name and description of an existing channel.
func (s *Service) UpdateChannel(ctx context.Context, channelID, name, description string) error {
	return s.store.UpdateChannel(ctx, channelID, name, description)
}

// HardDeleteUser immediately purges all user data and anonymizes the users row.
// Unlike the self-delete flow, there is no grace period.
func (s *Service) HardDeleteUser(ctx context.Context, userID string) error {
	return s.store.HardDeleteUser(ctx, userID)
}

// SetUserRole updates the role of an existing user.
// Valid roles are "user", "admin", and "super_admin".
func (s *Service) SetUserRole(ctx context.Context, userID, role string) error {
	switch role {
	case token.RoleUser, token.RoleAdmin, token.RoleSuperAdmin:
	default:
		return ErrInvalidRole
	}
	return s.store.SetUserRole(ctx, userID, role)
}
