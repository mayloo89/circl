package middleware_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mayloo89/circl/backend/internal/middleware"
)

type stubLimiter struct {
	allowed bool
	err     error
	key     string
	limit   int
	window  time.Duration
}

func (s *stubLimiter) Allow(_ context.Context, key string, limit int, window time.Duration) (bool, error) {
	s.key, s.limit, s.window = key, limit, window
	return s.allowed, s.err
}

func serveRateLimited(t *testing.T, limiter *stubLimiter) *httptest.ResponseRecorder {
	t.Helper()
	handler := middleware.RateLimit(limiter, 300, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/contacts", nil)
	req.RemoteAddr = "203.0.113.7:51234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestRateLimit_AllowedPassesThrough(t *testing.T) {
	limiter := &stubLimiter{allowed: true}
	rec := serveRateLimited(t, limiter)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if limiter.key != "global:ip:203.0.113.7" {
		t.Errorf("limiter key = %q, want %q", limiter.key, "global:ip:203.0.113.7")
	}
	if limiter.limit != 300 || limiter.window != time.Minute {
		t.Errorf("limiter called with limit=%d window=%v, want 300 / 1m", limiter.limit, limiter.window)
	}
}

func TestRateLimit_DeniedReturns429(t *testing.T) {
	rec := serveRateLimited(t, &stubLimiter{allowed: false})

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "60" {
		t.Errorf("Retry-After = %q, want %q", got, "60")
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != "rate_limited" {
		t.Errorf("error code = %q, want %q", body.Code, "rate_limited")
	}
}

func TestRateLimit_LimiterErrorFailsOpen(t *testing.T) {
	rec := serveRateLimited(t, &stubLimiter{err: errors.New("redis down")})

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (fail-open)", rec.Code)
	}
}
