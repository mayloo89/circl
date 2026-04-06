package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/mayloo89/circl/backend/internal/email"
)

// mockStore is a test double for Store.
type mockStore struct {
	record    *userRecord
	getErr    error
	createErr error
	updateErr error
	deleteErr error
}

func (m *mockStore) GetUserByEmail(_ context.Context, _ string) (*userRecord, error) {
	return m.record, m.getErr
}

func (m *mockStore) CreateUser(_ context.Context, emailAddr, _ string) (*userRecord, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	return &userRecord{ID: "new-uuid", Email: emailAddr, Status: "active"}, nil
}

func (m *mockStore) GetUserByID(_ context.Context, _ string) (*userRecord, error) {
	return m.record, m.getErr
}

func (m *mockStore) UpdatePassword(_ context.Context, _, _ string) error { return m.updateErr }
func (m *mockStore) DeleteUser(_ context.Context, _ string) error        { return m.deleteErr }

func (m *mockStore) CreatePasswordReset(_ context.Context, _, _ string, _ time.Time) error {
	return nil
}
func (m *mockStore) GetPasswordReset(_ context.Context, _ string) (*passwordResetRecord, error) {
	return nil, errors.New("not found")
}
func (m *mockStore) MarkPasswordResetUsed(_ context.Context, _ string) error { return nil }
func (m *mockStore) CreateEmailVerification(_ context.Context, _, _ string, _ time.Time) error {
	return nil
}
func (m *mockStore) GetEmailVerification(_ context.Context, _ string) (*emailVerificationRecord, error) {
	return nil, errors.New("not found")
}
func (m *mockStore) MarkEmailVerified(_ context.Context, _, _ string) error { return nil }

// noopMailer satisfies email.Sender without actually sending anything.
type noopMailer struct{}

func (n *noopMailer) Send(_ context.Context, _ email.Message) error { return nil }

var noop = &noopMailer{}

func hashPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	return string(hash)
}

// verifiedAt is a convenience helper that returns a non-nil *time.Time for
// userRecord.EmailVerifiedAt, indicating a verified account.
func verifiedAt() *time.Time {
	t := time.Now().Add(-time.Hour)
	return &t
}

// --- Login ---

func TestService_Login_Success(t *testing.T) {
	svc := NewService(&mockStore{
		record: &userRecord{
			ID:              "abc-123",
			Email:           "user@example.com",
			PasswordHash:    hashPassword(t, "secret"),
			Status:          "active",
			EmailVerifiedAt: verifiedAt(),
		},
	}, noop)

	user, err := svc.Login(t.Context(), "user@example.com", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != "abc-123" {
		t.Errorf("ID = %q, want %q", user.ID, "abc-123")
	}
}

func TestService_Login_UserNotFound(t *testing.T) {
	svc := NewService(&mockStore{getErr: errors.New("user not found")}, noop)

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
	}, noop)

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
	}, noop)

	_, err := svc.Login(t.Context(), "user@example.com", "secret")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("got %v, want ErrInvalidCredentials", err)
	}
}

func TestService_Login_EmailNotVerified(t *testing.T) {
	svc := NewService(&mockStore{
		record: &userRecord{
			ID:              "abc-123",
			Email:           "user@example.com",
			PasswordHash:    hashPassword(t, "secret"),
			Status:          "active",
			EmailVerifiedAt: nil, // not verified
		},
	}, noop)

	_, err := svc.Login(t.Context(), "user@example.com", "secret")
	if !errors.Is(err, ErrEmailNotVerified) {
		t.Errorf("got %v, want ErrEmailNotVerified", err)
	}
}

// --- Register ---

func TestService_Register_Success(t *testing.T) {
	svc := NewService(&mockStore{}, noop)

	user, err := svc.Register(t.Context(), "new@example.com", "Secure1pass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Email != "new@example.com" {
		t.Errorf("email = %q, want %q", user.Email, "new@example.com")
	}
}

