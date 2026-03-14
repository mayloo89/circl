package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/token"
)

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
	tok, _ := token.Generate("user-123", testSecret, time.Hour)

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
	tok, _ := token.Generate("user-123", testSecret, -time.Second)

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
