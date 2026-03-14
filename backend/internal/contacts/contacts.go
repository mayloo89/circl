package contacts

import (
	"context"
	"errors"
	"time"
)

// Status values for a contact relationship.
const (
	StatusPending  = "pending"
	StatusAccepted = "accepted"
	StatusBlocked  = "blocked"
)

// Sentinel errors returned by the service and store layers.
var (
	ErrNotFound      = errors.New("contact not found")
	ErrAlreadyExists = errors.New("contact request already exists")
	ErrSelfContact   = errors.New("cannot add yourself as a contact")
	ErrForbidden     = errors.New("forbidden")
)

// Contact represents a directed relationship between two users.
type Contact struct {
	ID          string    `json:"id"`
	RequesterID string    `json:"requester_id"`
	AddresseeID string    `json:"addressee_id"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UserSummary is a lightweight view of a user returned in search results
// and contact lists.
type UserSummary struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

// AcceptedContact represents an accepted contact with the contact row ID
// (needed to remove) and the peer user's details.
type AcceptedContact struct {
	ContactID   string `json:"contact_id"`
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

// PendingRequest represents an incoming pending contact request with the
// contact row ID (needed to accept/decline) and the requester's details.
type PendingRequest struct {
	ContactID   string `json:"contact_id"`
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

// SentRequest represents an outgoing pending contact request with the
// contact row ID (needed to cancel) and the addressee's details.
type SentRequest struct {
	ContactID   string `json:"contact_id"`
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

// Store is the persistence interface required by the service.
type Store interface {
	// SendRequest creates a pending contact request from requesterID to addresseeID.
	SendRequest(ctx context.Context, requesterID, addresseeID string) (*Contact, error)
	// Accept transitions a pending request to accepted.
	// Only the addressee may accept; returns ErrForbidden otherwise.
	Accept(ctx context.Context, contactID, addresseeID string) (*Contact, error)
	// Delete removes a contact row regardless of status.
	// Either participant may delete; returns ErrForbidden otherwise.
	Delete(ctx context.Context, contactID, userID string) error
	// ListAccepted returns all accepted contacts for the given user.
	ListAccepted(ctx context.Context, userID string) ([]AcceptedContact, error)
	// ListPending returns incoming pending requests for the given user.
	ListPending(ctx context.Context, addresseeID string) ([]PendingRequest, error)
	// ListSent returns outgoing pending requests sent by the given user.
	ListSent(ctx context.Context, requesterID string) ([]SentRequest, error)
	// SearchUsers returns users whose email or display_name matches the query,
	// excluding the requesting user.
	SearchUsers(ctx context.Context, query, excludeUserID string) ([]UserSummary, error)
}

// Service implements the contacts business logic.
type Service struct {
	store Store
}

// NewService creates a Service backed by the given store.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// SendRequest creates a contact request from requesterID to addresseeID.
func (s *Service) SendRequest(ctx context.Context, requesterID, addresseeID string) (*Contact, error) {
	if requesterID == addresseeID {
		return nil, ErrSelfContact
	}
	return s.store.SendRequest(ctx, requesterID, addresseeID)
}

// Accept marks the contact as accepted. Only the addressee can accept.
func (s *Service) Accept(ctx context.Context, contactID, userID string) (*Contact, error) {
	return s.store.Accept(ctx, contactID, userID)
}

// Delete removes a contact. Either participant can delete.
func (s *Service) Delete(ctx context.Context, contactID, userID string) error {
	return s.store.Delete(ctx, contactID, userID)
}

// ListAccepted returns the accepted contacts for the given user.
func (s *Service) ListAccepted(ctx context.Context, userID string) ([]AcceptedContact, error) {
	return s.store.ListAccepted(ctx, userID)
}

// ListPending returns incoming pending requests for the given user.
func (s *Service) ListPending(ctx context.Context, userID string) ([]PendingRequest, error) {
	return s.store.ListPending(ctx, userID)
}

// ListSent returns outgoing pending requests sent by the given user.
func (s *Service) ListSent(ctx context.Context, userID string) ([]SentRequest, error) {
	return s.store.ListSent(ctx, userID)
}

// SearchUsers searches for users by email or display name.
func (s *Service) SearchUsers(ctx context.Context, query, userID string) ([]UserSummary, error) {
	if query == "" {
		return []UserSummary{}, nil
	}
	return s.store.SearchUsers(ctx, query, userID)
}
