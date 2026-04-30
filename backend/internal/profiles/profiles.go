package profiles

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
)

const (
	MaxProfilePhotos = 6
	MaxInterests     = 20
)

var (
	ErrNotFound        = errors.New("profile not found")
	ErrPhotoNotFound   = errors.New("photo not found")
	ErrInvalidInput    = errors.New("invalid input")
	ErrUsernameTaken   = errors.New("username already taken")

	usernameRe = regexp.MustCompile(`^[a-z0-9_]{3,30}$`)
)

// ProfilePhoto is a single showcase photo on a user's profile.
type ProfilePhoto struct {
	ID  string
	URL string
}

// Profile represents a user's public profile data.
type Profile struct {
	ID           string
	UserID       string
	Username     string
	DisplayName  string
	Bio          string
	AvatarURL    string
	DateOfBirth  *time.Time
	Gender       string
	LocationText string
	Latitude     *float64
	Longitude    *float64
	Interests    []string
	Photos       []ProfilePhoto
	OnboardedAt  *time.Time
}

// ProfileInput holds the editable fields for profile create/update.
type ProfileInput struct {
	Username     string
	DisplayName  string
	Bio          string
	AvatarURL    string
	DateOfBirth  *time.Time
	Gender       string
	LocationText string
	Latitude     *float64
	Longitude    *float64
	Interests    []string
	OnboardedAt  *time.Time
}

// ProfilePreferences holds discovery preferences for a user.
type ProfilePreferences struct {
	UserID           string
	MinAge           *int
	MaxAge           *int
	MaxDistanceKm    *int
	GenderPreference []string
	Locale           string
}

// InterestSuggestion is a suggested interest with its global usage count.
type InterestSuggestion struct {
	Name  string
	Count int
}

// BrowseProfile is a profile summary for the browse/explore view.
type BrowseProfile struct {
	ID            string
	UserID        string
	Username      string
	DisplayName   string
	AvatarURL     string
	DateOfBirth   *time.Time
	Age           *int
	Gender        string
	LocationText  string
	DistanceKm    *float64
	FirstPhotoURL string
	Interests     []string
	// CreatedAt is used internally to encode the next-page cursor; not serialised to the API.
	CreatedAt time.Time
}

// BrowsePage is a paginated set of browse results.
// NextCursor is an opaque token to pass as ?cursor= on the next request.
// An empty NextCursor means there are no more pages.
type BrowsePage struct {
	Profiles   []BrowseProfile
	NextCursor string
}

// browseCursor holds the keyset values needed to continue a Browse query.
type browseCursor struct {
	CreatedAt  time.Time `json:"ca"`
	ID         string    `json:"id"`
	DistanceKm *float64  `json:"dk,omitempty"`
}

// EncodeBrowseCursor encodes the last profile of a page into an opaque cursor string.
func EncodeBrowseCursor(p BrowseProfile, sortByDistance bool) string {
	c := browseCursor{CreatedAt: p.CreatedAt, ID: p.ID}
	if sortByDistance {
		c.DistanceKm = p.DistanceKm
	}
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

// DecodeBrowseCursor decodes an opaque cursor string produced by EncodeBrowseCursor.
func DecodeBrowseCursor(s string) (browseCursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return browseCursor{}, err
	}
	var c browseCursor
	return c, json.Unmarshal(b, &c)
}

// Store is the data-access interface required by the profiles service.
type Store interface {
	GetByUserID(ctx context.Context, userID string) (*Profile, error)
	GetByUsername(ctx context.Context, username string) (*Profile, error)
	IsUsernameAvailable(ctx context.Context, username string) (bool, error)
	Upsert(ctx context.Context, userID string, in ProfileInput) (*Profile, error)
	SyncInterests(ctx context.Context, userID string, names []string) error
	UpdateAvatar(ctx context.Context, userID, avatarURL string) error
	GetPhotosByUserID(ctx context.Context, userID string) ([]ProfilePhoto, error)
	CountPhotos(ctx context.Context, userID string) (int, error)
	AddPhoto(ctx context.Context, userID, url string) (*ProfilePhoto, error)
	DeletePhoto(ctx context.Context, photoID, userID string) error
	GetPreferences(ctx context.Context, userID string) (*ProfilePreferences, error)
	UpsertPreferences(ctx context.Context, userID string, prefs ProfilePreferences) (*ProfilePreferences, error)
	SearchInterests(ctx context.Context, query string, limit int) ([]InterestSuggestion, error)
	Browse(ctx context.Context, userID string, limit int, cursor string, sortByDistance bool, interests []string) ([]BrowseProfile, error)
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
			profile, err = s.store.Upsert(ctx, userID, ProfileInput{})
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
	photos, err := s.store.GetPhotosByUserID(ctx, profile.UserID)
	if err != nil {
		return nil, err
	}
	profile.Photos = photos
	return profile, nil
}

