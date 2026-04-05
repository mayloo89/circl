package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

const (
	minPasswordLen = 8
	maxPasswordLen = 128 // prevent bcrypt DoS via oversized input
)

var (
	// ErrInvalidCredentials is returned for any login failure.
	// A single error type prevents callers from distinguishing between
	// "user not found" and "wrong password", which would leak information.
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrEmailTaken is returned when trying to register an email that already exists.
	ErrEmailTaken = errors.New("email already taken")

	// ErrInvalidInput is returned for malformed or out-of-range input.
	ErrInvalidInput = errors.New("invalid input")

	// ErrAccountLocked is returned when the account has been temporarily locked
	// due to too many consecutive failed login attempts.
	ErrAccountLocked = errors.New("account locked")
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
}

// AccountManager handles authenticated account mutations.
// *Service satisfies this interface.
type AccountManager interface {
	ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error
	DeleteAccount(ctx context.Context, userID, password string) error
}

// userRecord is the internal DB representation of an authenticated user.
// It is unexported to keep the bcrypt hash inside the auth package only.
type userRecord struct {
	ID           string
	Email        string
	PasswordHash string
	Status       string
	IsAdmin      bool
}

// Service handles authentication business logic.
type Service struct {
	store Store
}

// NewService creates a new auth Service backed by the given Store.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// Login verifies the credentials and returns the authenticated user.
//
// Security notes:
//   - Always runs bcrypt even when the user is not found to prevent
//     timing-based user enumeration attacks.
//   - Returns the same ErrInvalidCredentials for every failure reason.
func (s *Service) Login(ctx context.Context, email, password string) (*User, error) {
	record, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		// Run bcrypt on a dummy hash so the response time is the same
		// whether the user exists or not.
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

	return &User{ID: record.ID, Email: record.Email, IsAdmin: record.IsAdmin}, nil
}

// Register creates a new local user account and returns the created user.
func (s *Service) Register(ctx context.Context, email, password string) (*User, error) {
	if err := validateEmail(email); err != nil {
		return nil, err
	}
	if err := validatePassword(password); err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	record, err := s.store.CreateUser(ctx, email, string(hash))
	if err != nil {
		return nil, err
	}

	return &User{ID: record.ID, Email: record.Email, IsAdmin: record.IsAdmin}, nil
}

// ChangePassword verifies currentPassword against the stored hash and replaces
// it with a freshly-hashed newPassword.
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

// DeleteAccount verifies password and soft-deletes the account by setting
// status = 'deleted', which blocks future logins.
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

// validateEmail checks that the given string is a valid email address.
func validateEmail(email string) error {
	if _, err := mail.ParseAddress(email); err != nil {
		return fmt.Errorf("%w: invalid email address", ErrInvalidInput)
	}
	return nil
}

// validatePassword checks that the password meets length and complexity requirements.
// Complexity rule: at least one uppercase letter, one lowercase letter, and one digit.
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
