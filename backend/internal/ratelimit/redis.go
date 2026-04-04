package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisLimiter implements a simple counter-based rate limiter backed by Redis.
type RedisLimiter struct {
	client *redis.Client
}

// NewRedisLimiter creates a RedisLimiter backed by the given Redis client.
func NewRedisLimiter(client *redis.Client) *RedisLimiter {
	return &RedisLimiter{client: client}
}

// Allow increments the counter for key and returns true when the call is within
// the limit for the given window. The first increment also sets the TTL so the
// key expires automatically after the window elapses.
func (r *RedisLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	count, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("rate limit incr: %w", err)
	}
	if count == 1 {
		r.client.Expire(ctx, key, window) //nolint:errcheck
	}
	return count <= int64(limit), nil
}

// IsLocked returns true when the counter for key has reached or exceeded limit.
// It does not increment the counter. Returns false if the key does not exist.
func (r *RedisLimiter) IsLocked(ctx context.Context, key string, limit int) (bool, error) {
	count, err := r.client.Get(ctx, key).Int64()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("rate limit get: %w", err)
	}
	return count >= int64(limit), nil
}

// RecordFailure increments the failure counter for key and returns true if the
// counter has now reached or exceeded limit. The window TTL is set on the first
// increment only, so the lockout period is anchored to the first failure.
func (r *RedisLimiter) RecordFailure(ctx context.Context, key string, limit int, window time.Duration) (locked bool, err error) {
	count, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("rate limit incr: %w", err)
	}
	if count == 1 {
		r.client.Expire(ctx, key, window) //nolint:errcheck
	}
	return count >= int64(limit), nil
}

// Reset deletes the counter for key, clearing any recorded failures.
func (r *RedisLimiter) Reset(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}
