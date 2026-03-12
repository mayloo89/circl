package auth

import (
	"context"
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// ErrInvalidCredentials is returned for any authentication failure.
// A single error type prevents callers from distinguishing between
// "user not found" and "wrong password", which would leak information.
var ErrInvalidCredentials = errors.New("invalid credentials")

// User holds the data returned after a successful login.
type User struct {
	ID    string
	Email string
}

// Store is the data-access interface required by the auth service.
type Store interface {
	GetUserByEmail(ctx context.Context, email string) (*userRecord, error)
}

// userRecord is the internal DB representation of an authenticated user.
// It is unexported to keep the bcrypt hash inside the auth package only.
type userRecord struct {
	ID           string
	Email        string
	PasswordHash string
	Status       string
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

	return &User{ID: record.ID, Email: record.Email}, nil
}
