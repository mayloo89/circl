// Package appeals manages the suspension-appeal flow: a token is emailed to a
// suspended user, they submit a written statement via the locked-out
// /appeal/{token} page, and admin reviews + approves or denies the appeal.
//
// The appeal token is a 32-byte random value, stored hashed, valid for 30
// days. Approving an appeal reactivates the user; denying it leaves the
// suspension in place. Either outcome is emailed back to the user.
package appeals

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// TTL is how long an emailed appeal link remains valid. 30 days covers a
// reasonable holiday absence without leaving a never-expiring token in the
// wild.
const TTL = 30 * 24 * time.Hour

// Valid appeal statuses.
const (
	StatusOpen      = "open"
	StatusSubmitted = "submitted"
	StatusApproved  = "approved"
	StatusDenied    = "denied"
	StatusExpired   = "expired"
)

// Sentinel errors.
var (
	ErrNotFound         = errors.New("appeal not found")
	ErrInvalidToken     = errors.New("invalid or expired appeal token")
	ErrInvalidStatus    = errors.New("invalid appeal status")
	ErrAlreadySubmitted = errors.New("appeal already submitted")
	ErrAlreadyResolved  = errors.New("appeal already resolved")
)

// Appeal is the persisted state of a user's suspension appeal.
type Appeal struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id"`
	SuspensionID   string     `json:"suspension_id"`
	Status         string     `json:"status"`
	Body           string     `json:"body"`
	ExpiresAt      time.Time  `json:"expires_at"`
	SubmittedAt    *time.Time `json:"submitted_at,omitzero"`
	ResolvedAt     *time.Time `json:"resolved_at,omitzero"`
	ResolvedBy     *string    `json:"resolved_by,omitempty"`
	ResolutionNote string     `json:"resolution_note"`
	CreatedAt      time.Time  `json:"created_at"`
}

// AppealWithUserInfo augments an appeal with display fields for the admin
// table.
type AppealWithUserInfo struct {
	Appeal
	UserEmail string `json:"user_email"`
	UserName  string `json:"user_name"`
}

// PublicView is the subset of an appeal returned to the un-authenticated
// /appeal/{token} page. The user only needs to see the status and the body
// they previously typed (so they can refresh the page).
type PublicView struct {
	Status         string    `json:"status"`
	Body           string    `json:"body"`
	ExpiresAt      time.Time `json:"expires_at"`
	ResolutionNote string    `json:"resolution_note,omitempty"`
}

// Reactivator restores a suspended user to 'active' status once their appeal
// is approved. Satisfied by admin.Service.
type Reactivator interface {
	ReactivateUser(ctx context.Context, userID string) error
}

// Store is the persistence interface.
type Store interface {
	// Create inserts a new appeal in the 'open' state. tokenHash is the
	// SHA-256 of the plaintext token emailed to the user.
	Create(ctx context.Context, userID, suspensionID, tokenHash string, expiresAt time.Time) (*Appeal, error)
	// GetByTokenHash returns the appeal matching the hashed token.
	GetByTokenHash(ctx context.Context, tokenHash string) (*Appeal, error)
	// GetByID returns the appeal with the given ID.
	GetByID(ctx context.Context, id string) (*Appeal, error)
	// Submit records the user's statement and moves the appeal to 'submitted'.
	Submit(ctx context.Context, id, body string) (*Appeal, error)
	// Resolve marks the appeal as 'approved' or 'denied' and records the admin
	// who made the decision plus an optional note.
	Resolve(ctx context.Context, id, status, resolvedBy, note string) (*Appeal, error)
	// List returns appeals filtered by status, newest-first. Empty status
	// returns everything.
	List(ctx context.Context, status string) ([]AppealWithUserInfo, error)
}

// Service implements the appeals business logic.
type Service struct {
	store       Store
	reactivator Reactivator
}

// NewService creates an appeals Service.
func NewService(store Store, reactivator Reactivator) *Service {
	return &Service{store: store, reactivator: reactivator}
}

