package auth

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	refreshTokenTTL  = 7 * 24 * time.Hour
	revocationKeyTTL = refreshTokenTTL + time.Hour

	rtKeyPrefix      = "rt:"
	rtRevokedPrefix  = "rt:revoked_at:"
)

// RefreshTokenStore manages opaque refresh tokens backed by Redis.
type RefreshTokenStore interface {
	Create(ctx context.Context, hash, userID, role string, issuedAt time.Time) error
	Get(ctx context.Context, hash string) (*refreshTokenRecord, error)
	Delete(ctx context.Context, hash string) error
	RevokeAllForUser(ctx context.Context, userID string) error
}

type refreshTokenRecord struct {
	UserID   string    `json:"user_id"`
	Role     string    `json:"role"`
	IssuedAt time.Time `json:"issued_at"`
}

type redisRefreshStore struct {
	rdb *redis.Client
}

// NewRedisRefreshStore returns a RefreshTokenStore backed by Redis.
func NewRedisRefreshStore(rdb *redis.Client) RefreshTokenStore {
	return &redisRefreshStore{rdb: rdb}
}

func (s *redisRefreshStore) Create(ctx context.Context, hash, userID, role string, issuedAt time.Time) error {
	val, err := json.Marshal(refreshTokenRecord{UserID: userID, Role: role, IssuedAt: issuedAt})
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, rtKeyPrefix+hash, val, refreshTokenTTL).Err()
}

// Get retrieves the token record and validates it against any pending revocation.
// Returns ErrInvalidToken when the token does not exist, has been deleted, or was
// issued before a RevokeAllForUser call.
func (s *redisRefreshStore) Get(ctx context.Context, hash string) (*refreshTokenRecord, error) {
	raw, err := s.rdb.Get(ctx, rtKeyPrefix+hash).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrInvalidToken
	}
	if err != nil {
		return nil, err
	}

	var rt refreshTokenRecord
	if err := json.Unmarshal(raw, &rt); err != nil {
		return nil, err
	}

	// Check user-level revocation (password change, account deletion).
	revokedAt, err := s.rdb.Get(ctx, rtRevokedPrefix+rt.UserID).Int64()
	if err == nil && rt.IssuedAt.UnixNano() <= revokedAt {
		_ = s.rdb.Del(ctx, rtKeyPrefix+hash)
		return nil, ErrInvalidToken
	}

	return &rt, nil
}

func (s *redisRefreshStore) Delete(ctx context.Context, hash string) error {
	return s.rdb.Del(ctx, rtKeyPrefix+hash).Err()
}

// RevokeAllForUser invalidates every refresh token issued to userID before now.
// Tokens issued after this call are unaffected.
func (s *redisRefreshStore) RevokeAllForUser(ctx context.Context, userID string) error {
	return s.rdb.Set(ctx, rtRevokedPrefix+userID, time.Now().UnixNano(), revocationKeyTTL).Err()
}
