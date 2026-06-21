package profiles

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockStore struct {
	profile           *Profile
	photos            []ProfilePhoto
	browseFn          func(seed string)
	photo             *ProfilePhoto
	photoCount        int
	prefs             *ProfilePreferences
	interests         []InterestSuggestion
	browseProfiles    []BrowseProfile
	privacyFlags      map[string]PrivacyFlags
	contactIDs        []string
	lastUpsertUpdate  *PreferencesUpdate
	browseErr         error
	usernameAvailable bool
	getErr            error
	upsertErr         error
	syncErr           error
	photosErr         error
	countErr          error
	addErr            error
	deleteErr         error
	reorderErr        error
	prefsErr          error
	upsertPrefsErr    error
	searchIntErr      error
	usernameAvailErr  error
	privacyFlagsErr   error
	contactIDsErr     error
}

func (m *mockStore) GetByUserID(_ context.Context, _ string) (*Profile, error) {
	return m.profile, m.getErr
}

func (m *mockStore) GetByUsername(_ context.Context, _ string) (*Profile, error) {
	return m.profile, m.getErr
}

func (m *mockStore) IsUsernameAvailable(_ context.Context, _ string) (bool, error) {
	return m.usernameAvailable, m.usernameAvailErr
}

func (m *mockStore) Upsert(_ context.Context, userID string, in ProfileInput) (*Profile, error) {
	if m.upsertErr != nil {
		return nil, m.upsertErr
	}
	return &Profile{
		ID: "prof-1", UserID: userID, Username: in.Username, DisplayName: in.DisplayName, Bio: in.Bio, AvatarURL: in.AvatarURL,
		DateOfBirth: in.DateOfBirth, Gender: in.Gender, LocationText: in.LocationText,
		Latitude: in.Latitude, Longitude: in.Longitude, Interests: []string{},
	}, nil
}

func (m *mockStore) SyncInterests(_ context.Context, _ string, _ []string) error {
	return m.syncErr
}

func (m *mockStore) GetPhotosByUserID(_ context.Context, _ string) ([]ProfilePhoto, error) {
	return m.photos, m.photosErr
}

func (m *mockStore) CountPhotos(_ context.Context, _ string) (int, error) {
	return m.photoCount, m.countErr
}

func (m *mockStore) AddPhoto(_ context.Context, _, _ string) (*ProfilePhoto, error) {
	return m.photo, m.addErr
}

func (m *mockStore) DeletePhoto(_ context.Context, _, _ string) error {
	return m.deleteErr
}

func (m *mockStore) ReorderPhotos(_ context.Context, _ string, _ []string) error {
	return m.reorderErr
}

func (m *mockStore) UpdateAvatar(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockStore) GetPreferences(_ context.Context, _ string) (*ProfilePreferences, error) {
	if m.prefsErr != nil {
		return nil, m.prefsErr
	}
	if m.prefs != nil {
		return m.prefs, nil
	}
	return &ProfilePreferences{GenderPreference: []string{}}, nil
}

func (m *mockStore) UpsertPreferences(_ context.Context, _ string, update PreferencesUpdate) (*ProfilePreferences, error) {
	if m.upsertPrefsErr != nil {
		return nil, m.upsertPrefsErr
	}
	m.lastUpsertUpdate = &update
	// Build a result reflecting only the fields the caller set, so tests
	// that assert on the returned struct see the partial-update semantics
	// surfaced by the production store.
	out := ProfilePreferences{GenderPreference: []string{}}
	if update.MinAge.Set {
		out.MinAge = update.MinAge.Value
	}
	if update.MaxAge.Set {
		out.MaxAge = update.MaxAge.Value
	}
	if update.MaxDistanceKm.Set {
		out.MaxDistanceKm = update.MaxDistanceKm.Value
	}
	if update.GenderPreference != nil {
		out.GenderPreference = *update.GenderPreference
	}
	if update.Locale != nil {
		out.Locale = *update.Locale
	}
	if update.RequirePhoto != nil {
		out.RequirePhoto = *update.RequirePhoto
	}
	if update.DiscoveryPaused != nil {
		out.DiscoveryPaused = *update.DiscoveryPaused
	}
	if update.HideDistanceFromNonContacts != nil {
		out.HideDistanceFromNonContacts = *update.HideDistanceFromNonContacts
	}
	if update.HidePresence != nil {
		out.HidePresence = *update.HidePresence
	}
	if update.HideReadReceipts != nil {
		out.HideReadReceipts = *update.HideReadReceipts
	}
	if update.HideTypingIndicator != nil {
		out.HideTypingIndicator = *update.HideTypingIndicator
	}
	if update.NotifyChatMessages != nil {
		out.NotifyChatMessages = *update.NotifyChatMessages
	}
	if update.NotifyContactRequests != nil {
		out.NotifyContactRequests = *update.NotifyContactRequests
	}
	if update.NotifyChannelMentions != nil {
		out.NotifyChannelMentions = *update.NotifyChannelMentions
	}
	if update.NotifySystem != nil {
		out.NotifySystem = *update.NotifySystem
	}
	return &out, nil
}

func (m *mockStore) SearchInterests(_ context.Context, _ string, _ int) ([]InterestSuggestion, error) {
	if m.searchIntErr != nil {
		return nil, m.searchIntErr
	}
	if m.interests != nil {
		return m.interests, nil
	}
	return []InterestSuggestion{}, nil
}

