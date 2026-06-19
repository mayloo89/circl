package profiles

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
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
	// ErrMediaNotApproved is returned when avatar/photo media has not cleared
	// moderation (still pending, rejected, or quarantined).
	ErrMediaNotApproved = errors.New("media not approved")

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
	// "Looking for" — public stated intent, distinct from the private
	// `profile_preferences` discovery filters. Empty array / nil ints mean
	// the user hasn't said.
	LookingForGender []string
	LookingForAgeMin *int
	LookingForAgeMax *int
}

// ProfileInput holds the editable fields for profile create/update.
type ProfileInput struct {
	Username         string
	DisplayName      string
	Bio              string
	AvatarURL        string
	DateOfBirth      *time.Time
	Gender           string
	LocationText     string
	Latitude         *float64
	Longitude        *float64
	Interests        []string
	OnboardedAt      *time.Time
	LookingForGender []string
	LookingForAgeMin *int
	LookingForAgeMax *int
}

// LookingForGenders is the canonical set the "Looking for" multi-select
// renders against. Matches the self-id gender list (minus "Custom") so
// users can express interest in the same identities the platform supports
// for self-id. "Prefer not to say" stays in for inclusion symmetry.
var LookingForGenders = []string{
	"Male",
	"Female",
	"Trans male",
	"Trans female",
	"Non-binary",
	"Prefer not to say",
}

// ProfilePreferences holds discovery, privacy, and notification preferences
// for a user.
type ProfilePreferences struct {
	UserID           string
	MinAge           *int
	MaxAge           *int
	MaxDistanceKm    *int
	GenderPreference []string
	Locale           string
	// Discovery filter — when true, browse hides profiles that don't have an
	// avatar. Default false (no filter applied).
	RequirePhoto bool
	// When true, this user's profile is hidden from others' browse results.
	// The user can still browse others. Default false.
	DiscoveryPaused bool
	// Privacy toggles. All default false.
	HideDistanceFromNonContacts bool
	HidePresence                bool
	HideReadReceipts            bool
	HideTypingIndicator         bool
	// Per-category notification toggles. All default true.
	NotifyChatMessages    bool
	NotifyContactRequests bool
	NotifyChannelMentions bool
	NotifySystem          bool
}

// PrivacyFlags is the read-only subset of privacy toggles used by services
// that gate behavior on a user's privacy preferences without needing the
// full preferences object (e.g. presence handler, chat hub, browse).
type PrivacyFlags struct {
	HideDistanceFromNonContacts bool
	HidePresence                bool
	HideReadReceipts            bool
	HideTypingIndicator         bool
}

// NotificationFlags is the read-only subset of per-category notification
// toggles consumed by the push gate. All fields default to true so users
// without a preferences row keep receiving notifications.
type NotificationFlags struct {
	ChatMessages    bool
	ContactRequests bool
	ChannelMentions bool
	System          bool
}

// Optional represents a JSON field that distinguishes "omitted" (Set=false),
// "explicit null" (Set=true, Value=nil), and "present value" (Set=true,
// Value=&v). Used by partial-update request payloads where the three
// states have distinct semantics — for example, the browse "Clear filters"
// affordance posts `{"min_age": null}` to clear, while a locale-only PUT
// omits min_age entirely and expects the existing value to be preserved.
type Optional[T any] struct {
	Set   bool
	Value *T
}

