package auth

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// mockStore is a test double for Store.
type mockStore struct {
	record    *userRecord
	getErr    error
	createErr error
}

func (m *mockStore) GetUserByEmail(_ context.Context, _ string) (*userRecord, error) {
	return m.record, m.getErr
}

func (m *mockStore) CreateUser(_ context.Context, email, _ string) (*userRecord, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	return &userRecord{ID: "new-uuid", Email: email, Status: "active"}, nil
}

func hashPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	return string(hash)
}

// --- Login ---

func TestService_Login_Success(t *testing.T) {
	svc := NewService(&mockStore{
		record: &userRecord{
			ID:           "abc-123",
			Email:        "user@example.com",
			PasswordHash: hashPassword(t, "secret"),
			Status:       "active",
		},
	})

	user, err := svc.Login(t.Context(), "user@example.com", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != "abc-123" {
		t.Errorf("ID = %q, want %q", user.ID, "abc-123")
	}
}

func TestService_Login_UserNotFound(t *testing.T) {
	svc := NewService(&mockStore{getErr: errors.New("user not found")})

	_, err := svc.Login(t.Context(), "nobody@example.com", "password")
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

	_, err := svc.Login(t.Context(), "user@example.com", "wrong")
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

	_, err := svc.Login(t.Context(), "user@example.com", "secret")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("got %v, want ErrInvalidCredentials", err)
	}
}

// --- Register ---

func TestService_Register_Success(t *testing.T) {
	svc := NewService(&mockStore{})

	user, err := svc.Register(t.Context(), "new@example.com", "Secure1pass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Email != "new@example.com" {
		t.Errorf("email = %q, want %q", user.Email, "new@example.com")
	}
}

func TestService_Register_InvalidEmail(t *testing.T) {
	svc := NewService(&mockStore{})

	_, err := svc.Register(t.Context(), "not-an-email", "securepass")
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestService_Register_PasswordTooShort(t *testing.T) {
	svc := NewService(&mockStore{})

	_, err := svc.Register(t.Context(), "user@example.com", "short")
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestService_Register_PasswordTooLong(t *testing.T) {
	svc := NewService(&mockStore{})

	longPass := make([]byte, maxPasswordLen+1)
	for i := range longPass {
		longPass[i] = 'a'
	}

	_, err := svc.Register(t.Context(), "user@example.com", string(longPass))
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestService_Register_EmailTaken(t *testing.T) {
	svc := NewService(&mockStore{createErr: ErrEmailTaken})

	_, err := svc.Register(t.Context(), "taken@example.com", "Secure1pass")
	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("got %v, want ErrEmailTaken", err)
	}
}

// --- validateEmail ---

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"user@example.com", false},
		{"user+tag@sub.domain.com", false},
		{"notanemail", true},
		{"@nodomain", true},
		{"", true},
	}
	for _, tt := range tests {
		err := validateEmail(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateEmail(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
		}
	}
}

// --- validatePassword ---

func TestValidatePassword(t *testing.T) {
	maxValid := make([]byte, maxPasswordLen)
	for i := range maxValid {
		switch i % 3 {
		case 0:
			maxValid[i] = 'A'
		case 1:
			maxValid[i] = 'a'
		default:
			maxValid[i] = '1'
		}
	}

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid", "Secure1pass", false},
		{"exactly min length", "Abc1defg", false},
		{"too short", "Ab1d", true},
		{"too long", string(make([]byte, maxPasswordLen+1)), true},
		{"exactly max length", string(maxValid), false},
		{"missing uppercase", "secure1pass", true},
		{"missing lowercase", "SECURE1PASS", true},
		{"missing digit", "SecurePass!", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePassword(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePassword() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