func TestService_Register_InvalidEmail(t *testing.T) {
	svc := NewService(&mockStore{}, noop)

	_, err := svc.Register(t.Context(), "not-an-email", "securepass")
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestService_Register_PasswordTooShort(t *testing.T) {
	svc := NewService(&mockStore{}, noop)

	_, err := svc.Register(t.Context(), "user@example.com", "short")
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestService_Register_PasswordTooLong(t *testing.T) {
	svc := NewService(&mockStore{}, noop)

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
	svc := NewService(&mockStore{createErr: ErrEmailTaken}, noop)

	_, err := svc.Register(t.Context(), "taken@example.com", "Secure1pass")
	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("got %v, want ErrEmailTaken", err)
	}
}

// --- ChangePassword ---

func TestService_ChangePassword_Success(t *testing.T) {
	svc := NewService(&mockStore{
		record: &userRecord{
			ID:           "abc-123",
			Email:        "user@example.com",
			PasswordHash: hashPassword(t, "OldPass1"),
			Status:       "active",
		},
	}, noop)

	if err := svc.ChangePassword(t.Context(), "abc-123", "OldPass1", "NewPass2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestService_ChangePassword_WrongCurrentPassword(t *testing.T) {
	svc := NewService(&mockStore{
		record: &userRecord{
			ID:           "abc-123",
			PasswordHash: hashPassword(t, "OldPass1"),
			Status:       "active",
		},
	}, noop)

	err := svc.ChangePassword(t.Context(), "abc-123", "WrongPass1", "NewPass2")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("got %v, want ErrInvalidCredentials", err)
	}
}

func TestService_ChangePassword_NewPasswordTooWeak(t *testing.T) {
	svc := NewService(&mockStore{
		record: &userRecord{
			ID:           "abc-123",
			PasswordHash: hashPassword(t, "OldPass1"),
			Status:       "active",
		},
	}, noop)

	err := svc.ChangePassword(t.Context(), "abc-123", "OldPass1", "weak")
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestService_ChangePassword_UserNotFound(t *testing.T) {
	svc := NewService(&mockStore{getErr: errors.New("user not found")}, noop)

	err := svc.ChangePassword(t.Context(), "abc-123", "OldPass1", "NewPass2")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("got %v, want ErrInvalidCredentials", err)
	}
}

func TestService_ChangePassword_UpdateError(t *testing.T) {
	svc := NewService(&mockStore{
		record: &userRecord{
			ID:           "abc-123",
			PasswordHash: hashPassword(t, "OldPass1"),
			Status:       "active",
		},
		updateErr: errors.New("db error"),
	}, noop)

	err := svc.ChangePassword(t.Context(), "abc-123", "OldPass1", "NewPass2")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- DeleteAccount ---

func TestService_DeleteAccount_Success(t *testing.T) {
	svc := NewService(&mockStore{
		record: &userRecord{
			ID:           "abc-123",
			PasswordHash: hashPassword(t, "MyPass1"),
			Status:       "active",
		},
	}, noop)

	if err := svc.DeleteAccount(t.Context(), "abc-123", "MyPass1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestService_DeleteAccount_WrongPassword(t *testing.T) {
	svc := NewService(&mockStore{
		record: &userRecord{
			ID:           "abc-123",
			PasswordHash: hashPassword(t, "MyPass1"),
			Status:       "active",
		},
	}, noop)

	err := svc.DeleteAccount(t.Context(), "abc-123", "WrongPass1")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("got %v, want ErrInvalidCredentials", err)
	}
}

func TestService_DeleteAccount_UserNotFound(t *testing.T) {
	svc := NewService(&mockStore{getErr: errors.New("user not found")}, noop)

	err := svc.DeleteAccount(t.Context(), "abc-123", "MyPass1")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("got %v, want ErrInvalidCredentials", err)
	}
}

// --- ForgotPassword ---

func TestService_ForgotPassword_UnknownEmail(t *testing.T) {
	// Always returns nil regardless of whether the email exists.
	svc := NewService(&mockStore{getErr: errors.New("not found")}, noop)
	if err := svc.ForgotPassword(t.Context(), "nobody@example.com", "http://localhost:3000"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestService_ForgotPassword_KnownEmail(t *testing.T) {
	svc := NewService(&mockStore{
		record: &userRecord{
			ID:     "abc-123",
			Email:  "user@example.com",
			Status: "active",
		},
	}, noop)
	if err := svc.ForgotPassword(t.Context(), "user@example.com", "http://localhost:3000"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- ResetPassword ---

func TestService_ResetPassword_EmptyToken(t *testing.T) {
	svc := NewService(&mockStore{}, noop)
	err := svc.ResetPassword(t.Context(), "", "NewPass1")
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestService_ResetPassword_InvalidToken(t *testing.T) {
	svc := NewService(&mockStore{}, noop) // GetPasswordReset returns "not found"
	err := svc.ResetPassword(t.Context(), "badtoken", "NewPass1")
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("got %v, want ErrInvalidToken", err)
	}
}

func TestService_ResetPassword_WeakPassword(t *testing.T) {
	ms := &mockStore{}
	ms.record = &userRecord{ID: "u1", Email: "u@e.com", Status: "active"}
	// Override GetPasswordReset via a dedicated mock
	prs := &mockStoreWithReset{
		mockStore: ms,
		resetRecord: &passwordResetRecord{
			ID:        "r1",
			UserID:    "u1",
			ExpiresAt: time.Now().Add(time.Hour),
		},
	}
	svc := NewService(prs, noop)
	err := svc.ResetPassword(t.Context(), "validtoken", "weak")
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

// --- VerifyEmail ---

func TestService_VerifyEmail_EmptyToken(t *testing.T) {
	svc := NewService(&mockStore{}, noop)
	err := svc.VerifyEmail(t.Context(), "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
}

func TestService_VerifyEmail_InvalidToken(t *testing.T) {
	svc := NewService(&mockStore{}, noop) // GetEmailVerification returns "not found"
	err := svc.VerifyEmail(t.Context(), "badtoken")
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("got %v, want ErrInvalidToken", err)
	}
}

func TestService_VerifyEmail_ExpiredToken(t *testing.T) {
	ev := &mockStoreWithVerification{
		mockStore: &mockStore{},
		evRecord: &emailVerificationRecord{
			ID:        "v1",
			UserID:    "u1",
			ExpiresAt: time.Now().Add(-time.Hour), // expired
		},
	}
	svc := NewService(ev, noop)
	err := svc.VerifyEmail(t.Context(), "sometoken")
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("got %v, want ErrInvalidToken", err)
	}
}

func TestService_VerifyEmail_AlreadyUsed(t *testing.T) {
	usedAt := time.Now()
	ev := &mockStoreWithVerification{
		mockStore: &mockStore{},
		evRecord: &emailVerificationRecord{
			ID:         "v1",
			UserID:     "u1",
			ExpiresAt:  time.Now().Add(time.Hour),
			VerifiedAt: &usedAt,
		},
	}
	svc := NewService(ev, noop)
	err := svc.VerifyEmail(t.Context(), "sometoken")
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("got %v, want ErrInvalidToken", err)
	}
}

// --- ResendVerification ---

func TestService_ResendVerification_UnknownEmail(t *testing.T) {
	svc := NewService(&mockStore{getErr: errors.New("not found")}, noop)
	if err := svc.ResendVerification(t.Context(), "nobody@example.com", "http://localhost:3000"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestService_ResendVerification_AlreadyVerified(t *testing.T) {
	svc := NewService(&mockStore{
		record: &userRecord{
			ID:              "u1",
			Email:           "u@e.com",
			Status:          "active",
			EmailVerifiedAt: verifiedAt(),
		},
	}, noop)
	if err := svc.ResendVerification(t.Context(), "u@e.com", "http://localhost:3000"); err != nil {
		t.Fatalf("unexpected error: %v", err)
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

// --- extended mocks for token-based tests ---

type mockStoreWithReset struct {
	*mockStore
	resetRecord *passwordResetRecord
	resetErr    error
}

func (m *mockStoreWithReset) GetPasswordReset(_ context.Context, _ string) (*passwordResetRecord, error) {
	if m.resetErr != nil {
		return nil, m.resetErr
	}
	return m.resetRecord, nil
}

type mockStoreWithVerification struct {
	*mockStore
	evRecord *emailVerificationRecord
	evErr    error
}

func (m *mockStoreWithVerification) GetEmailVerification(_ context.Context, _ string) (*emailVerificationRecord, error) {
	if m.evErr != nil {
		return nil, m.evErr
	}
	return m.evRecord, nil
}
