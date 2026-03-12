package auth

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// mockStore is a test double for Store.
type mockStore struct {
	record *userRecord
	err    error
}

func (m *mockStore) GetUserByEmail(_ context.Context, _ string) (*userRecord, error) {
	return m.record, m.err
}

func hashPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	return string(hash)
}

func TestService_Login_Success(t *testing.T) {
	svc := NewService(&mockStore{
		record: &userRecord{
			ID:           "abc-123",
			Email:        "user@example.com",
			PasswordHash: hashPassword(t, "secret"),
			Status:       "active",
		},
	})

	user, err := svc.Login(context.Background(), "user@example.com", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != "abc-123" {
		t.Errorf("ID = %q, want %q", user.ID, "abc-123")
	}
	if user.Email != "user@example.com" {
		t.Errorf("Email = %q, want %q", user.Email, "user@example.com")
	}
}

func TestService_Login_UserNotFound(t *testing.T) {
	svc := NewService(&mockStore{err: errors.New("user not found")})

	_, err := svc.Login(context.Background(), "nobody@example.com", "password")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("got %v, want ErrInvalidCredentials", err)
	}
}

func TestService_Login_WrongPassword(t *testing.T) {
	svc := NewService(&mockStore{
		record: &userRecord{
			ID:           "abc-123",
			Email:        "user@example.com",
			PasswordHash: hashPassword(t, "correct"),
			Status:       "active",
		},
	})

	_, err := svc.Login(context.Background(), "user@example.com", "wrong")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("got %v, want ErrInvalidCredentials", err)
	}
}

func TestService_Login_SuspendedAccount(t *testing.T) {
	svc := NewService(&mockStore{
		record: &userRecord{
			ID:           "abc-123",
			Email:        "user@example.com",
			PasswordHash: hashPassword(t, "secret"),
			Status:       "suspended",
		},
	})

	_, err := svc.Login(context.Background(), "user@example.com", "secret")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("got %v, want ErrInvalidCredentials", err)
	}
}
