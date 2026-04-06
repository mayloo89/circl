package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"

	"github.com/mayloo89/circl/backend/internal/email"
)

const (
	minPasswordLen = 8
	maxPasswordLen = 128
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailTaken         = errors.New("email already taken")
	ErrInvalidInput       = errors.New("invalid input")
	ErrAccountLocked      = errors.New("account locked")
	ErrEmailNotVerified   = errors.New("email not verified")
	ErrInvalidToken       = errors.New("invalid or expired token")
)

// User holds the data returned after a successful login or registration.
type User struct {
	ID      string
	Email   string
	IsAdmin bool
}

// Store is the data-access interface required by the auth service.
type Store interface {
	GetUserByEmail(ctx context.Context, email string) (*userRecord, error)
	CreateUser(ctx context.Context, email, passwordHash string) (*userRecord, error)
	GetUserByID(ctx context.Context, userID string) (*userRecord, error)
	UpdatePassword(ctx context.Context, userID, newHash string) error
	DeleteUser(ctx context.Context, userID string) error
	CreatePasswordReset(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error
	GetPasswordReset(ctx context.Context, tokenHash string) (*passwordResetRecord, error)
	MarkPasswordResetUsed(ctx context.Context, id string) error
	CreateEmailVerification(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error
	GetEmailVerification(ctx context.Context, tokenHash string) (*emailVerificationRecord, error)
	MarkEmailVerified(ctx context.Context, userID, verificationID string) error
}

// AccountManager handles authenticated account mutations.
type AccountManager interface {
	ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error
	DeleteAccount(ctx context.Context, userID, password string) error
}

// EmailFlowService handles password reset and email verification.
type EmailFlowService interface {
	ForgotPassword(ctx context.Context, emailAddr, frontendURL string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
	SendVerificationEmail(ctx context.Context, userID, userEmail, frontendURL string) error
	ResendVerification(ctx context.Context, emailAddr, frontendURL string) error
	VerifyEmail(ctx context.Context, token string) error
}

// Authenticator is the interface the login/register handler depends on.
type Authenticator interface {
	Login(ctx context.Context, email, password string) (*User, error)
	Register(ctx context.Context, email, password string) (*User, error)
}

// userRecord is the internal DB representation of an authenticated user.
type userRecord struct {
	ID              string
	Email           string
	PasswordHash    string
	Status          string
	IsAdmin         bool
	EmailVerifiedAt *time.Time
}

// passwordResetRecord represents a password reset token row.
type passwordResetRecord struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
}

// emailVerificationRecord represents an email verification token row.
type emailVerificationRecord struct {
	ID         string
	UserID     string
	TokenHash  string
	ExpiresAt  time.Time
	VerifiedAt *time.Time
}

// Service handles authentication business logic.
type Service struct {
	store  Store
	mailer email.Sender
}

// NewService creates a new auth Service backed by the given Store and Sender.
func NewService(store Store, mailer email.Sender) *Service {
	return &Service{store: store, mailer: mailer}
}

// Login verifies credentials and returns the authenticated user.
// Returns ErrEmailNotVerified if the account exists and credentials are correct
// but the email address has not yet been confirmed.
func (s *Service) Login(ctx context.Context, emailAddr, password string) (*User, error) {
	record, err := s.store.GetUserByEmail(ctx, emailAddr)
	if err != nil {
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$dummyhashfordummypassword000000"), []byte(password))
		return nil, ErrInvalidCredentials
	}

	if record.Status != "active" {
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$dummyhashfordummypassword000000"), []byte(password))
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(record.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	if record.EmailVerifiedAt == nil {
		return nil, ErrEmailNotVerified
	}

	return &User{ID: record.ID, Email: record.Email, IsAdmin: record.IsAdmin}, nil
}

// Register creates a new local user account and returns the created user.
// It does not issue a JWT — the caller (handler) sends a verification email
// and returns a "check your email" response.
func (s *Service) Register(ctx context.Context, emailAddr, password string) (*User, error) {
	if err := validateEmail(emailAddr); err != nil {
		return nil, err
	}
	if err := validatePassword(password); err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	record, err := s.store.CreateUser(ctx, emailAddr, string(hash))
	if err != nil {
		return nil, err
	}

	return &User{ID: record.ID, Email: record.Email, IsAdmin: record.IsAdmin}, nil
}

// ChangePassword verifies currentPassword and replaces it with newPassword.
func (s *Service) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	record, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return ErrInvalidCredentials
	}
	if len(currentPassword) > maxPasswordLen {
		return ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(record.PasswordHash), []byte(currentPassword)); err != nil {
		return ErrInvalidCredentials
	}
	if err := validatePassword(newPassword); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.store.UpdatePassword(ctx, userID, string(hash))
}

// DeleteAccount verifies password and soft-deletes the account.
func (s *Service) DeleteAccount(ctx context.Context, userID, password string) error {
	record, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return ErrInvalidCredentials
	}
	if len(password) > maxPasswordLen {
		return ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(record.PasswordHash), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}
	return s.store.DeleteUser(ctx, userID)
}

// ForgotPassword generates a password reset token and sends the reset email.
// Always returns nil to prevent email enumeration.
func (s *Service) ForgotPassword(ctx context.Context, emailAddr, frontendURL string) error {
	record, err := s.store.GetUserByEmail(ctx, emailAddr)
	if err != nil || record.Status != "active" {
		return nil
	}

	plaintext, hash, err := generateSecureToken()
	if err != nil {
		return fmt.Errorf("generate reset token: %w", err)
	}

	expiresAt := time.Now().Add(time.Hour)
	if err := s.store.CreatePasswordReset(ctx, record.ID, hash, expiresAt); err != nil {
		return fmt.Errorf("store reset token: %w", err)
	}

	resetURL := frontendURL + "/reset-password?token=" + plaintext
	_ = s.mailer.Send(ctx, email.PasswordResetMessage(record.Email, resetURL))
	return nil
}

// ResetPassword validates the token and replaces the user's password.
func (s *Service) ResetPassword(ctx context.Context, plaintoken, newPassword string) error {
	if plaintoken == "" {
		return fmt.Errorf("%w: token is required", ErrInvalidInput)
	}

	tokenHash := hashToken(plaintoken)
	record, err := s.store.GetPasswordReset(ctx, tokenHash)
	if err != nil {
		return ErrInvalidToken
	}
	if record.UsedAt != nil || record.ExpiresAt.Before(time.Now()) {
		return ErrInvalidToken
	}

	if err := validatePassword(newPassword); err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if err := s.store.UpdatePassword(ctx, record.UserID, string(hash)); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return s.store.MarkPasswordResetUsed(ctx, record.ID)
}

// SendVerificationEmail generates a verification token and emails it to the user.
// Called by the register handler immediately after account creation.
func (s *Service) SendVerificationEmail(ctx context.Context, userID, userEmail, frontendURL string) error {
	plaintext, hash, err := generateSecureToken()
	if err != nil {
		return fmt.Errorf("generate verification token: %w", err)
	}

	expiresAt := time.Now().Add(24 * time.Hour)
	if err := s.store.CreateEmailVerification(ctx, userID, hash, expiresAt); err != nil {
		return fmt.Errorf("store verification token: %w", err)
	}

	verifyURL := frontendURL + "/verify-email?token=" + plaintext
	return s.mailer.Send(ctx, email.EmailVerificationMessage(userEmail, verifyURL))
}

// ResendVerification looks up the user by email and resends the verification email.
// Always returns nil to prevent enumeration.
func (s *Service) ResendVerification(ctx context.Context, emailAddr, frontendURL string) error {
	record, err := s.store.GetUserByEmail(ctx, emailAddr)
	if err != nil || record.Status != "active" || record.EmailVerifiedAt != nil {
		return nil
	}
	_ = s.SendVerificationEmail(ctx, record.ID, record.Email, frontendURL)
	return nil
}

// VerifyEmail validates the token and marks the user's email as verified.
func (s *Service) VerifyEmail(ctx context.Context, plaintoken string) error {
	if plaintoken == "" {
		return fmt.Errorf("%w: token is required", ErrInvalidInput)
	}

	tokenHash := hashToken(plaintoken)
	record, err := s.store.GetEmailVerification(ctx, tokenHash)
	if err != nil {
		return ErrInvalidToken
	}
	if record.VerifiedAt != nil || record.ExpiresAt.Before(time.Now()) {
		return ErrInvalidToken
	}

	return s.store.MarkEmailVerified(ctx, record.UserID, record.ID)
}

// --- helpers ---

func generateSecureToken() (plaintext, hash string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return
	}
	plaintext = hex.EncodeToString(raw)
	hash = hashToken(plaintext)
	return
}

func hashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

func validateEmail(addr string) error {
	if _, err := mail.ParseAddress(addr); err != nil {
		return fmt.Errorf("%w: invalid email address", ErrInvalidInput)
	}
	return nil
}

func validatePassword(password string) error {
	if len(password) < minPasswordLen {
		return fmt.Errorf("%w: password must be at least %d characters", ErrInvalidInput, minPasswordLen)
	}
	if len(password) > maxPasswordLen {
		return fmt.Errorf("%w: password too long", ErrInvalidInput)
	}
	var hasUpper, hasLower, hasDigit bool
	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		return fmt.Errorf("%w: password must contain at least one uppercase letter, one lowercase letter, and one digit", ErrInvalidInput)
	}
	return nil
}
