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

// CurrentPolicyVersion is the version recorded against new users when they
// accept the legal terms at registration. Bump only when the published policy
// text changes in a way that requires re-consent from existing users.
const CurrentPolicyVersion = "v1"

const deletionGracePeriod = 30 * 24 * time.Hour

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
	ID          string
	Email       string
	Role        string
	Reactivated bool // true when login automatically restored a soft-deleted account
}

// CreateUserInput captures the fields the auth service writes when a new
// account is created, including the legal-consent record collected at
// registration time.
type CreateUserInput struct {
	Email                 string
	PasswordHash          string
	AcceptedTermsAt       time.Time
	AcceptedPrivacyAt     time.Time
	AcceptedPolicyVersion string
}

// RegistrationInput is the service-level payload for Register. The service
// hashes the password and stamps the consent timestamps before delegating to
// the store.
type RegistrationInput struct {
	Email                 string
	Password              string
	AcceptedPolicyVersion string
}

// Store is the data-access interface required by the auth service.
type Store interface {
	GetUserByEmail(ctx context.Context, email string) (*userRecord, error)
	CreateUser(ctx context.Context, in CreateUserInput) (*userRecord, error)
	GetUserByID(ctx context.Context, userID string) (*userRecord, error)
	UpdatePassword(ctx context.Context, userID, newHash string) error
	DeleteUser(ctx context.Context, userID string) error
	ReactivateUser(ctx context.Context, userID string) error
	GetExpiredDeletedUserIDs(ctx context.Context, before time.Time) ([]string, error)
	GetUserUploadKeys(ctx context.Context, userID string) (storageKeys, thumbnailKeys []string, err error)
	DeleteUserData(ctx context.Context, userID string) error
	AnonymizeUser(ctx context.Context, userID string) error
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

// AgeAttestation captures the evidence trail that a user claimed they were of
// legal age at registration. The row is preserved across hard-delete (the
// users FK is ON DELETE SET NULL) because the attestation outlives the
// account it created — investigators need to be able to prove what we knew
// and when.
type AgeAttestation struct {
	UserID        string
	UserEmail     string
	AttestedAge   int
	IP            string
	UserAgent     string
	DateOfBirth   *time.Time
	PolicyVersion string
}

// AgeAuditStore is the optional dependency that, when wired in, makes the
// register handler log each successful registration to the
// age_verification_audit table.
type AgeAuditStore interface {
	LogAgeAttestation(ctx context.Context, in AgeAttestation) error
}

// EmailFlowService handles password reset and email verification.
type EmailFlowService interface {
	ForgotPassword(ctx context.Context, emailAddr, frontendURL string) error
	// ResetPassword validates the token, replaces the password, and returns the
	// affected userID so callers can revoke existing sessions.
	ResetPassword(ctx context.Context, token, newPassword string) (userID string, err error)
	SendVerificationEmail(ctx context.Context, userID, userEmail, frontendURL string) error
	ResendVerification(ctx context.Context, emailAddr, frontendURL string) error
	VerifyEmail(ctx context.Context, token string) error
}

// Authenticator is the interface the login/register handler depends on.
type Authenticator interface {
	Login(ctx context.Context, email, password string) (*User, error)
	Register(ctx context.Context, in RegistrationInput) (*User, error)
}

// userRecord is the internal DB representation of an authenticated user.
type userRecord struct {
	ID              string
	Email           string
	PasswordHash    string
	Status          string
	Role            string
	EmailVerifiedAt *time.Time
	DeletedAt       *time.Time
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
	store       Store
	mailer      email.Sender
	frontendURL string
}

// NewService creates a new auth Service backed by the given Store and Sender.
func NewService(store Store, mailer email.Sender, frontendURL string) *Service {
	return &Service{store: store, mailer: mailer, frontendURL: frontendURL}
}

// Login verifies credentials and returns the authenticated user.
// If the account was soft-deleted within the 30-day grace period it is
// automatically reactivated and the returned User has Reactivated set to true.
// Returns ErrEmailNotVerified if the email has not been confirmed.
func (s *Service) Login(ctx context.Context, emailAddr, password string) (*User, error) {
	record, err := s.store.GetUserByEmail(ctx, emailAddr)
	if err != nil {
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$dummyhashfordummypassword000000"), []byte(password))
		return nil, ErrInvalidCredentials
	}

	withinGrace := record.DeletedAt != nil && time.Since(*record.DeletedAt) < deletionGracePeriod

	if record.Status != "active" && !withinGrace {
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$dummyhashfordummypassword000000"), []byte(password))
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(record.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	if withinGrace {
		if err := s.store.ReactivateUser(ctx, record.ID); err != nil {
			return nil, fmt.Errorf("reactivate user: %w", err)
		}
		return &User{ID: record.ID, Email: record.Email, Role: record.Role, Reactivated: true}, nil
	}

	if record.EmailVerifiedAt == nil {
		return nil, ErrEmailNotVerified
	}

	return &User{ID: record.ID, Email: record.Email, Role: record.Role}, nil
}

// Register creates a new local user account and returns the created user.
// It does not issue a JWT — the caller (handler) sends a verification email
// and returns a "check your email" response. The legal-consent timestamps are
// stamped server-side at the moment the row is written.
func (s *Service) Register(ctx context.Context, in RegistrationInput) (*User, error) {
	if err := validateEmail(in.Email); err != nil {
		return nil, err
	}
	if err := validatePassword(in.Password); err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost+2)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	now := time.Now()
	record, err := s.store.CreateUser(ctx, CreateUserInput{
		Email:                 in.Email,
		PasswordHash:          string(hash),
		AcceptedTermsAt:       now,
		AcceptedPrivacyAt:     now,
		AcceptedPolicyVersion: in.AcceptedPolicyVersion,
	})
	if err != nil {
		return nil, err
	}

	return &User{ID: record.ID, Email: record.Email, Role: record.Role}, nil
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
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost+2)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.store.UpdatePassword(ctx, userID, string(hash))
}

// DeleteAccount verifies password and soft-deletes the account.
// The account enters a 30-day grace period during which it can be reactivated.
// A warning email is sent asynchronously so the user knows how to undo the deletion.
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
	if err := s.store.DeleteUser(ctx, userID); err != nil {
		return err
	}
	loginURL := s.frontendURL + "/login"
	bgCtx := context.WithoutCancel(ctx)
	go func() {
		_ = s.mailer.Send(bgCtx, email.AccountDeletionMessage(s.frontendURL, record.Email, loginURL))
	}()
	return nil
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
	_ = s.mailer.Send(ctx, email.PasswordResetMessage(s.frontendURL, record.Email, resetURL))
	return nil
}

// ResetPassword validates the token, replaces the user's password, and returns
// the affected userID so the caller can revoke existing refresh tokens.
func (s *Service) ResetPassword(ctx context.Context, plaintoken, newPassword string) (string, error) {
	if plaintoken == "" {
		return "", fmt.Errorf("%w: token is required", ErrInvalidInput)
	}

	tokenHash := hashToken(plaintoken)
	record, err := s.store.GetPasswordReset(ctx, tokenHash)
	if err != nil {
		return "", ErrInvalidToken
	}
	if record.UsedAt != nil || record.ExpiresAt.Before(time.Now()) {
		return "", ErrInvalidToken
	}

	if err := validatePassword(newPassword); err != nil {
		return "", err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost+2)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	if err := s.store.UpdatePassword(ctx, record.UserID, string(hash)); err != nil {
		return "", fmt.Errorf("update password: %w", err)
	}
	return record.UserID, s.store.MarkPasswordResetUsed(ctx, record.ID)
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
	return s.mailer.Send(ctx, email.EmailVerificationMessage(s.frontendURL, userEmail, verifyURL))
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
