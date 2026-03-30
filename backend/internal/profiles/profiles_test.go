package profiles

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockStore struct {
	profile        *Profile
	photos         []ProfilePhoto
	photo          *ProfilePhoto
	photoCount     int
	prefs          *ProfilePreferences
	interests      []InterestSuggestion
	getErr         error
	upsertErr      error
	syncErr        error
	photosErr      error
	countErr       error
	addErr         error
	deleteErr      error
	prefsErr       error
	upsertPrefsErr error
	searchIntErr   error
}

func (m *mockStore) GetByUserID(_ context.Context, _ string) (*Profile, error) {
	return m.profile, m.getErr
}

func (m *mockStore) Upsert(_ context.Context, userID string, in ProfileInput) (*Profile, error) {
	if m.upsertErr != nil {
		return nil, m.upsertErr
	}
	return &Profile{
		ID: "prof-1", UserID: userID, DisplayName: in.DisplayName, Bio: in.Bio, AvatarURL: in.AvatarURL,
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

	p, err := svc.UpdateMyProfile(t.Context(), "user-1", ProfileInput{DisplayName: "Alice", Bio: "Bio here"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.DisplayName != "Alice" {
		t.Errorf("DisplayName = %q, want %q", p.DisplayName, "Alice")
	}
}

func TestUpdateMyProfile_WithNewFields(t *testing.T) {
	dob := time.Now().AddDate(-25, 0, 0)
	lat := 48.8566
	lon := 2.3522
	svc := NewService(&mockStore{})

	p, err := svc.UpdateMyProfile(t.Context(), "user-1", ProfileInput{
		DisplayName:  "Alice",
		DateOfBirth:  &dob,
		Gender:       "female",
		LocationText: "Paris",
		Latitude:     &lat,
		Longitude:    &lon,
		Interests:    []string{"hiking", "music"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Gender != "female" {
		t.Errorf("Gender = %q, want female", p.Gender)
	}
	if len(p.Interests) != 2 {
		t.Errorf("Interests len = %d, want 2", len(p.Interests))
	}
}

func TestUpdateMyProfile_EmptyDisplayName(t *testing.T) {
	svc := NewService(&mockStore{})

	_, err := svc.UpdateMyProfile(t.Context(), "user-1", ProfileInput{Bio: "Bio"})
	if err == nil {
		t.Fatal("expected error for empty display name")
	}
}

func TestUpdateMyProfile_TooYoung(t *testing.T) {
	dob := time.Now().AddDate(-17, 0, 0) // 17 years old
	svc := NewService(&mockStore{})

	_, err := svc.UpdateMyProfile(t.Context(), "user-1", ProfileInput{DisplayName: "Alice", DateOfBirth: &dob})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestUpdateMyProfile_TooManyInterests(t *testing.T) {
	interests := make([]string, MaxInterests+1)
	for i := range interests {
		interests[i] = "tag"
	}
	svc := NewService(&mockStore{})

	_, err := svc.UpdateMyProfile(t.Context(), "user-1", ProfileInput{DisplayName: "Alice", Interests: interests})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestUpdateMyProfile_StoreError(t *testing.T) {
	svc := NewService(&mockStore{upsertErr: errors.New("db error")})

	_, err := svc.UpdateMyProfile(t.Context(), "user-1", ProfileInput{DisplayName: "Alice"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUpdateMyProfile_SyncInterestsError(t *testing.T) {
	svc := NewService(&mockStore{syncErr: errors.New("db error")})

	_, err := svc.UpdateMyProfile(t.Context(), "user-1", ProfileInput{DisplayName: "Alice", Interests: []string{"hiking"}})
	if err == nil {
		t.Fatal("expected error, got nil")
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
