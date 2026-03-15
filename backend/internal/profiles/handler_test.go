package profiles_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/profiles"
	"github.com/mayloo89/circl/backend/internal/token"
)

const testSecret = "supersecretfortesting-mustbe32chars!!"

type mockProfileManager struct {
	profile    *profiles.Profile
	getErr     error
	updateErr  error
}

func (m *mockProfileManager) GetMyProfile(_ context.Context, userID string) (*profiles.Profile, error) {
	return m.profile, m.getErr
}

func (m *mockProfileManager) UpdateMyProfile(_ context.Context, _, _, _, _ string) (*profiles.Profile, error) {
	return m.profile, m.updateErr
}

// authedRequest creates a request with a valid Bearer token for userID.
func authedRequest(t *testing.T, method, path, body string) *http.Request {
	t.Helper()
	tok, err := token.Generate("user-123", testSecret, time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r.Header.Set("Authorization", "Bearer "+tok)
	// Inject user ID into context (simulating middleware)
	ctx := r.Context()
	r = r.WithContext(context.WithValue(ctx, struct{ name string }{"userID"}, "user-123"))
	return r
}

// serve wraps the handler with auth middleware and serves the request.
func serve(h http.Handler, r *http.Request, rec *httptest.ResponseRecorder) {
	middleware.RequireAuth(testSecret)(h).ServeHTTP(rec, r)
}

func TestGetMyProfile_OK(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{
		profile: &profiles.Profile{ID: "p1", UserID: "user-123", DisplayName: "Alice", Bio: "Hey"},
	})

	tok, _ := token.Generate("user-123", testSecret, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/profiles/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["display_name"] != "Alice" {
		t.Errorf("display_name = %q, want %q", resp["display_name"], "Alice")
	}
}

func TestGetMyProfile_Unauthorized(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := httptest.NewRequest(http.MethodGet, "/profiles/me", nil)
	rec := httptest.NewRecorder()
	middleware.RequireAuth(testSecret)(h).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGetMyProfile_ServiceError(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{getErr: errors.New("db error")})

	tok, _ := token.Generate("user-123", testSecret, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/profiles/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestUpdateMyProfile_OK(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{
		profile: &profiles.Profile{ID: "p1", UserID: "user-123", DisplayName: "Alice", Bio: "Updated"},
	})

	tok, _ := token.Generate("user-123", testSecret, time.Hour)
	req := httptest.NewRequest(http.MethodPut, "/profiles/me", strings.NewReader(`{"display_name":"Alice","bio":"Updated"}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestUpdateMyProfile_InvalidInput(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{updateErr: profiles.ErrInvalidInput})

	tok, _ := token.Generate("user-123", testSecret, time.Hour)
	req := httptest.NewRequest(http.MethodPut, "/profiles/me", strings.NewReader(`{"display_name":""}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdateMyProfile_MalformedJSON(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	tok, _ := token.Generate("user-123", testSecret, time.Hour)
	req := httptest.NewRequest(http.MethodPut, "/profiles/me", strings.NewReader("{bad"))
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdateMyProfile_ServiceError(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{updateErr: errors.New("db error")})

	tok, _ := token.Generate("user-123", testSecret, time.Hour)
	req := httptest.NewRequest(http.MethodPut, "/profiles/me", strings.NewReader(`{"display_name":"Alice"}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// TestGetMyProfile_NoUserIDInContext calls the handler directly (bypassing auth
// middleware) without a userID in the context, covering the defensive !ok branch.
func TestGetMyProfile_NoUserIDInContext(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := httptest.NewRequest(http.MethodGet, "/profiles/me", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// TestUpdateMyProfile_NoUserIDInContext covers the defensive !ok branch in updateMyProfile.
func TestUpdateMyProfile_NoUserIDInContext(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := httptest.NewRequest(http.MethodPut, "/profiles/me", strings.NewReader(`{"display_name":"Alice"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestUpdateMyProfile_Unauthorized(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := httptest.NewRequest(http.MethodPut, "/profiles/me", strings.NewReader(`{"display_name":"Alice"}`))
	rec := httptest.NewRecorder()
	middleware.RequireAuth(testSecret)(h).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
