package profiles

import (
	"context"
	"errors"
	"testing"
)

type mockStore struct {
	profile    *Profile
	photos     []ProfilePhoto
	photo      *ProfilePhoto
	photoCount int
	getErr     error
	upsertErr  error
	photosErr  error
	countErr   error
	addErr     error
	deleteErr  error
}

func (m *mockStore) GetByUserID(_ context.Context, _ string) (*Profile, error) {
	return m.profile, m.getErr
}

func (m *mockStore) Upsert(_ context.Context, userID, displayName, bio, avatarURL string) (*Profile, error) {
	if m.upsertErr != nil {
		return nil, m.upsertErr
	}
	return &Profile{ID: "prof-1", UserID: userID, DisplayName: displayName, Bio: bio, AvatarURL: avatarURL}, nil
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

	p, err := svc.UpdateMyProfile(t.Context(), "user-1", "Alice", "Bio here", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.DisplayName != "Alice" {
		t.Errorf("DisplayName = %q, want %q", p.DisplayName, "Alice")
	}
}

func TestUpdateMyProfile_EmptyDisplayName(t *testing.T) {
	svc := NewService(&mockStore{})

	_, err := svc.UpdateMyProfile(t.Context(), "user-1", "", "Bio", "")
	if err == nil {
		t.Fatal("expected error for empty display name")
	}
}

func TestUpdateMyProfile_StoreError(t *testing.T) {
	svc := NewService(&mockStore{upsertErr: errors.New("db error")})

	_, err := svc.UpdateMyProfile(t.Context(), "user-1", "Alice", "Bio", "")
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
