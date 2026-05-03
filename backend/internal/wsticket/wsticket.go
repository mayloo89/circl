package wsticket

import (
	"context"
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

// Issuer creates single-use WebSocket tickets.
type Issuer interface {
	Issue(ctx context.Context, userID string) (string, error)
}

// Redeemer consumes a ticket and returns the associated userID.
// Tickets are deleted on first use.
type Redeemer interface {
	Redeem(ctx context.Context, ticket string) (string, error)
}

// Store implements both Issuer and Redeemer backed by Redis.
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
	if err := s.rdb.Set(ctx, keyPrefix+ticket, userID, ticketTTL).Err(); err != nil {
		return "", fmt.Errorf("wsticket issue: %w", err)
	}
	return ticket, nil
}

// Redeem atomically retrieves and deletes the ticket, returning the userID.
// Returns ErrInvalid if the ticket does not exist or has expired.
func (s *Store) Redeem(ctx context.Context, ticket string) (string, error) {
	userID, err := s.rdb.GetDel(ctx, keyPrefix+ticket).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrInvalid
	}
	if err != nil {
		return "", fmt.Errorf("wsticket redeem: %w", err)
	}
	return userID, nil
}
