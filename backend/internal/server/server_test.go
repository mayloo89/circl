package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mayloo89/circl/backend/internal/server"
)

// mockPinger is a test double for DBPinger.
type mockPinger struct{ err error }

func (m *mockPinger) Ping(_ context.Context) error { return m.err }

func TestHealthHandler_DBOk(t *testing.T) {
	h := server.New(&mockPinger{}, "test", []string{"http://localhost:3000"}, http.NotFoundHandler())

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
	h := server.New(&mockPinger{err: errors.New("connection refused")}, "test", []string{"*"}, http.NotFoundHandler())

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
	h := server.New(&mockPinger{}, "production", []string{"*"}, http.NotFoundHandler())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
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