// CreateForSuspension generates a new appeal token, persists the appeal row,
// and returns the plaintext token so the caller can email it to the user. The
// plaintext is never stored — only its SHA-256 hash.
func (s *Service) CreateForSuspension(ctx context.Context, userID, suspensionID string) (plainToken string, appeal *Appeal, err error) {
	plain, hash, err := generateToken()
	if err != nil {
		return "", nil, fmt.Errorf("generate token: %w", err)
	}
	a, err := s.store.Create(ctx, userID, suspensionID, hash, time.Now().Add(TTL))
	if err != nil {
		return "", nil, err
	}
	return plain, a, nil
}

// GetPublic looks up an appeal by its plaintext token and returns the
// un-authenticated view. Expired tokens are reported as invalid.
func (s *Service) GetPublic(ctx context.Context, plainToken string) (*PublicView, error) {
	a, err := s.lookup(ctx, plainToken)
	if err != nil {
		return nil, err
	}
	return &PublicView{
		Status:         a.Status,
		Body:           a.Body,
		ExpiresAt:      a.ExpiresAt,
		ResolutionNote: a.ResolutionNote,
	}, nil
}

// SubmitPublic records the user's statement against the given token. Returns
// ErrAlreadySubmitted if the appeal has already been resolved.
func (s *Service) SubmitPublic(ctx context.Context, plainToken, body string) (*PublicView, error) {
	a, err := s.lookup(ctx, plainToken)
	if err != nil {
		return nil, err
	}
	if a.Status != StatusOpen && a.Status != StatusSubmitted {
		return nil, ErrAlreadyResolved
	}
	updated, err := s.store.Submit(ctx, a.ID, body)
	if err != nil {
		return nil, err
	}
	return &PublicView{
		Status:    updated.Status,
		Body:      updated.Body,
		ExpiresAt: updated.ExpiresAt,
	}, nil
}

// List returns appeals matching the optional status filter.
func (s *Service) List(ctx context.Context, status string) ([]AppealWithUserInfo, error) {
	if status != "" && !isValidStatus(status) {
		return nil, ErrInvalidStatus
	}
	return s.store.List(ctx, status)
}

// Resolve marks the appeal as approved or denied. Approval also reactivates
// the user so they can log in again. Denial leaves the suspension in place.
func (s *Service) Resolve(ctx context.Context, id, status, resolvedBy, note string) (*Appeal, error) {
	if status != StatusApproved && status != StatusDenied {
		return nil, ErrInvalidStatus
	}
	a, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if a.Status == StatusApproved || a.Status == StatusDenied {
		return nil, ErrAlreadyResolved
	}
	updated, err := s.store.Resolve(ctx, id, status, resolvedBy, note)
	if err != nil {
		return nil, err
	}
	if status == StatusApproved && s.reactivator != nil {
		if err := s.reactivator.ReactivateUser(ctx, updated.UserID); err != nil {
			return nil, fmt.Errorf("reactivate user: %w", err)
		}
	}
	return updated, nil
}

// lookup is the shared logic used by GetPublic and SubmitPublic: hash the
// plaintext, fetch by hash, treat expired tokens as invalid.
func (s *Service) lookup(ctx context.Context, plainToken string) (*Appeal, error) {
	if plainToken == "" {
		return nil, ErrInvalidToken
	}
	hash := HashToken(plainToken)
	a, err := s.store.GetByTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}
	if a.ExpiresAt.Before(time.Now()) || a.Status == StatusExpired {
		return nil, ErrInvalidToken
	}
	return a, nil
}

// HashToken returns the hex-encoded SHA-256 of the plaintext token. Exported
// because the store package needs it for direct lookups in tests.
func HashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func generateToken() (plain, hash string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return
	}
	plain = hex.EncodeToString(raw)
	hash = HashToken(plain)
	return
}

func isValidStatus(s string) bool {
	switch s {
	case StatusOpen, StatusSubmitted, StatusApproved, StatusDenied, StatusExpired:
		return true
	}
	return false
}
