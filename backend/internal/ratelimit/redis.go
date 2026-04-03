package ratelimit

import (
	"context"
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
