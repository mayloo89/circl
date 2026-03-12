package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mayloo89/circl/backend/internal/auth"
)

// mockAuth is a test double for Authenticator.
type mockAuth struct {
	user *auth.User
	err  error
}

func (m *mockAuth) Login(_ context.Context, _, _ string) (*auth.User, error) {
	return m.user, m.err
}

func TestLoginHandler_Success(t *testing.T) {
	h := auth.NewHandler(&mockAuth{user: &auth.User{ID: "abc-123", Email: "user@example.com"}})

	body := `{"email":"user@example.com","password":"secret"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp["id"] != "abc-123" {
		t.Errorf("id = %q, want %q", resp["id"], "abc-123")
	}
	if resp["email"] != "user@example.com" {
		t.Errorf("email = %q, want %q", resp["email"], "user@example.com")
	}
}

func TestLoginHandler_InvalidCredentials(t *testing.T) {
	h := auth.NewHandler(&mockAuth{err: auth.ErrInvalidCredentials})

	body := `{"email":"user@example.com","password":"wrong"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLoginHandler_InternalError(t *testing.T) {
	h := auth.NewHandler(&mockAuth{err: errors.New("unexpected db error")})

	body := `{"email":"user@example.com","password":"secret"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestLoginHandler_MalformedJSON(t *testing.T) {
	h := auth.NewHandler(&mockAuth{})

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader("{not json"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLoginHandler_MissingEmail(t *testing.T) {
	h := auth.NewHandler(&mockAuth{})

	body := `{"email":"","password":"secret"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLoginHandler_MissingPassword(t *testing.T) {
	h := auth.NewHandler(&mockAuth{})

	body := `{"email":"user@example.com","password":""}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLoginHandler_PasswordTooLong(t *testing.T) {
	h := auth.NewHandler(&mockAuth{})

	body, _ := json.Marshal(map[string]string{
		"email":    "user@example.com",
		"password": strings.Repeat("a", 129),
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLoginHandler_ContentType(t *testing.T) {
	h := auth.NewHandler(&mockAuth{user: &auth.User{ID: "1", Email: "user@example.com"}})

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"x@x.com","password":"pass"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}