// UnmarshalJSON marks Set=true on any present field, including JSON null,
// and decodes a non-null value into Value.
func (o *Optional[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	if len(b) == 4 && string(b) == "null" {
		o.Value = nil
		return nil
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	o.Value = &v
	return nil
}

// PreferencesUpdate is the partial-update payload for UpsertPreferences.
// Each field's "is this field present in the request?" semantic is
// expressed differently:
//   - Optional[int] for the three nullable filter ints, since "set to
//     NULL" must be distinguishable from "omit".
//   - *T for fields whose null state is meaningless (the caller would
//     never post `{"locale": null}` or `{"hide_presence": null}`); a nil
//     pointer there means "omit, preserve existing".
//   - *[]string for gender_preference: nil means omit; an empty slice
//     pointer means "clear".
//
// The store maps Set=false / nil-pointer to "leave column untouched" and
// Set=true / non-nil-pointer to "overwrite with this value (including
// NULL when applicable)".
type PreferencesUpdate struct {
	MinAge                      Optional[int]
	MaxAge                      Optional[int]
	MaxDistanceKm               Optional[int]
	GenderPreference            *[]string
	Locale                      *string
	RequirePhoto                *bool
	DiscoveryPaused             *bool
	HideDistanceFromNonContacts *bool
	HidePresence                *bool
	HideReadReceipts            *bool
	HideTypingIndicator         *bool
	NotifyChatMessages          *bool
	NotifyContactRequests       *bool
	NotifyChannelMentions       *bool
	NotifySystem                *bool
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
	// CreatedAt is used internally for the distance-sort cursor; not serialised to the API.
	CreatedAt time.Time
	// Score is the relevance score from the browse ranking formula; not serialised to the API.
	Score float64
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
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"ca,omitzero"` // used only when sortByDistance
	DistanceKm *float64  `json:"dk,omitempty"` // distance sort
	Score      *float64  `json:"sc,omitempty"` // relevance sort (default)
	AsOf       time.Time `json:"ao,omitzero"`  // recency anchor, pinned on first page
}

// EncodeBrowseCursor encodes the last profile of a page into an opaque cursor string.
// asOf is the recency-anchor timestamp that was used for scoring on the first page;
// it is carried through all subsequent pages so the score formula stays deterministic.
func EncodeBrowseCursor(p BrowseProfile, sortByDistance bool, asOf time.Time) string {
	c := browseCursor{ID: p.ID, AsOf: asOf}
	if sortByDistance {
		c.CreatedAt = p.CreatedAt
		c.DistanceKm = p.DistanceKm
	} else {
		c.Score = &p.Score
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
	GetPrivacyFlagsByIDs(ctx context.Context, userIDs []string) (map[string]PrivacyFlags, error)
	AcceptedContactIDs(ctx context.Context, userID string) ([]string, error)
	UpsertPreferences(ctx context.Context, userID string, update PreferencesUpdate) (*ProfilePreferences, error)
	SearchInterests(ctx context.Context, query string, limit int) ([]InterestSuggestion, error)
	Browse(ctx context.Context, userID string, limit int, cursor string, sortByDistance bool, interests []string, seed string, asOf time.Time) ([]BrowseProfile, error)
}

// Service handles profile business logic.
type Service struct {
	store            Store
	storagePublicURL string // non-empty: avatar/photo URLs must start with this prefix
	// MediaApproved, when set, reports whether the image at storageKey (owned
	// by userID) has cleared moderation. UpdateAvatar and AddPhoto refuse media
	// that has not been approved, so an un-moderated or rejected image can
	// never be published to the user's public profile surfaces.
	MediaApproved func(ctx context.Context, storageKey, userID string) (bool, error)
}

// NewService creates a new profiles Service.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// SetStoragePublicURL configures the expected URL prefix for uploaded media.
// When set, UpdateAvatar and AddPhoto reject any URL that does not start with
// this prefix, preventing users from embedding arbitrary external URLs.
func (s *Service) SetStoragePublicURL(u string) { s.storagePublicURL = u }

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
	if err := validateLookingFor(in); err != nil {
		return nil, err
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

// validateLookingFor applies the public-intent validation rules used by
// UpdateMyProfile: gender values must come from the canonical set, and
// the optional age bounds must each be in [18, 120] with min ≤ max when
// both are provided.
func validateLookingFor(in ProfileInput) error {
	genderSet := make(map[string]struct{}, len(LookingForGenders))
	for _, g := range LookingForGenders {
		genderSet[g] = struct{}{}
	}
	for _, g := range in.LookingForGender {
		if _, ok := genderSet[g]; !ok {
			return fmt.Errorf("%w: unknown looking_for_gender %q", ErrInvalidInput, g)
		}
	}
	if v := in.LookingForAgeMin; v != nil && (*v < 18 || *v > 120) {
		return fmt.Errorf("%w: looking_for_age_min must be between 18 and 120", ErrInvalidInput)
	}
	if v := in.LookingForAgeMax; v != nil && (*v < 18 || *v > 120) {
		return fmt.Errorf("%w: looking_for_age_max must be between 18 and 120", ErrInvalidInput)
	}
	if mn, mx := in.LookingForAgeMin, in.LookingForAgeMax; mn != nil && mx != nil && *mn > *mx {
		return fmt.Errorf("%w: looking_for_age_min must be less than or equal to looking_for_age_max", ErrInvalidInput)
	}
	return nil
}

// SearchInterests returns interest suggestions matching the given prefix, ordered by usage.
func (s *Service) SearchInterests(ctx context.Context, query string) ([]InterestSuggestion, error) {
	return s.store.SearchInterests(ctx, query, 10)
}

// UpdateAvatar updates only the avatar URL for the given user.
func (s *Service) UpdateAvatar(ctx context.Context, userID, avatarURL string) error {
	if s.storagePublicURL != "" && !strings.HasPrefix(avatarURL, s.storagePublicURL) {
		return fmt.Errorf("%w: avatar URL must be a storage URL", ErrInvalidInput)
	}
	if err := s.requireMediaApproved(ctx, avatarURL, userID); err != nil {
		return err
	}
	return s.store.UpdateAvatar(ctx, userID, avatarURL)
}

// AddPhoto adds a showcase photo for the given user.
// Returns ErrInvalidInput if the user already has MaxProfilePhotos.
func (s *Service) AddPhoto(ctx context.Context, userID, url string) (*ProfilePhoto, error) {
	if url == "" {
		return nil, fmt.Errorf("%w: url is required", ErrInvalidInput)
	}
	if s.storagePublicURL != "" && !strings.HasPrefix(url, s.storagePublicURL) {
		return nil, fmt.Errorf("%w: photo URL must be a storage URL", ErrInvalidInput)
	}
	if err := s.requireMediaApproved(ctx, url, userID); err != nil {
		return nil, err
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

// requireMediaApproved enforces the attach-time moderation gate for a public
// media URL. It derives the storage key by stripping the configured public-URL
// prefix and asks the MediaApproved hook whether the upload has cleared
// moderation. A no-op when the hook or prefix is unset (dev / tests).
func (s *Service) requireMediaApproved(ctx context.Context, mediaURL, userID string) error {
	if s.MediaApproved == nil || s.storagePublicURL == "" {
		return nil
	}
	storageKey := strings.TrimPrefix(strings.TrimPrefix(mediaURL, s.storagePublicURL), "/")
	approved, err := s.MediaApproved(ctx, storageKey, userID)
	if err != nil {
		return err
	}
	if !approved {
		return ErrMediaNotApproved
	}
	return nil
}

// DeletePhoto removes a showcase photo, verifying ownership.
func (s *Service) DeletePhoto(ctx context.Context, userID, photoID string) error {
	return s.store.DeletePhoto(ctx, photoID, userID)
}

// GetMyPreferences returns the discovery preferences for the given user.
func (s *Service) GetMyPreferences(ctx context.Context, userID string) (*ProfilePreferences, error) {
	return s.store.GetPreferences(ctx, userID)
}

// GetPrivacyFlags returns the privacy toggles for a single user. Used by
// chat and presence to gate WS frame delivery and presence visibility on
// the user's stored preferences.
func (s *Service) GetPrivacyFlags(ctx context.Context, userID string) (PrivacyFlags, error) {
	prefs, err := s.store.GetPreferences(ctx, userID)
	if err != nil {
		return PrivacyFlags{}, err
	}
	return PrivacyFlags{
		HideDistanceFromNonContacts: prefs.HideDistanceFromNonContacts,
		HidePresence:                prefs.HidePresence,
		HideReadReceipts:            prefs.HideReadReceipts,
		HideTypingIndicator:         prefs.HideTypingIndicator,
	}, nil
}

// GetNotificationFlags returns the per-category notification toggles for a
// user. Used by the push gate at delivery time. All fields default true.
func (s *Service) GetNotificationFlags(ctx context.Context, userID string) (NotificationFlags, error) {
	prefs, err := s.store.GetPreferences(ctx, userID)
	if err != nil {
		return NotificationFlags{}, err
	}
	return NotificationFlags{
		ChatMessages:    prefs.NotifyChatMessages,
		ContactRequests: prefs.NotifyContactRequests,
		ChannelMentions: prefs.NotifyChannelMentions,
		System:          prefs.NotifySystem,
	}, nil
}

// GetPrivacyFlagsByIDs returns the privacy toggles for the given user IDs.
// Users with no preferences row are absent from the map; callers must treat
// absence as the zero PrivacyFlags (all false).
func (s *Service) GetPrivacyFlagsByIDs(ctx context.Context, userIDs []string) (map[string]PrivacyFlags, error) {
	return s.store.GetPrivacyFlagsByIDs(ctx, userIDs)
}

// UpdateMyPreferences validates the present fields of update and applies
// them via partial upsert. Fields the caller did not set preserve the
// existing DB value (or fall back to the SQL column default when the row
// is new). Explicit JSON null on a nullable filter int clears the column.
func (s *Service) UpdateMyPreferences(ctx context.Context, userID string, update PreferencesUpdate) (*ProfilePreferences, error) {
	if v := update.MinAge.Value; v != nil && *v < 18 {
		return nil, fmt.Errorf("%w: min_age must be at least 18", ErrInvalidInput)
	}
	if v := update.MaxAge.Value; v != nil && *v > 120 {
		return nil, fmt.Errorf("%w: max_age must be at most 120", ErrInvalidInput)
	}
	if min, max := update.MinAge.Value, update.MaxAge.Value; min != nil && max != nil && *min > *max {
		return nil, fmt.Errorf("%w: min_age must be less than or equal to max_age", ErrInvalidInput)
	}
	if v := update.MaxDistanceKm.Value; v != nil && *v <= 0 {
		return nil, fmt.Errorf("%w: max_distance_km must be positive", ErrInvalidInput)
	}
	return s.store.UpsertPreferences(ctx, userID, update)
}

// Browse returns a cursor-paginated list of profiles visible to the given user,
// filtered by their stored discovery preferences. cursor is an opaque token
// returned by a previous call; pass "" to start from the first page.
//
// Distance is suppressed (DistanceKm = nil) for any browsed profile whose
// owner has set hide_distance_from_non_contacts and is not an accepted
// contact of the caller.
func (s *Service) Browse(ctx context.Context, userID string, limit int, cursor string, sortByDistance bool, interests []string, seed string) (*BrowsePage, error) {
	if seed == "" {
		seed = userID
	}
	// Pin the recency-scoring anchor to the first page so subsequent pages score
	// consistently. A fresh first page always anchors to now.
	var asOf time.Time
	if cursor == "" {
		asOf = time.Now()
	} else if cur, err := DecodeBrowseCursor(cursor); err == nil && !cur.AsOf.IsZero() {
		asOf = cur.AsOf
	} else {
		asOf = time.Now()
	}
	profiles, err := s.store.Browse(ctx, userID, limit+1, cursor, sortByDistance, interests, seed, asOf)
	if err != nil {
		return nil, err
	}
	var nextCursor string
	if len(profiles) > limit {
		profiles = profiles[:limit]
		nextCursor = EncodeBrowseCursor(profiles[limit-1], sortByDistance, asOf)
	}

	if len(profiles) > 0 {
		ids := make([]string, len(profiles))
		for i, p := range profiles {
			ids[i] = p.UserID
		}
		flags, err := s.store.GetPrivacyFlagsByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		contactIDs, err := s.store.AcceptedContactIDs(ctx, userID)
		if err != nil {
			return nil, err
		}
		isContact := make(map[string]struct{}, len(contactIDs))
		for _, id := range contactIDs {
			isContact[id] = struct{}{}
		}
		for i := range profiles {
			if !flags[profiles[i].UserID].HideDistanceFromNonContacts {
				continue
			}
			if _, ok := isContact[profiles[i].UserID]; ok {
				continue
			}
			profiles[i].DistanceKm = nil
		}
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
