package guest

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	sessionTTL    = 4 * time.Hour
	keyPrefix     = "guest:session:"
	ipKeyPrefix   = "guest:session:ip:"
	banKeyPrefix  = "guest:ban:"
	muteKeyPrefix = "guest:muted:"
)

var ErrSessionNotFound = errors.New("guest session not found or expired")

// Session represents an ephemeral guest identity stored in Redis.
type Session struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	// IPHash is the SHA-256 hex of the client IP stored at session creation.
	// It is used to enforce per-room IP bans when a guest is kicked.
	IPHash string `json:"ip_hash,omitempty"`
}

// SessionStore creates and retrieves guest sessions backed by Redis.
type SessionStore struct {
	rdb *redis.Client
}

// NewSessionStore returns a ready-to-use SessionStore.
func NewSessionStore(rdb *redis.Client) *SessionStore {
	return &SessionStore{rdb: rdb}
}

// HashIP returns a stable SHA-256 hex string for the given IP address.
// The raw IP is never stored.
func HashIP(ip string) string {
	h := sha256.Sum256([]byte(ip))
	return fmt.Sprintf("%x", h)
}

// Create generates a new guest session ID, stores nickname and IP hash in
// Redis with a 4h TTL, and returns the session.
func (s *SessionStore) Create(ctx context.Context, nickname, ipHash string) (*Session, error) {
	id := "guest:" + uuid.NewString()
	sess := &Session{ID: id, Nickname: nickname, IPHash: ipHash}
	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, keyPrefix+id, nickname, sessionTTL)
	if ipHash != "" {
		pipe.Set(ctx, ipKeyPrefix+id, ipHash, sessionTTL)
	}
	if _, err := pipe.Exec(ctx); err != nil {
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
	// Best-effort: retrieve IP hash (may not exist for sessions created before this feature).
	ipHash, _ := s.rdb.Get(ctx, ipKeyPrefix+sessionID).Result()
	return &Session{ID: sessionID, Nickname: nickname, IPHash: ipHash}, nil
}

// BanRoomIP stores a per-room IP ban in Redis with the given TTL.
func (s *SessionStore) BanRoomIP(ctx context.Context, roomID, ipHash string, ttl time.Duration) error {
	return s.rdb.Set(ctx, banKeyPrefix+roomID+":"+ipHash, "1", ttl).Err()
}

// IsRoomIPBanned returns true when the IP hash is banned from the given room.
func (s *SessionStore) IsRoomIPBanned(ctx context.Context, roomID, ipHash string) (bool, error) {
	exists, err := s.rdb.Exists(ctx, banKeyPrefix+roomID+":"+ipHash).Result()
	if err != nil {
		return false, fmt.Errorf("guest ip ban check: %w", err)
	}
	return exists > 0, nil
}

// MuteInRoom stores a per-room mute in Redis with the given TTL.
func (s *SessionStore) MuteInRoom(ctx context.Context, roomID, userID string, ttl time.Duration) error {
	return s.rdb.Set(ctx, muteKeyPrefix+roomID+":"+userID, "1", ttl).Err()
}

// UnmuteInRoom removes a per-room mute.
func (s *SessionStore) UnmuteInRoom(ctx context.Context, roomID, userID string) error {
	return s.rdb.Del(ctx, muteKeyPrefix+roomID+":"+userID).Err()
}

// IsMutedInRoom returns true when the user is muted in the given room.
func (s *SessionStore) IsMutedInRoom(ctx context.Context, roomID, userID string) bool {
	exists, err := s.rdb.Exists(ctx, muteKeyPrefix+roomID+":"+userID).Result()
	if err != nil {
		return false // fail open: don't block on Redis errors
	}
	return exists > 0
}
