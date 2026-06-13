package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/server"
)

// mockPinger is a test double for DBPinger.
type mockPinger struct{ err error }

func (m *mockPinger) Ping(_ context.Context) error { return m.err }

// minimalConfig returns a Config with only the required fields set so tests
// can override the specific field under test without repeating boilerplate.
func minimalConfig(db server.DBPinger) server.Config {
	noop := func(h http.Handler) http.Handler { return h }
	return server.Config{
		DB:            db,
		Log:           zerolog.Nop(),
		Env:           "test",
		CORSOrigins:   []string{"http://localhost:3000"},
		RequireAuth:   noop,
		Auth:          http.NotFoundHandler(),
		Account:       http.NotFoundHandler(),
		Profile:       http.NotFoundHandler(),
		Available:     http.NotFoundHandler(),
		Contacts:      http.NotFoundHandler(),
		Notifications: http.NotFoundHandler(),
		Chat:          http.NotFoundHandler(),
		ChatWS:        http.NotFoundHandler(),
		Presence:      http.NotFoundHandler(),
		Upload:        http.NotFoundHandler(),
		Reports:       http.NotFoundHandler(),
		Push:          http.NotFoundHandler(),
		Admin:         http.NotFoundHandler(),
	}
}

func TestHealthHandler_DBOk(t *testing.T) {
	h := server.New(minimalConfig(&mockPinger{}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q, want %q", body["status"], "ok")
	}
	if body["env"] != "test" {
		t.Errorf("env = %q, want %q", body["env"], "test")
	}
	if body["db"] != "ok" {
		t.Errorf("db = %q, want %q", body["db"], "ok")
	}
}

func TestHealthHandler_DBError(t *testing.T) {
	cfg := minimalConfig(&mockPinger{err: errors.New("connection refused")})
	cfg.CORSOrigins = []string{"*"}
	h := server.New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if body["db"] != "error" {
		t.Errorf("db = %q, want %q", body["db"], "error")
	}
}

func TestHealthHandler_ContentType(t *testing.T) {
	cfg := minimalConfig(&mockPinger{})
	cfg.Env = "production"
	cfg.CORSOrigins = []string{"*"}
	h := server.New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

func TestHealthHandler_RedisOk(t *testing.T) {
	cfg := minimalConfig(&mockPinger{})
	cfg.RedisPing = func(_ context.Context) error { return nil }
	h := server.New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["redis"] != "ok" {
		t.Errorf("redis = %q, want %q", body["redis"], "ok")
	}
}

func TestHealthHandler_RedisError(t *testing.T) {
	cfg := minimalConfig(&mockPinger{})
	cfg.RedisPing = func(_ context.Context) error { return errors.New("dial refused") }
	h := server.New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["redis"] != "error" {
		t.Errorf("redis = %q, want %q", body["redis"], "error")
	}
}

func TestHealthHandler_RedisNil_DefaultsOk(t *testing.T) {
	// When RedisPing is nil the response must still contain a redis field.
	h := server.New(minimalConfig(&mockPinger{}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := body["redis"]; !ok {
		t.Error("expected redis field in health response even when RedisPing is nil")
	}
}

func TestHealthHandler_VersionField(t *testing.T) {
	cfg := minimalConfig(&mockPinger{})
	cfg.Version = "1.2.3"
	h := server.New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["version"] != "1.2.3" {
		t.Errorf("version = %q, want %q", body["version"], "1.2.3")
	}
}

func TestHealthHandler_VersionDefault(t *testing.T) {
	h := server.New(minimalConfig(&mockPinger{}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["version"] != "dev" {
		t.Errorf("version = %q, want %q", body["version"], "dev")
	}
}

func TestMetricsHandler_Mounted(t *testing.T) {
	cfg := minimalConfig(&mockPinger{})
	cfg.MetricsHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("# metrics")) //nolint:errcheck
	})
	h := server.New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestMetricsHandler_NotMountedWhenNil(t *testing.T) {
	h := server.New(minimalConfig(&mockPinger{}))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 when MetricsHandler is nil", rec.Code)
	}
}

func TestTracingMiddleware_Invoked(t *testing.T) {
	called := false
	cfg := minimalConfig(&mockPinger{})
	cfg.TracingMiddleware = func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			next.ServeHTTP(w, r)
		})
	}
	h := server.New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !called {
		t.Error("expected TracingMiddleware to be called")
	}
}

func TestTracingMiddleware_NilDoesNotPanic(t *testing.T) {
	h := server.New(minimalConfig(&mockPinger{}))
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req) // must not panic when TracingMiddleware is nil
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestNormalizeCORSOrigins(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "single origin",
			input: "http://localhost:3000",
			want:  []string{"http://localhost:3000"},
		},
		{
			name:  "multiple origins",
			input: "http://localhost:3000,https://example.com",
			want:  []string{"http://localhost:3000", "https://example.com"},
		},
		{
			name:  "trims whitespace",
			input: " http://localhost:3000 , https://example.com ",
			want:  []string{"http://localhost:3000", "https://example.com"},
		},
		{
			name:  "ignores empty entries",
			input: "http://localhost:3000,,https://example.com",
			want:  []string{"http://localhost:3000", "https://example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := server.NormalizeCORSOrigins(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v (len %d), want %v (len %d)", got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("origins[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestGlobalRateLimit_AppliesToAPIButNotHealth(t *testing.T) {
	cfg := minimalConfig(&mockPinger{})
	cfg.Auth = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	cfg.GlobalRateLimit = func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		})
	}
	h := server.New(cfg)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/auth/login", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("API route status = %d, want 429 (rate limit must wrap the API group)", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("/health status = %d, want 200 (health must stay outside the rate limit)", rec.Code)
	}
}
