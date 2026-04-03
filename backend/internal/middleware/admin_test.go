package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/token"
)

func TestRequireAdmin_AllowsAdmin(t *testing.T) {
	// Generate a token with isAdmin=true.
	tok, err := token.Generate("admin-user", true, testSecret, time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	handler := middleware.RequireAuth(testSecret)(middleware.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRequireAdmin_RejectsNonAdmin(t *testing.T) {
	// Generate a token with isAdmin=false.
	tok, err := token.Generate("regular-user", false, testSecret, time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	handler := middleware.RequireAuth(testSecret)(middleware.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestRequireAdmin_RejectsNoContext(t *testing.T) {
	// Call RequireAdmin directly without RequireAuth in the chain.
	handler := middleware.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}