func (m *mockStore) Browse(_ context.Context, _ string, _ int, _ string, _ bool, _ []string, seed string, _ time.Time) ([]BrowseProfile, error) {
	if m.browseFn != nil {
		m.browseFn(seed)
	}
	if m.browseErr != nil {
		return nil, m.browseErr
	}
	if m.browseProfiles != nil {
		return m.browseProfiles, nil
	}
	return []BrowseProfile{}, nil
}

func (m *mockStore) GetPrivacyFlagsByIDs(_ context.Context, _ []string) (map[string]PrivacyFlags, error) {
	if m.privacyFlagsErr != nil {
		return nil, m.privacyFlagsErr
	}
	if m.privacyFlags != nil {
		return m.privacyFlags, nil
	}
	return map[string]PrivacyFlags{}, nil
}

func (m *mockStore) AcceptedContactIDs(_ context.Context, _ string) ([]string, error) {
	if m.contactIDsErr != nil {
		return nil, m.contactIDsErr
	}
	return m.contactIDs, nil
}

// validDOB returns a DOB 25 years in the past (always valid for 18+ check).
func validDOB() *time.Time {
	t := time.Now().AddDate(-25, 0, 0)
	return &t
}

// --- GetMyProfile ---

func TestGetMyProfile_ExistingProfile(t *testing.T) {
	svc := NewService(&mockStore{
		profile: &Profile{ID: "prof-1", UserID: "user-1", DisplayName: "Alice", Bio: "Hi"},
	})

	p, err := svc.GetMyProfile(t.Context(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.DisplayName != "Alice" {
		t.Errorf("DisplayName = %q, want %q", p.DisplayName, "Alice")
	}
}

func TestGetMyProfile_CreatesWhenNotFound(t *testing.T) {
	svc := NewService(&mockStore{getErr: ErrNotFound})

	p, err := svc.GetMyProfile(t.Context(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", p.UserID, "user-1")
	}
}

func TestGetMyProfile_StoreError(t *testing.T) {
	svc := NewService(&mockStore{getErr: errors.New("db error")})

	_, err := svc.GetMyProfile(t.Context(), "user-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetMyProfile_UpsertError(t *testing.T) {
	svc := NewService(&mockStore{getErr: ErrNotFound, upsertErr: errors.New("db error")})

	_, err := svc.GetMyProfile(t.Context(), "user-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetMyProfile_PhotosError(t *testing.T) {
	svc := NewService(&mockStore{
		profile:   &Profile{ID: "prof-1", UserID: "user-1", DisplayName: "Alice"},
		photosErr: errors.New("db error"),
	})

	_, err := svc.GetMyProfile(t.Context(), "user-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetMyProfile_IncludesPhotos(t *testing.T) {
	svc := NewService(&mockStore{
		profile: &Profile{ID: "prof-1", UserID: "user-1", DisplayName: "Alice"},
		photos:  []ProfilePhoto{{ID: "ph-1", URL: "https://example.com/1.jpg"}},
	})

	p, err := svc.GetMyProfile(t.Context(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Photos) != 1 {
		t.Errorf("Photos len = %d, want 1", len(p.Photos))
	}
}

// --- UpdateMyProfile ---

func TestUpdateMyProfile_Success(t *testing.T) {
	svc := NewService(&mockStore{})

	p, err := svc.UpdateMyProfile(t.Context(), "user-1", ProfileInput{DisplayName: "Alice", Bio: "Bio here", DateOfBirth: validDOB()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.DisplayName != "Alice" {
		t.Errorf("DisplayName = %q, want %q", p.DisplayName, "Alice")
	}
}

func TestUpdateMyProfile_WithNewFields(t *testing.T) {
	lat := 48.8566
	lon := 2.3522
	svc := NewService(&mockStore{})

	p, err := svc.UpdateMyProfile(t.Context(), "user-1", ProfileInput{
		Username:     "alice42",
		DisplayName:  "Alice",
		DateOfBirth:  validDOB(),
		Gender:       "female",
		LocationText: "Paris",
		Latitude:     &lat,
		Longitude:    &lon,
		Interests:    []string{"hiking", "music"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Username != "alice42" {
		t.Errorf("Username = %q, want alice42", p.Username)
	}
	if p.Gender != "female" {
		t.Errorf("Gender = %q, want female", p.Gender)
	}
	if len(p.Interests) != 2 {
		t.Errorf("Interests len = %d, want 2", len(p.Interests))
	}
}

func TestUpdateMyProfile_EmptyDisplayNameAllowed(t *testing.T) {
	// display_name is optional on the service layer; the store preserves the
	// existing value when an empty string is sent (COALESCE logic).
	svc := NewService(&mockStore{})

	_, err := svc.UpdateMyProfile(t.Context(), "user-1", ProfileInput{Bio: "Bio", DateOfBirth: validDOB()})
	if err != nil {
		t.Fatalf("expected no error for empty display name, got %v", err)
	}
}

func TestUpdateMyProfile_MissingDOB(t *testing.T) {
	svc := NewService(&mockStore{})

	_, err := svc.UpdateMyProfile(t.Context(), "user-1", ProfileInput{DisplayName: "Alice"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestUpdateMyProfile_TooYoung(t *testing.T) {
	dob := time.Now().AddDate(-17, 0, 0)
	svc := NewService(&mockStore{})

	_, err := svc.UpdateMyProfile(t.Context(), "user-1", ProfileInput{DisplayName: "Alice", DateOfBirth: &dob})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestUpdateMyProfile_InvalidUsername(t *testing.T) {
	svc := NewService(&mockStore{})

	for _, bad := range []string{"AB", "hello world", "toolongusernamethatexceedsthirtycharacters", "UPPER"} {
		_, err := svc.UpdateMyProfile(t.Context(), "user-1", ProfileInput{
			Username: bad, DisplayName: "Alice", DateOfBirth: validDOB(),
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("username %q: got %v, want ErrInvalidInput", bad, err)
		}
	}
}

func TestUpdateMyProfile_UsernameTaken(t *testing.T) {
	svc := NewService(&mockStore{upsertErr: ErrUsernameTaken})

	_, err := svc.UpdateMyProfile(t.Context(), "user-1", ProfileInput{
		Username: "alice", DisplayName: "Alice", DateOfBirth: validDOB(),
	})
	if !errors.Is(err, ErrUsernameTaken) {
		t.Errorf("got %v, want ErrUsernameTaken", err)
	}
}

func TestUpdateMyProfile_TooManyInterests(t *testing.T) {
	interests := make([]string, MaxInterests+1)
	for i := range interests {
		interests[i] = "tag"
	}
	svc := NewService(&mockStore{})

	_, err := svc.UpdateMyProfile(t.Context(), "user-1", ProfileInput{DisplayName: "Alice", DateOfBirth: validDOB(), Interests: interests})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

// --- LookingFor validation ---

func TestUpdateMyProfile_LookingForUnknownGender(t *testing.T) {
	svc := NewService(&mockStore{})
	_, err := svc.UpdateMyProfile(t.Context(), "user-1", ProfileInput{
		DisplayName: "Alice", DateOfBirth: validDOB(),
		LookingForGender: []string{"Robot"},
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestUpdateMyProfile_LookingForAgeOutOfRange(t *testing.T) {
	svc := NewService(&mockStore{})
	for _, c := range []struct {
		name     string
		min, max *int
	}{
		{"min below 18", ptrInt(17), nil},
		{"max above 120", nil, ptrInt(150)},
		{"min greater than max", ptrInt(40), ptrInt(30)},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.UpdateMyProfile(t.Context(), "user-1", ProfileInput{
				DisplayName: "Alice", DateOfBirth: validDOB(),
				LookingForAgeMin: c.min, LookingForAgeMax: c.max,
			})
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("got %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestUpdateMyProfile_LookingForAcceptsCanonicalValues(t *testing.T) {
	svc := NewService(&mockStore{})
	min, max := 25, 35
	_, err := svc.UpdateMyProfile(t.Context(), "user-1", ProfileInput{
		DisplayName:      "Alice",
		DateOfBirth:      validDOB(),
		LookingForGender: []string{"Female", "Non-binary"},
		LookingForAgeMin: &min,
		LookingForAgeMax: &max,
	})
	if err != nil {
		t.Errorf("expected no error for canonical values, got %v", err)
	}
}

func ptrInt(v int) *int { return &v }

func TestUpdateMyProfile_StoreError(t *testing.T) {
	svc := NewService(&mockStore{upsertErr: errors.New("db error")})

	_, err := svc.UpdateMyProfile(t.Context(), "user-1", ProfileInput{DisplayName: "Alice", DateOfBirth: validDOB()})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUpdateMyProfile_SyncInterestsError(t *testing.T) {
	svc := NewService(&mockStore{syncErr: errors.New("db error")})

	_, err := svc.UpdateMyProfile(t.Context(), "user-1", ProfileInput{DisplayName: "Alice", DateOfBirth: validDOB(), Interests: []string{"hiking"}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- IsUsernameAvailable ---

func TestIsUsernameAvailable_Available(t *testing.T) {
	svc := NewService(&mockStore{usernameAvailable: true})

	ok, err := svc.IsUsernameAvailable(t.Context(), "alice42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected available=true")
	}
}

func TestIsUsernameAvailable_Taken(t *testing.T) {
	svc := NewService(&mockStore{usernameAvailable: false})

	ok, err := svc.IsUsernameAvailable(t.Context(), "alice42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected available=false")
	}
}

func TestIsUsernameAvailable_InvalidFormat(t *testing.T) {
	svc := NewService(&mockStore{usernameAvailable: true})

	// Invalid format → false without hitting store
	ok, err := svc.IsUsernameAvailable(t.Context(), "AB bad!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected available=false for invalid format")
	}
}

func TestIsUsernameAvailable_StoreError(t *testing.T) {
	svc := NewService(&mockStore{usernameAvailErr: errors.New("db error")})

	_, err := svc.IsUsernameAvailable(t.Context(), "alice42")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- GetPublicProfileByUsername ---

func TestGetPublicProfileByUsername_Success(t *testing.T) {
	svc := NewService(&mockStore{
		profile: &Profile{ID: "prof-1", UserID: "user-2", Username: "bob", DisplayName: "Bob"},
		photos:  []ProfilePhoto{{ID: "ph-1", URL: "https://example.com/1.jpg"}},
	})

	p, err := svc.GetPublicProfileByUsername(t.Context(), "bob")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Username != "bob" {
		t.Errorf("Username = %q, want bob", p.Username)
	}
	if len(p.Photos) != 1 {
		t.Errorf("Photos len = %d, want 1", len(p.Photos))
	}
}

func TestGetPublicProfileByUsername_NotFound(t *testing.T) {
	svc := NewService(&mockStore{getErr: ErrNotFound})

	_, err := svc.GetPublicProfileByUsername(t.Context(), "nobody")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

// --- SearchInterests ---

func TestSearchInterests_Success(t *testing.T) {
	svc := NewService(&mockStore{
		interests: []InterestSuggestion{{Name: "hiking", Count: 5}, {Name: "history", Count: 2}},
	})

	results, err := svc.SearchInterests(t.Context(), "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("len = %d, want 2", len(results))
	}
	if results[0].Name != "hiking" {
		t.Errorf("Name = %q, want hiking", results[0].Name)
	}
}

func TestSearchInterests_Empty(t *testing.T) {
	svc := NewService(&mockStore{})

	results, err := svc.SearchInterests(t.Context(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results == nil {
		t.Error("expected non-nil slice")
	}
}

func TestSearchInterests_StoreError(t *testing.T) {
	svc := NewService(&mockStore{searchIntErr: errors.New("db error")})

	_, err := svc.SearchInterests(t.Context(), "hi")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- GetPublicProfile ---

func TestGetPublicProfile_Success(t *testing.T) {
	svc := NewService(&mockStore{
		profile: &Profile{ID: "prof-1", UserID: "user-2", DisplayName: "Bob"},
		photos:  []ProfilePhoto{{ID: "ph-1", URL: "https://example.com/1.jpg"}},
	})

	p, err := svc.GetPublicProfile(t.Context(), "user-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.DisplayName != "Bob" {
		t.Errorf("DisplayName = %q, want Bob", p.DisplayName)
	}
	if len(p.Photos) != 1 {
		t.Errorf("Photos len = %d, want 1", len(p.Photos))
	}
}

func TestGetPublicProfile_NotFound(t *testing.T) {
	svc := NewService(&mockStore{getErr: ErrNotFound})

	_, err := svc.GetPublicProfile(t.Context(), "user-2")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestGetPublicProfile_StoreError(t *testing.T) {
	svc := NewService(&mockStore{getErr: errors.New("db error")})

	_, err := svc.GetPublicProfile(t.Context(), "user-2")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- AddPhoto ---

func TestAddPhoto_Success(t *testing.T) {
	svc := NewService(&mockStore{
		photoCount: 2,
		photo:      &ProfilePhoto{ID: "ph-1", URL: "https://example.com/1.jpg"},
	})

	ph, err := svc.AddPhoto(t.Context(), "user-1", "https://example.com/1.jpg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ph.ID != "ph-1" {
		t.Errorf("ID = %q, want ph-1", ph.ID)
	}
}

func TestAddPhoto_MaxReached(t *testing.T) {
	svc := NewService(&mockStore{photoCount: MaxProfilePhotos})

	_, err := svc.AddPhoto(t.Context(), "user-1", "https://example.com/1.jpg")
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestAddPhoto_EmptyURL(t *testing.T) {
	svc := NewService(&mockStore{})

	_, err := svc.AddPhoto(t.Context(), "user-1", "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestAddPhoto_CountError(t *testing.T) {
	svc := NewService(&mockStore{countErr: errors.New("db error")})

	_, err := svc.AddPhoto(t.Context(), "user-1", "https://example.com/1.jpg")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAddPhoto_StoreError(t *testing.T) {
	svc := NewService(&mockStore{addErr: errors.New("db error")})

	_, err := svc.AddPhoto(t.Context(), "user-1", "https://example.com/1.jpg")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAddPhoto_StorageURLValidation(t *testing.T) {
	svc := NewService(&mockStore{photo: &ProfilePhoto{ID: "ph-1", URL: "https://cdn.example.com/photo.jpg"}})
	svc.SetStoragePublicURL("https://cdn.example.com/")

	t.Run("allows storage URL", func(t *testing.T) {
		_, err := svc.AddPhoto(t.Context(), "user-1", "https://cdn.example.com/photo.jpg")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("rejects external URL", func(t *testing.T) {
		_, err := svc.AddPhoto(t.Context(), "user-1", "https://evil.example.com/bad.jpg")
		if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("got %v, want ErrInvalidInput", err)
		}
	})
}

func TestUpdateAvatar_StorageURLValidation(t *testing.T) {
	svc := NewService(&mockStore{})
	svc.SetStoragePublicURL("https://cdn.example.com/")

	t.Run("allows storage URL", func(t *testing.T) {
		if err := svc.UpdateAvatar(t.Context(), "user-1", "https://cdn.example.com/avatar.jpg"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("rejects external URL", func(t *testing.T) {
		err := svc.UpdateAvatar(t.Context(), "user-1", "https://tracking.evil.com/pixel.gif")
		if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("got %v, want ErrInvalidInput", err)
		}
	})

	t.Run("no restriction when storagePublicURL unset", func(t *testing.T) {
		svc2 := NewService(&mockStore{})
		if err := svc2.UpdateAvatar(t.Context(), "user-1", "https://anything.example.com/img.jpg"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestUpdateAvatar_ModerationGate(t *testing.T) {
	svc := NewService(&mockStore{})
	svc.SetStoragePublicURL("https://cdn.example.com/")

	t.Run("rejects unapproved media", func(t *testing.T) {
		svc.MediaApproved = func(_ context.Context, _, _ string) (bool, error) { return false, nil }
		err := svc.UpdateAvatar(t.Context(), "user-1", "https://cdn.example.com/avatars/x.jpg")
		if !errors.Is(err, ErrMediaNotApproved) {
			t.Errorf("got %v, want ErrMediaNotApproved", err)
		}
	})

	t.Run("derives storage key without the public prefix", func(t *testing.T) {
		var gotKey string
		svc.MediaApproved = func(_ context.Context, storageKey, _ string) (bool, error) {
			gotKey = storageKey
			return true, nil
		}
		if err := svc.UpdateAvatar(t.Context(), "user-1", "https://cdn.example.com/avatars/x.jpg"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotKey != "avatars/x.jpg" {
			t.Errorf("storage key = %q, want %q", gotKey, "avatars/x.jpg")
		}
	})
}

// --- DeletePhoto ---

func TestDeletePhoto_Success(t *testing.T) {
	svc := NewService(&mockStore{})

	if err := svc.DeletePhoto(t.Context(), "user-1", "ph-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeletePhoto_NotFound(t *testing.T) {
	svc := NewService(&mockStore{deleteErr: ErrPhotoNotFound})

	err := svc.DeletePhoto(t.Context(), "user-1", "ph-1")
	if !errors.Is(err, ErrPhotoNotFound) {
		t.Errorf("got %v, want ErrPhotoNotFound", err)
	}
}

func TestDeletePhoto_StoreError(t *testing.T) {
	svc := NewService(&mockStore{deleteErr: errors.New("db error")})

	err := svc.DeletePhoto(t.Context(), "user-1", "ph-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- ReorderPhotos ---

func TestReorderPhotos_Success(t *testing.T) {
	svc := NewService(&mockStore{})

	if err := svc.ReorderPhotos(t.Context(), "user-1", []string{"ph-1", "ph-2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReorderPhotos_InvalidInput(t *testing.T) {
	svc := NewService(&mockStore{reorderErr: ErrInvalidInput})

	err := svc.ReorderPhotos(t.Context(), "user-1", []string{"ph-1"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestReorderPhotos_NotFound(t *testing.T) {
	svc := NewService(&mockStore{reorderErr: ErrPhotoNotFound})

	err := svc.ReorderPhotos(t.Context(), "user-1", []string{"ph-unknown"})
	if !errors.Is(err, ErrPhotoNotFound) {
		t.Errorf("got %v, want ErrPhotoNotFound", err)
	}
}

func TestReorderPhotos_StoreError(t *testing.T) {
	svc := NewService(&mockStore{reorderErr: errors.New("db error")})

	err := svc.ReorderPhotos(t.Context(), "user-1", []string{"ph-1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- GetMyPreferences ---

func TestGetMyPreferences_Success(t *testing.T) {
	minAge := 25
	svc := NewService(&mockStore{prefs: &ProfilePreferences{
		UserID: "user-1", MinAge: &minAge, GenderPreference: []string{"female"},
	}})

	p, err := svc.GetMyPreferences(t.Context(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.MinAge == nil || *p.MinAge != 25 {
		t.Errorf("MinAge = %v, want 25", p.MinAge)
	}
}

func TestGetMyPreferences_Error(t *testing.T) {
	svc := NewService(&mockStore{prefsErr: errors.New("db error")})

	_, err := svc.GetMyPreferences(t.Context(), "user-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- UpdateMyPreferences ---

// optInt is a small constructor helper for present-non-null Optional[int]
// so test bodies don't need to take the address of locals each time.
func optInt(v int) Optional[int] {
	return Optional[int]{Set: true, Value: &v}
}

// optIntNull is the explicit-JSON-null variant ("clear this column").
func optIntNull() Optional[int] {
	return Optional[int]{Set: true, Value: nil}
}

func TestUpdateMyPreferences_Success(t *testing.T) {
	gp := []string{"any"}
	svc := NewService(&mockStore{})

	p, err := svc.UpdateMyPreferences(t.Context(), "user-1", PreferencesUpdate{
		MinAge: optInt(20), MaxAge: optInt(35), MaxDistanceKm: optInt(50), GenderPreference: &gp,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.MaxAge == nil || *p.MaxAge != 35 {
		t.Errorf("MaxAge = %v, want 35", p.MaxAge)
	}
}

func TestUpdateMyPreferences_MinAgeTooLow(t *testing.T) {
	svc := NewService(&mockStore{})

	_, err := svc.UpdateMyPreferences(t.Context(), "user-1", PreferencesUpdate{MinAge: optInt(16)})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestUpdateMyPreferences_MaxAgeTooHigh(t *testing.T) {
	svc := NewService(&mockStore{})

	_, err := svc.UpdateMyPreferences(t.Context(), "user-1", PreferencesUpdate{MaxAge: optInt(200)})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestUpdateMyPreferences_MinGreaterThanMax(t *testing.T) {
	svc := NewService(&mockStore{})

	_, err := svc.UpdateMyPreferences(t.Context(), "user-1", PreferencesUpdate{MinAge: optInt(40), MaxAge: optInt(30)})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestUpdateMyPreferences_InvalidDistance(t *testing.T) {
	svc := NewService(&mockStore{})

	_, err := svc.UpdateMyPreferences(t.Context(), "user-1", PreferencesUpdate{MaxDistanceKm: optInt(0)})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestUpdateMyPreferences_StoreError(t *testing.T) {
	svc := NewService(&mockStore{upsertPrefsErr: errors.New("db error")})

	_, err := svc.UpdateMyPreferences(t.Context(), "user-1", PreferencesUpdate{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- Partial update semantics ---

// A locale-only PUT must not appear in the upsert as anything but a single
// `locale` column write. The bug we're fixing is that the previous code
// treated absent fields as zero values and clobbered them.
func TestUpdateMyPreferences_LocaleOnlyDoesNotClobber(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store)

	loc := "en"
	if _, err := svc.UpdateMyPreferences(t.Context(), "user-1", PreferencesUpdate{Locale: &loc}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := store.lastUpsertUpdate
	if got == nil {
		t.Fatal("expected store to receive an update; got nil")
	}
	if got.Locale == nil || *got.Locale != "en" {
		t.Errorf("Locale = %v, want \"en\"", got.Locale)
	}
	if got.MinAge.Set || got.MaxAge.Set || got.MaxDistanceKm.Set {
		t.Errorf("filter ints should be unset on locale-only PUT, got %+v", got)
	}
	if got.GenderPreference != nil {
		t.Errorf("GenderPreference should be nil on locale-only PUT, got %+v", got.GenderPreference)
	}
	if got.HidePresence != nil || got.HideReadReceipts != nil || got.HideTypingIndicator != nil ||
		got.HideDistanceFromNonContacts != nil {
		t.Errorf("privacy flags should be nil on locale-only PUT, got %+v", got)
	}
	if got.NotifyChatMessages != nil || got.NotifyContactRequests != nil ||
		got.NotifyChannelMentions != nil || got.NotifySystem != nil {
		t.Errorf("notification flags should be nil on locale-only PUT, got %+v", got)
	}
}

// A toggle-only PUT for one privacy flag must not touch any other column.
func TestUpdateMyPreferences_PrivacyToggleOnlyIsolated(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store)

	hide := true
	if _, err := svc.UpdateMyPreferences(t.Context(), "user-1", PreferencesUpdate{HidePresence: &hide}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := store.lastUpsertUpdate
	if got == nil || got.HidePresence == nil || !*got.HidePresence {
		t.Fatalf("HidePresence not forwarded: %+v", got)
	}
	if got.Locale != nil || got.GenderPreference != nil ||
		got.MinAge.Set || got.MaxAge.Set || got.MaxDistanceKm.Set ||
		got.NotifyChatMessages != nil || got.NotifyContactRequests != nil {
		t.Errorf("unrelated fields should be unset on a privacy-toggle-only PUT, got %+v", got)
	}
}

// require_photo follows the same partial-update path as the privacy
// toggles — a PUT that only sets it must not touch any other column.
func TestUpdateMyPreferences_RequirePhotoToggleIsolated(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store)

	rp := true
	if _, err := svc.UpdateMyPreferences(t.Context(), "user-1", PreferencesUpdate{RequirePhoto: &rp}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := store.lastUpsertUpdate
	if got == nil || got.RequirePhoto == nil || !*got.RequirePhoto {
		t.Fatalf("RequirePhoto not forwarded: %+v", got)
	}
	if got.Locale != nil || got.GenderPreference != nil ||
		got.MinAge.Set || got.MaxAge.Set || got.MaxDistanceKm.Set ||
		got.HidePresence != nil || got.NotifyChatMessages != nil {
		t.Errorf("unrelated fields should be unset on a require-photo-only PUT, got %+v", got)
	}
}

// Browse "Clear filters" sends explicit JSON nulls for the three nullable
// filter ints. Optional[int] preserves the distinction between "omitted"
// and "explicit null" so the store can clear the column on a null.
func TestUpdateMyPreferences_ExplicitNullClearsFilterInts(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store)

	gp := []string{}
	_, err := svc.UpdateMyPreferences(t.Context(), "user-1", PreferencesUpdate{
		MinAge:           optIntNull(),
		MaxAge:           optIntNull(),
		MaxDistanceKm:    optIntNull(),
		GenderPreference: &gp,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := store.lastUpsertUpdate
	if got == nil {
		t.Fatal("expected store to receive an update; got nil")
	}
	if !got.MinAge.Set || got.MinAge.Value != nil {
		t.Errorf("MinAge should be Set with nil Value (explicit null), got %+v", got.MinAge)
	}
	if !got.MaxAge.Set || got.MaxAge.Value != nil {
		t.Errorf("MaxAge should be Set with nil Value, got %+v", got.MaxAge)
	}
	if !got.MaxDistanceKm.Set || got.MaxDistanceKm.Value != nil {
		t.Errorf("MaxDistanceKm should be Set with nil Value, got %+v", got.MaxDistanceKm)
	}
	if got.GenderPreference == nil || len(*got.GenderPreference) != 0 {
		t.Errorf("GenderPreference should be a non-nil empty slice, got %+v", got.GenderPreference)
	}
}

// --- Browse ---

func TestBrowse_ReturnsProfiles(t *testing.T) {
	dob := time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC)
	svc := NewService(&mockStore{
		browseProfiles: []BrowseProfile{
			{ID: "p1", UserID: "u1", Username: "alice", DisplayName: "Alice", DateOfBirth: &dob},
			{ID: "p2", UserID: "u2", Username: "bob", DisplayName: "Bob", DateOfBirth: &dob},
		},
	})

	page, err := svc.Browse(t.Context(), "requester", 10, "", false, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Profiles) != 2 {
		t.Errorf("len = %d, want 2", len(page.Profiles))
	}
	if page.NextCursor != "" {
		t.Error("NextCursor should be empty when no more pages")
	}
	if page.Profiles[0].Age == nil {
		t.Error("Age should be computed from DateOfBirth")
	}
}

func TestBrowse_HasMore(t *testing.T) {
	dob := time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC)
	// Store returns limit+1 rows to signal more pages
	svc := NewService(&mockStore{
		browseProfiles: []BrowseProfile{
			{ID: "p1", UserID: "u1", DateOfBirth: &dob},
			{ID: "p2", UserID: "u2", DateOfBirth: &dob},
			{ID: "p3", UserID: "u3", DateOfBirth: &dob},
		},
	})

	page, err := svc.Browse(t.Context(), "requester", 2, "", false, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Profiles) != 2 {
		t.Errorf("len = %d, want 2 (extra trimmed)", len(page.Profiles))
	}
	if page.NextCursor == "" {
		t.Error("NextCursor should be set when there are more pages")
	}
}

func TestBrowse_StoreError(t *testing.T) {
	svc := NewService(&mockStore{browseErr: errors.New("db error")})

	_, err := svc.Browse(t.Context(), "requester", 20, "", false, nil, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestBrowse_NilDOBSkipsAge(t *testing.T) {
	svc := NewService(&mockStore{
		browseProfiles: []BrowseProfile{
			{ID: "p1", UserID: "u1", DateOfBirth: nil},
		},
	})

	page, err := svc.Browse(t.Context(), "requester", 10, "", false, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Profiles[0].Age != nil {
		t.Error("Age should be nil when DateOfBirth is nil")
	}
}

func TestBrowse_EmptySeedDefaultsToUserID(t *testing.T) {
	var capturedSeed string
	dob := time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC)
	m := &mockStore{
		browseProfiles: []BrowseProfile{{ID: "p1", UserID: "u1", DateOfBirth: &dob}},
	}
	m.browseFn = func(seed string) { capturedSeed = seed }
	svc := NewService(m)

	_, err := svc.Browse(t.Context(), "user-abc", 10, "", false, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedSeed != "user-abc" {
		t.Errorf("seed = %q, want %q (userID fallback)", capturedSeed, "user-abc")
	}
}

func TestBrowse_CursorCarriesAsOf(t *testing.T) {
	dob := time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC)
	score := 0.75
	svc := NewService(&mockStore{
		browseProfiles: []BrowseProfile{
			{ID: "p1", UserID: "u1", DateOfBirth: &dob},
			{ID: "p2", UserID: "u2", DateOfBirth: &dob},
			{ID: "p3", UserID: "u3", DateOfBirth: &dob},
		},
	})

	page1, err := svc.Browse(t.Context(), "r", 2, "", false, nil, "seed-x")
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	if page1.NextCursor == "" {
		t.Fatal("expected next cursor on page 1")
	}
	cur, err := DecodeBrowseCursor(page1.NextCursor)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	if cur.AsOf.IsZero() {
		t.Error("cursor AsOf should be set on first page")
	}
	_ = score
}

func TestBrowse_SeedPassedToStore(t *testing.T) {
	var capturedSeed string
	dob := time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC)
	m := &mockStore{
		browseProfiles: []BrowseProfile{{ID: "p1", UserID: "u1", DateOfBirth: &dob}},
	}
	m.browseFn = func(seed string) { capturedSeed = seed }
	svc := NewService(m)

	_, err := svc.Browse(t.Context(), "user-1", 10, "", false, nil, "custom-seed-42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedSeed != "custom-seed-42" {
		t.Errorf("seed = %q, want %q", capturedSeed, "custom-seed-42")
	}
}

func TestBrowse_DiscoveryPausedInPreferences(t *testing.T) {
	svc := NewService(&mockStore{})

	paused := true
	update := PreferencesUpdate{DiscoveryPaused: &paused}
	prefs, err := svc.UpdateMyPreferences(t.Context(), "u1", update)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !prefs.DiscoveryPaused {
		t.Error("DiscoveryPaused should be true after update")
	}
}

// --- UpdateAvatar ---

func TestUpdateAvatar_Success(t *testing.T) {
	svc := NewService(&mockStore{})

	if err := svc.UpdateAvatar(t.Context(), "user-1", "https://example.com/avatar.jpg"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- calcAge edge cases ---

func TestCalcAge_BirthdayNotYetThisYear(t *testing.T) {
	// If today is January and birthday is in December, age is (year diff - 1).
	now := time.Date(2024, time.January, 15, 0, 0, 0, 0, time.UTC)
	dob := time.Date(2000, time.December, 25, 0, 0, 0, 0, time.UTC)
	// 2024 - 2000 = 24, but birthday hasn't occurred yet → 23
	age := calcAge(dob, now)
	if age != 23 {
		t.Errorf("calcAge = %d, want 23", age)
	}
}

func TestCalcAge_SameDayNotYet(t *testing.T) {
	// Same month, but today's day is before birthday day.
	now := time.Date(2024, time.June, 10, 0, 0, 0, 0, time.UTC)
	dob := time.Date(2000, time.June, 20, 0, 0, 0, 0, time.UTC)
	// 2024 - 2000 = 24, but day 10 < 20 → 23
	age := calcAge(dob, now)
	if age != 23 {
		t.Errorf("calcAge = %d, want 23", age)
	}
}

// --- GetPublicProfile photos error ---

func TestGetPublicProfile_PhotosError(t *testing.T) {
	svc := NewService(&mockStore{
		profile:   &Profile{ID: "prof-1", UserID: "user-2", DisplayName: "Bob"},
		photosErr: errors.New("db error"),
	})

	_, err := svc.GetPublicProfile(t.Context(), "user-2")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- GetPublicProfileByUsername photos error ---

func TestGetPublicProfileByUsername_PhotosError(t *testing.T) {
	svc := NewService(&mockStore{
		profile:   &Profile{ID: "prof-1", UserID: "user-2", Username: "bob", DisplayName: "Bob"},
		photosErr: errors.New("db error"),
	})

	_, err := svc.GetPublicProfileByUsername(t.Context(), "bob")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- Browse: distance suppression for non-contacts ---

func TestBrowse_SuppressesDistanceForNonContactsWhenViewedHidesDistance(t *testing.T) {
	d1, d2, d3 := 5.0, 12.0, 30.0
	svc := NewService(&mockStore{
		browseProfiles: []BrowseProfile{
			{ID: "p1", UserID: "alice", DistanceKm: &d1},
			{ID: "p2", UserID: "bob", DistanceKm: &d2},
			{ID: "p3", UserID: "carol", DistanceKm: &d3},
		},
		privacyFlags: map[string]PrivacyFlags{
			"alice": {HideDistanceFromNonContacts: true},
			"bob":   {HideDistanceFromNonContacts: true},
			// carol has no row → defaults to all false; distance must remain.
		},
		contactIDs: []string{"alice"}, // viewer is contacts with alice only.
	})

	page, err := svc.Browse(t.Context(), "viewer", 10, "", false, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Profiles) != 3 {
		t.Fatalf("got %d profiles, want 3", len(page.Profiles))
	}
	// alice is a contact → distance preserved despite the flag.
	if page.Profiles[0].DistanceKm == nil || *page.Profiles[0].DistanceKm != d1 {
		t.Errorf("alice distance = %v, want %v (contact, flag does not apply)", page.Profiles[0].DistanceKm, d1)
	}
	// bob is not a contact and has the flag → distance must be nil.
	if page.Profiles[1].DistanceKm != nil {
		t.Errorf("bob distance = %v, want nil (non-contact + flag)", page.Profiles[1].DistanceKm)
	}
	// carol has no flag → distance preserved.
	if page.Profiles[2].DistanceKm == nil || *page.Profiles[2].DistanceKm != d3 {
		t.Errorf("carol distance = %v, want %v (no flag)", page.Profiles[2].DistanceKm, d3)
	}
}

func TestBrowse_PrivacyFlagsErrorPropagates(t *testing.T) {
	d := 5.0
	svc := NewService(&mockStore{
		browseProfiles:  []BrowseProfile{{ID: "p1", UserID: "alice", DistanceKm: &d}},
		privacyFlagsErr: errors.New("db error"),
	})

	_, err := svc.Browse(t.Context(), "viewer", 10, "", false, nil, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestBrowse_AcceptedContactIDsErrorPropagates(t *testing.T) {
	d := 5.0
	svc := NewService(&mockStore{
		browseProfiles: []BrowseProfile{{ID: "p1", UserID: "alice", DistanceKm: &d}},
		privacyFlags:   map[string]PrivacyFlags{"alice": {HideDistanceFromNonContacts: true}},
		contactIDsErr:  errors.New("db error"),
	})

	_, err := svc.Browse(t.Context(), "viewer", 10, "", false, nil, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- GetPrivacyFlags ---

func TestGetPrivacyFlags_DerivedFromPreferences(t *testing.T) {
	svc := NewService(&mockStore{
		prefs: &ProfilePreferences{
			UserID:                      "u-1",
			HideDistanceFromNonContacts: true,
			HidePresence:                false,
			HideReadReceipts:            true,
			HideTypingIndicator:         false,
		},
	})

	flags, err := svc.GetPrivacyFlags(t.Context(), "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !flags.HideDistanceFromNonContacts || flags.HidePresence || !flags.HideReadReceipts || flags.HideTypingIndicator {
		t.Errorf("flags = %+v", flags)
	}
}

// --- GetNotificationFlags ---

func TestGetNotificationFlags_DerivedFromPreferences(t *testing.T) {
	svc := NewService(&mockStore{
		prefs: &ProfilePreferences{
			UserID:                "u-1",
			NotifyChatMessages:    false,
			NotifyContactRequests: true,
			NotifyChannelMentions: false,
			NotifySystem:          true,
		},
	})

	flags, err := svc.GetNotificationFlags(t.Context(), "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if flags.ChatMessages || !flags.ContactRequests || flags.ChannelMentions || !flags.System {
		t.Errorf("flags = %+v", flags)
	}
}

func TestGetNotificationFlags_PropagatesStoreError(t *testing.T) {
	svc := NewService(&mockStore{prefsErr: errors.New("db error")})

	_, err := svc.GetNotificationFlags(t.Context(), "u-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
