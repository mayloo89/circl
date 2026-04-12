package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/token"
)

// mockStatusChecker is a test double for middleware.UserStatusChecker.
type mockStatusChecker struct {
	active bool
	err    error
}

func (m *mockStatusChecker) IsActiveUser(_ context.Context, _ string) (bool, error) {
	return m.active, m.err
}

const testSecret = "supersecretfortesting-mustbe32chars!!"

func okHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "no user id", http.StatusInternalServerError)
		return
	}
	w.Write([]byte(userID)) //nolint:errcheck
}

func TestRequireAuth_ValidToken(t *testing.T) {
	tok, _ := token.Generate("user-123", token.RoleUser, testSecret, time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()

	middleware.RequireAuth(testSecret)(http.HandlerFunc(okHandler)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "user-123" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "user-123")
	}
}

func TestRequireAuth_MissingHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	middleware.RequireAuth(testSecret)(http.HandlerFunc(okHandler)).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuth_WrongScheme(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic sometoken")
	rec := httptest.NewRecorder()

	middleware.RequireAuth(testSecret)(http.HandlerFunc(okHandler)).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuth_InvalidToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not.a.valid.token")
	rec := httptest.NewRecorder()

	middleware.RequireAuth(testSecret)(http.HandlerFunc(okHandler)).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuth_ExpiredToken(t *testing.T) {
	tok, _ := token.Generate("user-123", token.RoleUser, testSecret, -time.Second)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()

	middleware.RequireAuth(testSecret)(http.HandlerFunc(okHandler)).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestUserIDFromContext_Missing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, ok := middleware.UserIDFromContext(req.Context())
	if ok {
		t.Error("expected false for missing user ID")
	}
}

func TestRequireAuth_StatusChecker_ActiveUser(t *testing.T) {
	tok, _ := token.Generate("user-123", token.RoleUser, testSecret, time.Hour)
	checker := &mockStatusChecker{active: true}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()

	middleware.RequireAuth(testSecret, checker)(http.HandlerFunc(okHandler)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRequireAuth_StatusChecker_SuspendedUser(t *testing.T) {
	tok, _ := token.Generate("user-123", token.RoleUser, testSecret, time.Hour)
	checker := &mockStatusChecker{active: false}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()

	middleware.RequireAuth(testSecret, checker)(http.HandlerFunc(okHandler)).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuth_StatusChecker_Error(t *testing.T) {
	tok, _ := token.Generate("user-123", token.RoleUser, testSecret, time.Hour)
	checker := &mockStatusChecker{active: false, err: errors.New("db down")}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()

	middleware.RequireAuth(testSecret, checker)(http.HandlerFunc(okHandler)).ServeHTTP(rec, req)

	// Errors are treated as unauthorized for safety.
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuth_NoChecker_OnlyJWTValidation(t *testing.T) {
	// Without a checker, only JWT validation is performed.
	tok, _ := token.Generate("user-123", token.RoleUser, testSecret, time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()

	middleware.RequireAuth(testSecret)(http.HandlerFunc(okHandler)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestIsAdminFromContext_True(t *testing.T) {
	tok, _ := token.Generate("admin-user", token.RoleAdmin, testSecret, time.Hour)

	var gotAdmin bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAdmin = middleware.IsAdminFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()

	middleware.RequireAuth(testSecret)(handler).ServeHTTP(rec, req)

	if !gotAdmin {
		t.Error("expected IsAdminFromContext to return true for admin token")
	}
}

func TestIsAdminFromContext_False(t *testing.T) {
	tok, _ := token.Generate("regular-user", token.RoleUser, testSecret, time.Hour)

	var gotAdmin bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAdmin = middleware.IsAdminFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()

	middleware.RequireAuth(testSecret)(handler).ServeHTTP(rec, req)

	if gotAdmin {
		t.Error("expected IsAdminFromContext to return false for non-admin token")
	}
}
