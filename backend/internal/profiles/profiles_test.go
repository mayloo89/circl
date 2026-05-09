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
	photo             *ProfilePhoto
	photoCount        int
	prefs             *ProfilePreferences
	interests         []InterestSuggestion
	browseProfiles    []BrowseProfile
	privacyFlags      map[string]PrivacyFlags
	contactIDs        []string
	browseErr         error
	usernameAvailable bool
	getErr            error
	upsertErr         error
	syncErr           error
	photosErr         error
	countErr          error
	addErr            error
	deleteErr         error
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

func (m *mockStore) UpsertPreferences(_ context.Context, _ string, prefs ProfilePreferences) (*ProfilePreferences, error) {
	if m.upsertPrefsErr != nil {
		return nil, m.upsertPrefsErr
	}
	if prefs.GenderPreference == nil {
		prefs.GenderPreference = []string{}
	}
	return &prefs, nil
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

func (m *mockStore) Browse(_ context.Context, _ string, _ int, _ string, _ bool, _ []string) ([]BrowseProfile, error) {
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

func TestUpdateMyPreferences_Success(t *testing.T) {
	minAge := 20
	maxAge := 35
	dist := 50
	svc := NewService(&mockStore{})

	p, err := svc.UpdateMyPreferences(t.Context(), "user-1", ProfilePreferences{
		MinAge: &minAge, MaxAge: &maxAge, MaxDistanceKm: &dist, GenderPreference: []string{"any"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.MaxAge == nil || *p.MaxAge != 35 {
		t.Errorf("MaxAge = %v, want 35", p.MaxAge)
	}
}

func TestUpdateMyPreferences_MinAgeTooLow(t *testing.T) {
	minAge := 16
	svc := NewService(&mockStore{})

	_, err := svc.UpdateMyPreferences(t.Context(), "user-1", ProfilePreferences{MinAge: &minAge})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestUpdateMyPreferences_MaxAgeTooHigh(t *testing.T) {
	maxAge := 200
	svc := NewService(&mockStore{})

	_, err := svc.UpdateMyPreferences(t.Context(), "user-1", ProfilePreferences{MaxAge: &maxAge})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestUpdateMyPreferences_MinGreaterThanMax(t *testing.T) {
	minAge := 40
	maxAge := 30
	svc := NewService(&mockStore{})

	_, err := svc.UpdateMyPreferences(t.Context(), "user-1", ProfilePreferences{MinAge: &minAge, MaxAge: &maxAge})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestUpdateMyPreferences_InvalidDistance(t *testing.T) {
	dist := 0
	svc := NewService(&mockStore{})

	_, err := svc.UpdateMyPreferences(t.Context(), "user-1", ProfilePreferences{MaxDistanceKm: &dist})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestUpdateMyPreferences_StoreError(t *testing.T) {
	svc := NewService(&mockStore{upsertPrefsErr: errors.New("db error")})

	_, err := svc.UpdateMyPreferences(t.Context(), "user-1", ProfilePreferences{})
	if err == nil {
		t.Fatal("expected error, got nil")
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

	page, err := svc.Browse(t.Context(), "requester", 10, "", false, nil)
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

	page, err := svc.Browse(t.Context(), "requester", 2, "", false, nil)
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

	_, err := svc.Browse(t.Context(), "requester", 20, "", false, nil)
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

	page, err := svc.Browse(t.Context(), "requester", 10, "", false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Profiles[0].Age != nil {
		t.Error("Age should be nil when DateOfBirth is nil")
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

	page, err := svc.Browse(t.Context(), "viewer", 10, "", false, nil)
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

	_, err := svc.Browse(t.Context(), "viewer", 10, "", false, nil)
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

	_, err := svc.Browse(t.Context(), "viewer", 10, "", false, nil)
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
