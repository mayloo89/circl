package guest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	sessionTTL = 4 * time.Hour
	keyPrefix  = "guest:session:"
)

var ErrSessionNotFound = errors.New("guest session not found or expired")

// Session represents an ephemeral guest identity stored in Redis.
type Session struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
}

// SessionStore creates and retrieves guest sessions backed by Redis.
type SessionStore struct {
	rdb *redis.Client
}

// NewSessionStore returns a ready-to-use SessionStore.
func NewSessionStore(rdb *redis.Client) *SessionStore {
	return &SessionStore{rdb: rdb}
}

// Create generates a new guest session ID, stores nickname in Redis with a 4h TTL,
// and returns the session.
func (s *SessionStore) Create(ctx context.Context, nickname string) (*Session, error) {
	id := "guest:" + uuid.NewString()
	sess := &Session{ID: id, Nickname: nickname}
	if err := s.rdb.Set(ctx, keyPrefix+id, nickname, sessionTTL).Err(); err != nil {
		return nil, fmt.Errorf("guest session create: %w", err)
	}
	return sess, nil
}

// Get retrieves a guest session by ID. Returns ErrSessionNotFound if missing.
func (s *SessionStore) Get(ctx context.Context, sessionID string) (*Session, error) {
	nickname, err := s.rdb.Get(ctx, keyPrefix+sessionID).Result()
	if errors.Is(err, redis.Nil) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("guest session get: %w", err)
	}
	return &Session{ID: sessionID, Nickname: nickname}, nil
}
