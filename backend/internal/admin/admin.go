package admin

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrUserNotFound is returned when the target user does not exist.
	ErrUserNotFound = errors.New("user not found")
	// ErrAlreadySuspended is returned when the user is already suspended or banned.
	ErrAlreadySuspended = errors.New("user already suspended or banned")
)

// UserRecord holds the admin view of a user.
type UserRecord struct {
	ID        string
	Email     string
	Status    string
	IsAdmin   bool
	CreatedAt time.Time
}

// Suspension records a moderation action against a user.
type Suspension struct {
	ID             string
	UserID         string
	SuspendedUntil *time.Time
	Reason         string
	CreatedBy      string
	CreatedAt      time.Time
}

// Store is the data-access interface for admin/moderation operations.
type Store interface {
	GetUserByID(ctx context.Context, userID string) (*UserRecord, error)
	SetUserStatus(ctx context.Context, userID, status string) error
	CreateSuspension(ctx context.Context, userID string, suspendedUntil *time.Time, reason, createdBy string) (*Suspension, error)
	// IsActiveUser satisfies middleware.UserStatusChecker.
	IsActiveUser(ctx context.Context, userID string) (bool, error)
}

// Service wraps the admin Store with business logic.
type Service struct {
	store Store
}

// NewService creates a Service backed by the given Store.
func NewService(store Store) *Service {
	return &Service{store: store}
}

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
	_, err = s.store.CreateSuspension(ctx, userID, until, reason, adminID)
	return err
}

// BanUser permanently sets the user's status to 'banned' and records the action.
func (s *Service) BanUser(ctx context.Context, userID, reason, adminID string) error {
	if err := s.store.SetUserStatus(ctx, userID, "banned"); err != nil {
		return err
	}
	_, err := s.store.CreateSuspension(ctx, userID, nil, reason, adminID)
	return err
}
