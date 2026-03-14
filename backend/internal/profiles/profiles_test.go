package profiles

import (
	"context"
	"errors"
	"testing"
)

type mockStore struct {
	profile   *Profile
	getErr    error
	upsertErr error
}

func (m *mockStore) GetByUserID(_ context.Context, _ string) (*Profile, error) {
	return m.profile, m.getErr
}

func (m *mockStore) Upsert(_ context.Context, userID, displayName, bio string) (*Profile, error) {
	if m.upsertErr != nil {
		return nil, m.upsertErr
	}
	return &Profile{ID: "prof-1", UserID: userID, DisplayName: displayName, Bio: bio}, nil
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

// --- UpdateMyProfile ---

func TestUpdateMyProfile_Success(t *testing.T) {
	svc := NewService(&mockStore{})

	p, err := svc.UpdateMyProfile(t.Context(), "user-1", "Alice", "Bio here")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.DisplayName != "Alice" {
		t.Errorf("DisplayName = %q, want %q", p.DisplayName, "Alice")
	}
}

func TestUpdateMyProfile_EmptyDisplayName(t *testing.T) {
	svc := NewService(&mockStore{})

	_, err := svc.UpdateMyProfile(t.Context(), "user-1", "", "Bio")
	if err == nil {
		t.Fatal("expected error for empty display name")
	}
}

func TestUpdateMyProfile_StoreError(t *testing.T) {
	svc := NewService(&mockStore{upsertErr: errors.New("db error")})

	_, err := svc.UpdateMyProfile(t.Context(), "user-1", "Alice", "Bio")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
