package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// mockAuthInternal is an internal test double for the Authenticator interface.
type mockAuthInternal struct {
	user        *User
	loginErr    error
	registerErr error
}

func (m *mockAuthInternal) Login(_ context.Context, _, _ string) (*User, error) {
	return m.user, m.loginErr
}

func (m *mockAuthInternal) Register(_ context.Context, _, _ string) (*User, error) {
	return m.user, m.registerErr
}

// TestLoginHandler_TokenGenerateError covers the defensive error path when
// the token generator fails (e.g. signing infrastructure is unavailable).
func TestLoginHandler_TokenGenerateError(t *testing.T) {
	orig := generateTokenFn
	t.Cleanup(func() { generateTokenFn = orig })
	generateTokenFn = func(_, _ string, _ time.Duration) (string, error) {
		return "", errors.New("sign error")
	}

	h := NewHandler(&mockAuthInternal{user: &User{ID: "1", Email: "u@u.com"}}, "secret", time.Hour)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"u@u.com","password":"pass"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// TestRegisterHandler_TokenGenerateError covers the same defensive path for register.
func TestRegisterHandler_TokenGenerateError(t *testing.T) {
	orig := generateTokenFn
	t.Cleanup(func() { generateTokenFn = orig })
	generateTokenFn = func(_, _ string, _ time.Duration) (string, error) {
		return "", errors.New("sign error")
	}

	h := NewHandler(&mockAuthInternal{user: &User{ID: "1", Email: "u@u.com"}}, "secret", time.Hour)
	req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"email":"u@u.com","password":"pass"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
