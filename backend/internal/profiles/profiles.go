package profiles

import (
	"context"
	"errors"
)

var (
	ErrNotFound    = errors.New("profile not found")
	ErrInvalidInput = errors.New("invalid input")
)

// Profile represents a user's public profile data.
type Profile struct {
	ID          string
	UserID      string
	DisplayName string
	Bio         string
}

// Store is the data-access interface required by the profiles service.
type Store interface {
	GetByUserID(ctx context.Context, userID string) (*Profile, error)
	Upsert(ctx context.Context, userID, displayName, bio string) (*Profile, error)
}

// Service handles profile business logic.
type Service struct {
	store Store
}

// NewService creates a new profiles Service.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// GetMyProfile returns the profile for the given user.
// If no profile exists yet, an empty one is created automatically.
func (s *Service) GetMyProfile(ctx context.Context, userID string) (*Profile, error) {
	profile, err := s.store.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return s.store.Upsert(ctx, userID, "", "")
		}
		return nil, err
	}
	return profile, nil
}

// UpdateMyProfile updates the display name and bio for the given user.
// display_name is required.
func (s *Service) UpdateMyProfile(ctx context.Context, userID, displayName, bio string) (*Profile, error) {
	if displayName == "" {
		return nil, errors.New("display name is required: " + ErrInvalidInput.Error())
	}
	return s.store.Upsert(ctx, userID, displayName, bio)
}
