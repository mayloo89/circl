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
	user        *auth.User
	loginErr    error
	registerErr error
}

func (m *mockAuth) Login(_ context.Context, _, _ string) (*auth.User, error) {
	return m.user, m.loginErr
}

func (m *mockAuth) Register(_ context.Context, _, _ string) (*auth.User, error) {
	return m.user, m.registerErr
}

// --- Login handler ---

func TestLoginHandler_Success(t *testing.T) {
	h := auth.NewHandler(&mockAuth{user: &auth.User{ID: "abc-123", Email: "user@example.com"}})

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"user@example.com","password":"secret"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	assertJSONField(t, rec.Body.Bytes(), "id", "abc-123")
	assertJSONField(t, rec.Body.Bytes(), "email", "user@example.com")
}

func TestLoginHandler_InvalidCredentials(t *testing.T) {
	h := auth.NewHandler(&mockAuth{loginErr: auth.ErrInvalidCredentials})

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"user@example.com","password":"wrong"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLoginHandler_InternalError(t *testing.T) {
	h := auth.NewHandler(&mockAuth{loginErr: errors.New("unexpected db error")})

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"user@example.com","password":"secret"}`))
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

func TestLoginHandler_MissingFields(t *testing.T) {
	h := auth.NewHandler(&mockAuth{})

	for _, body := range []string{
		`{"email":"","password":"secret"}`,
		`{"email":"user@example.com","password":""}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %s: status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
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
	h := auth.NewHandler(&mockAuth{user: &auth.User{ID: "1", Email: "u@u.com"}})

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"u@u.com","password":"pass1234"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

// --- Register handler ---

func TestRegisterHandler_Success(t *testing.T) {
	h := auth.NewHandler(&mockAuth{user: &auth.User{ID: "new-uuid", Email: "new@example.com"}})

	req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"email":"new@example.com","password":"securepass"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	assertJSONField(t, rec.Body.Bytes(), "email", "new@example.com")
}

func TestRegisterHandler_EmailTaken(t *testing.T) {
	h := auth.NewHandler(&mockAuth{registerErr: auth.ErrEmailTaken})

	req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"email":"taken@example.com","password":"securepass"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestRegisterHandler_InvalidInput(t *testing.T) {
	h := auth.NewHandler(&mockAuth{registerErr: auth.ErrInvalidInput})

	req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"email":"bad","password":"short"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRegisterHandler_InternalError(t *testing.T) {
	h := auth.NewHandler(&mockAuth{registerErr: errors.New("unexpected error")})

	req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"email":"user@example.com","password":"securepass"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestRegisterHandler_MalformedJSON(t *testing.T) {
	h := auth.NewHandler(&mockAuth{})

	req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader("{not json"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRegisterHandler_MissingFields(t *testing.T) {
	h := auth.NewHandler(&mockAuth{})

	for _, body := range []string{
		`{"email":"","password":"securepass"}`,
		`{"email":"user@example.com","password":""}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %s: status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}

// --- helpers ---

func assertJSONField(t *testing.T, body []byte, key, want string) {
	t.Helper()
	var m map[string]string
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if m[key] != want {
		t.Errorf("%s = %q, want %q", key, m[key], want)
	}
}