// GetPublicProfileByUsername returns the profile (with photos) for any user by username.
// Returns ErrNotFound if the username does not exist.
func (s *Service) GetPublicProfileByUsername(ctx context.Context, username string) (*Profile, error) {
	profile, err := s.store.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	photos, err := s.store.GetPhotosByUserID(ctx, profile.UserID)
	if err != nil {
		return nil, err
	}
	profile.Photos = photos
	return profile, nil
}

// IsUsernameAvailable returns true if the username is valid format and not taken.
func (s *Service) IsUsernameAvailable(ctx context.Context, username string) (bool, error) {
	if !usernameRe.MatchString(username) {
		return false, nil
	}
	return s.store.IsUsernameAvailable(ctx, username)
}

// UpdateMyProfile validates and updates the profile for the given user.
func (s *Service) UpdateMyProfile(ctx context.Context, userID string, in ProfileInput) (*Profile, error) {
	if in.Username != "" && !usernameRe.MatchString(in.Username) {
		return nil, fmt.Errorf("%w: username must be 3–30 characters, lowercase letters, digits, or underscores", ErrInvalidInput)
	}
	if in.DateOfBirth == nil {
		return nil, fmt.Errorf("%w: date of birth is required", ErrInvalidInput)
	}
	if !isAtLeast18(*in.DateOfBirth) {
		return nil, fmt.Errorf("%w: must be at least 18 years old", ErrInvalidInput)
	}
	if len(in.Interests) > MaxInterests {
		return nil, fmt.Errorf("%w: maximum %d interests allowed", ErrInvalidInput, MaxInterests)
	}
	profile, err := s.store.Upsert(ctx, userID, in)
	if err != nil {
		return nil, err
	}
	interests := in.Interests
	if interests == nil {
		interests = []string{}
	}
	if err := s.store.SyncInterests(ctx, userID, interests); err != nil {
		return nil, err
	}
	profile.Interests = interests
	photos, err := s.store.GetPhotosByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	profile.Photos = photos
	return profile, nil
}

// SearchInterests returns interest suggestions matching the given prefix, ordered by usage.
func (s *Service) SearchInterests(ctx context.Context, query string) ([]InterestSuggestion, error) {
	return s.store.SearchInterests(ctx, query, 10)
}

// UpdateAvatar updates only the avatar URL for the given user.
func (s *Service) UpdateAvatar(ctx context.Context, userID, avatarURL string) error {
	return s.store.UpdateAvatar(ctx, userID, avatarURL)
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

// GetMyPreferences returns the discovery preferences for the given user.
func (s *Service) GetMyPreferences(ctx context.Context, userID string) (*ProfilePreferences, error) {
	return s.store.GetPreferences(ctx, userID)
}

// UpdateMyPreferences validates and persists discovery preferences.
func (s *Service) UpdateMyPreferences(ctx context.Context, userID string, prefs ProfilePreferences) (*ProfilePreferences, error) {
	if prefs.MinAge != nil && *prefs.MinAge < 18 {
		return nil, fmt.Errorf("%w: min_age must be at least 18", ErrInvalidInput)
	}
	if prefs.MaxAge != nil && *prefs.MaxAge > 120 {
		return nil, fmt.Errorf("%w: max_age must be at most 120", ErrInvalidInput)
	}
	if prefs.MinAge != nil && prefs.MaxAge != nil && *prefs.MinAge > *prefs.MaxAge {
		return nil, fmt.Errorf("%w: min_age must be less than or equal to max_age", ErrInvalidInput)
	}
	if prefs.MaxDistanceKm != nil && *prefs.MaxDistanceKm <= 0 {
		return nil, fmt.Errorf("%w: max_distance_km must be positive", ErrInvalidInput)
	}
	return s.store.UpsertPreferences(ctx, userID, prefs)
}

// Browse returns a cursor-paginated list of profiles visible to the given user,
// filtered by their stored discovery preferences. cursor is an opaque token
// returned by a previous call; pass "" to start from the first page.
func (s *Service) Browse(ctx context.Context, userID string, limit int, cursor string, sortByDistance bool, interests []string) (*BrowsePage, error) {
	profiles, err := s.store.Browse(ctx, userID, limit+1, cursor, sortByDistance, interests)
	if err != nil {
		return nil, err
	}
	var nextCursor string
	if len(profiles) > limit {
		profiles = profiles[:limit]
		nextCursor = EncodeBrowseCursor(profiles[limit-1], sortByDistance)
	}
	now := time.Now()
	for i := range profiles {
		if profiles[i].DateOfBirth != nil {
			a := calcAge(*profiles[i].DateOfBirth, now)
			profiles[i].Age = &a
		}
	}
	return &BrowsePage{Profiles: profiles, NextCursor: nextCursor}, nil
}

// isAtLeast18 returns true if the given birth date is at least 18 years in the past.
func isAtLeast18(dob time.Time) bool {
	return !dob.After(time.Now().AddDate(-18, 0, 0))
}

func calcAge(dob, now time.Time) int {
	a := now.Year() - dob.Year()
	if now.Month() < dob.Month() || (now.Month() == dob.Month() && now.Day() < dob.Day()) {
		a--
	}
	return a
}
