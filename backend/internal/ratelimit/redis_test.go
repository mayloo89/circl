package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/mayloo89/circl/backend/internal/ratelimit"
)

func newTestLimiter(t *testing.T) (*ratelimit.RedisLimiter, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	return ratelimit.NewRedisLimiter(client), mr
}

func TestAllow_FirstCallReturnsTrue(t *testing.T) {
	limiter, _ := newTestLimiter(t)

	allowed, err := limiter.Allow(context.Background(), "test-key", 5, time.Minute)
	if err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if !allowed {
		t.Error("expected allowed=true on first call")
	}
}

func TestAllow_WithinLimit(t *testing.T) {
	limiter, _ := newTestLimiter(t)
	ctx := context.Background()
	const limit = 3

	for i := 1; i <= limit; i++ {
		allowed, err := limiter.Allow(ctx, "test-key", limit, time.Minute)
		if err != nil {
			t.Fatalf("call %d: Allow() error = %v", i, err)
		}
		if !allowed {
			t.Errorf("call %d: expected allowed=true, got false", i)
		}
	}
}

func TestAllow_ExceedsLimit(t *testing.T) {
	limiter, _ := newTestLimiter(t)
	ctx := context.Background()
	const limit = 3

	// Exhaust the limit.
	for i := 0; i < limit; i++ {
		limiter.Allow(ctx, "test-key", limit, time.Minute) //nolint:errcheck
	}

	// The next call should be denied.
	allowed, err := limiter.Allow(ctx, "test-key", limit, time.Minute)
	if err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if allowed {
		t.Error("expected allowed=false after limit exceeded")
	}
}

func TestAllow_ErrorCase(t *testing.T) {
	limiter, mr := newTestLimiter(t)
	mr.Close() // close Redis to force an error

	_, err := limiter.Allow(context.Background(), "test-key", 5, time.Minute)
	if err == nil {
		t.Fatal("expected error when Redis is unreachable, got nil")
	}
}
