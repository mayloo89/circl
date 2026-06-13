package ratelimit_test

import (
	"testing"

	"github.com/mayloo89/circl/backend/internal/ratelimit"
)

func TestConcurrentLimiter_AcquireUpToMax(t *testing.T) {
	l := ratelimit.NewConcurrentLimiter(2)

	if !l.Acquire("ip") {
		t.Fatal("first Acquire should succeed")
	}
	if !l.Acquire("ip") {
		t.Fatal("second Acquire should succeed at max=2")
	}
	if l.Acquire("ip") {
		t.Error("third Acquire should be denied at max=2")
	}
}

func TestConcurrentLimiter_ReleaseFreesSlot(t *testing.T) {
	l := ratelimit.NewConcurrentLimiter(1)

	if !l.Acquire("ip") {
		t.Fatal("first Acquire should succeed")
	}
	if l.Acquire("ip") {
		t.Fatal("second Acquire should be denied at max=1")
	}

	l.Release("ip")
	if !l.Acquire("ip") {
		t.Error("Acquire should succeed again after Release")
	}
}

func TestConcurrentLimiter_KeysAreIndependent(t *testing.T) {
	l := ratelimit.NewConcurrentLimiter(1)

	if !l.Acquire("a") {
		t.Fatal("Acquire for key a should succeed")
	}
	if !l.Acquire("b") {
		t.Error("Acquire for key b should not be affected by key a")
	}
}

func TestConcurrentLimiter_FullReleaseForgetsKey(t *testing.T) {
	l := ratelimit.NewConcurrentLimiter(3)

	for range 3 {
		if !l.Acquire("ip") {
			t.Fatal("Acquire within max should succeed")
		}
	}
	for range 3 {
		l.Release("ip")
	}
	for range 3 {
		if !l.Acquire("ip") {
			t.Error("all slots should be available again after full release")
		}
	}
}
