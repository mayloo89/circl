package profiles

import (
	"context"
	"errors"
	"fmt"
)

const MaxProfilePhotos = 6

var (
	ErrNotFound     = errors.New("profile not found")
	ErrPhotoNotFound = errors.New("photo not found")
	ErrInvalidInput = errors.New("invalid input")
)

// ProfilePhoto is a single showcase photo on a user's profile.
type ProfilePhoto struct {
	ID  string
	URL string
}

// Profile represents a user's public profile data.
type Profile struct {
	ID          string
	UserID      string
	DisplayName string
	Bio         string
	AvatarURL   string
	Photos      []ProfilePhoto
}

// Store is the data-access interface required by the profiles service.
type Store interface {
	GetByUserID(ctx context.Context, userID string) (*Profile, error)
	Upsert(ctx context.Context, userID, displayName, bio, avatarURL string) (*Profile, error)
	GetPhotosByUserID(ctx context.Context, userID string) ([]ProfilePhoto, error)
	CountPhotos(ctx context.Context, userID string) (int, error)
	AddPhoto(ctx context.Context, userID, url string) (*ProfilePhoto, error)
	DeletePhoto(ctx context.Context, photoID, userID string) error
}

// Service handles profile business logic.
type Service struct {
	store Store
}

// NewService creates a new profiles Service.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// GetMyProfile returns the profile (with photos) for the given user.
// If no profile exists yet, an empty one is created automatically.
func (s *Service) GetMyProfile(ctx context.Context, userID string) (*Profile, error) {
	profile, err := s.store.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			profile, err = s.store.Upsert(ctx, userID, "", "", "")
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	photos, err := s.store.GetPhotosByUserID(ctx, profile.UserID)
	if err != nil {
		return nil, err
	}
	profile.Photos = photos
	return profile, nil
}

// GetPublicProfile returns the profile (with photos) for any user by ID.
// Returns ErrNotFound if the user has no profile.
func (s *Service) GetPublicProfile(ctx context.Context, userID string) (*Profile, error) {
	profile, err := s.store.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	photos, err := s.store.GetPhotosByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	profile.Photos = photos
	return profile, nil
}

// UpdateMyProfile updates the display name, bio, and avatar for the given user.
// display_name is required.
func (s *Service) UpdateMyProfile(ctx context.Context, userID, displayName, bio, avatarURL string) (*Profile, error) {
	if displayName == "" {
		return nil, fmt.Errorf("%w: display name is required", ErrInvalidInput)
	}
	profile, err := s.store.Upsert(ctx, userID, displayName, bio, avatarURL)
	if err != nil {
		return nil, err
	}
	photos, err := s.store.GetPhotosByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	profile.Photos = photos
	return profile, nil
}

// AddPhoto adds a showcase photo for the given user.
// Returns ErrInvalidInput if the user already has MaxProfilePhotos.
func (s *Service) AddPhoto(ctx context.Context, userID, url string) (*ProfilePhoto, error) {
	if url == "" {
		return nil, fmt.Errorf("%w: url is required", ErrInvalidInput)
	}
	count, err := s.store.CountPhotos(ctx, userID)
	if err != nil {
		return nil, err
	}
	if count >= MaxProfilePhotos {
		return nil, fmt.Errorf("%w: maximum %d photos reached", ErrInvalidInput, MaxProfilePhotos)
	}
	return s.store.AddPhoto(ctx, userID, url)
}

// DeletePhoto removes a showcase photo, verifying ownership.
func (s *Service) DeletePhoto(ctx context.Context, userID, photoID string) error {
	return s.store.DeletePhoto(ctx, photoID, userID)
}
