package wsticket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	ticketTTL = 60 * time.Second
	keyPrefix = "ws:ticket:"
)

// ErrInvalid is returned when a ticket does not exist or has already been used.
var ErrInvalid = errors.New("ws ticket invalid or expired")

// TicketData holds the identity payload stored in a WS ticket.
// For registered users, only UserID is set. For guests, IsGuest is true
// and GuestNickname carries the ephemeral display name.
type TicketData struct {
	UserID        string `json:"user_id"`
	IsGuest       bool   `json:"is_guest"`
	GuestNickname string `json:"guest_nickname,omitempty"`
}

// Issuer creates single-use WebSocket tickets.
type Issuer interface {
	Issue(ctx context.Context, userID string) (string, error)
}

// GuestIssuer creates single-use WebSocket tickets for guest sessions.
type GuestIssuer interface {
	IssueGuest(ctx context.Context, sessionID, nickname string) (string, error)
}

// Redeemer consumes a ticket and returns the associated TicketData.
// Tickets are deleted on first use.
type Redeemer interface {
	Redeem(ctx context.Context, ticket string) (TicketData, error)
}

// Store implements Issuer, GuestIssuer, and Redeemer backed by Redis.
type Store struct {
	rdb *redis.Client
}

// NewStore returns a ready-to-use Store.
func NewStore(rdb *redis.Client) *Store {
	return &Store{rdb: rdb}
}

// Issue generates a UUID ticket, stores userID in Redis with a 60s TTL, and
// returns the ticket string.
func (s *Store) Issue(ctx context.Context, userID string) (string, error) {
	ticket := uuid.NewString()
	data := TicketData{UserID: userID}
	if err := s.storeJSON(ctx, ticket, data); err != nil {
		return "", err
	}
	return ticket, nil
}

// IssueGuest generates a UUID ticket for a guest session and stores it in Redis.
func (s *Store) IssueGuest(ctx context.Context, sessionID, nickname string) (string, error) {
	ticket := uuid.NewString()
	data := TicketData{
		UserID:        sessionID,
		IsGuest:       true,
		GuestNickname: nickname,
	}
	if err := s.storeJSON(ctx, ticket, data); err != nil {
		return "", err
	}
	return ticket, nil
}

// Redeem atomically retrieves and deletes the ticket, returning the TicketData.
// Returns ErrInvalid if the ticket does not exist or has expired.
func (s *Store) Redeem(ctx context.Context, ticket string) (TicketData, error) {
	raw, err := s.rdb.GetDel(ctx, keyPrefix+ticket).Result()
	if errors.Is(err, redis.Nil) {
		return TicketData{}, ErrInvalid
	}
	if err != nil {
		return TicketData{}, fmt.Errorf("wsticket redeem: %w", err)
	}
	var data TicketData
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return TicketData{}, fmt.Errorf("wsticket redeem: unmarshal: %w", err)
	}
	return data, nil
}

func (s *Store) storeJSON(ctx context.Context, ticket string, data TicketData) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("wsticket issue: marshal: %w", err)
	}
	if err := s.rdb.Set(ctx, keyPrefix+ticket, b, ticketTTL).Err(); err != nil {
		return fmt.Errorf("wsticket issue: %w", err)
	}
	return nil
}
