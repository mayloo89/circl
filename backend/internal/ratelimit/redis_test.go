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
	for range limit {
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

// --- IsLocked ---

func TestIsLocked_NoKey(t *testing.T) {
	limiter, _ := newTestLimiter(t)

	locked, err := limiter.IsLocked(context.Background(), "absent-key", 5)
	if err != nil {
		t.Fatalf("IsLocked() error = %v", err)
	}
	if locked {
		t.Error("expected locked=false for a key that does not exist")
	}
}

func TestIsLocked_BelowLimit(t *testing.T) {
	limiter, _ := newTestLimiter(t)
	ctx := context.Background()

	// Record 4 failures against a limit of 5.
	for range 4 {
		limiter.RecordFailure(ctx, "test-key", 5, time.Minute) //nolint:errcheck
	}

	locked, err := limiter.IsLocked(ctx, "test-key", 5)
	if err != nil {
		t.Fatalf("IsLocked() error = %v", err)
	}
	if locked {
		t.Error("expected locked=false when below limit")
	}
}

func TestIsLocked_AtLimit(t *testing.T) {
	limiter, _ := newTestLimiter(t)
	ctx := context.Background()

	// Record 5 failures to reach the limit.
	for range 5 {
		limiter.RecordFailure(ctx, "test-key", 5, time.Minute) //nolint:errcheck
	}

	locked, err := limiter.IsLocked(ctx, "test-key", 5)
	if err != nil {
		t.Fatalf("IsLocked() error = %v", err)
	}
	if !locked {
		t.Error("expected locked=true when at limit")
	}
}

func TestIsLocked_ErrorCase(t *testing.T) {
	limiter, mr := newTestLimiter(t)
	mr.Close()

	// Pre-set a key so Get is actually called (not Nil path).
	_, err := limiter.IsLocked(context.Background(), "test-key", 5)
	if err == nil {
		t.Fatal("expected error when Redis is unreachable, got nil")
	}
}

// --- RecordFailure ---

func TestRecordFailure_NotLockedYet(t *testing.T) {
	limiter, _ := newTestLimiter(t)

	locked, err := limiter.RecordFailure(context.Background(), "test-key", 3, time.Minute)
	if err != nil {
		t.Fatalf("RecordFailure() error = %v", err)
	}
	if locked {
		t.Error("expected locked=false on first failure against limit=3")
	}
}

func TestRecordFailure_ReachesLimit(t *testing.T) {
	limiter, _ := newTestLimiter(t)
	ctx := context.Background()
	const limit = 3

	var locked bool
	var err error
	for range limit {
		locked, err = limiter.RecordFailure(ctx, "test-key", limit, time.Minute)
		if err != nil {
			t.Fatalf("RecordFailure() error = %v", err)
		}
	}
	if !locked {
		t.Error("expected locked=true on the final failure that reaches the limit")
	}
}

// --- Reset ---

func TestReset_ClearsCounter(t *testing.T) {
	limiter, _ := newTestLimiter(t)
	ctx := context.Background()

	// Record enough failures to lock.
	for range 5 {
		limiter.RecordFailure(ctx, "test-key", 5, time.Minute) //nolint:errcheck
	}

	if err := limiter.Reset(ctx, "test-key"); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}

	locked, err := limiter.IsLocked(ctx, "test-key", 5)
	if err != nil {
		t.Fatalf("IsLocked() after Reset error = %v", err)
	}
	if locked {
		t.Error("expected locked=false after Reset")
	}
}

func TestReset_NonExistentKey(t *testing.T) {
	limiter, _ := newTestLimiter(t)

	// Resetting a key that does not exist must not return an error.
	if err := limiter.Reset(context.Background(), "absent-key"); err != nil {
		t.Errorf("Reset() error = %v, want nil", err)
	}
}
