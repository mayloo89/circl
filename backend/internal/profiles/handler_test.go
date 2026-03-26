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
	profile        *profiles.Profile
	photo          *profiles.ProfilePhoto
	getErr         error
	updateErr      error
	publicErr      error
	addPhotoErr    error
	deletePhotoErr error
}

func (m *mockProfileManager) GetMyProfile(_ context.Context, userID string) (*profiles.Profile, error) {
	return m.profile, m.getErr
}

func (m *mockProfileManager) UpdateMyProfile(_ context.Context, _, _, _, _ string) (*profiles.Profile, error) {
	return m.profile, m.updateErr
}

func (m *mockProfileManager) GetPublicProfile(_ context.Context, _ string) (*profiles.Profile, error) {
	return m.profile, m.publicErr
}

func (m *mockProfileManager) AddPhoto(_ context.Context, _, _ string) (*profiles.ProfilePhoto, error) {
	return m.photo, m.addPhotoErr
}

func (m *mockProfileManager) DeletePhoto(_ context.Context, _, _ string) error {
	return m.deletePhotoErr
}

func (m *mockProfileManager) UpdateAvatar(_ context.Context, _, _ string) error {
	return nil
}

// serve wraps the handler with auth middleware and serves the request.
func serve(h http.Handler, r *http.Request, rec *httptest.ResponseRecorder) {
	middleware.RequireAuth(testSecret)(h).ServeHTTP(rec, r)
}

func authedReq(t *testing.T, method, path, body string) *http.Request {
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
	return r
}

// --- GET /profiles/me ---

func TestGetMyProfile_OK(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{
		profile: &profiles.Profile{ID: "p1", UserID: "user-123", DisplayName: "Alice", Bio: "Hey", Photos: []profiles.ProfilePhoto{}},
	})

	req := authedReq(t, http.MethodGet, "/profiles/me", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["display_name"] != "Alice" {
		t.Errorf("display_name = %q, want Alice", resp["display_name"])
	}
	if resp["photos"] == nil {
		t.Error("photos field missing from response")
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

	req := authedReq(t, http.MethodGet, "/profiles/me", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestGetMyProfile_NoUserIDInContext(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := httptest.NewRequest(http.MethodGet, "/profiles/me", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// --- PUT /profiles/me ---

func TestUpdateMyProfile_OK(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{
		profile: &profiles.Profile{ID: "p1", UserID: "user-123", DisplayName: "Alice", Bio: "Updated", Photos: []profiles.ProfilePhoto{}},
	})

	req := authedReq(t, http.MethodPut, "/profiles/me", `{"display_name":"Alice","bio":"Updated"}`)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestUpdateMyProfile_InvalidInput(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{updateErr: profiles.ErrInvalidInput})

	req := authedReq(t, http.MethodPut, "/profiles/me", `{"display_name":""}`)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdateMyProfile_MalformedJSON(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := authedReq(t, http.MethodPut, "/profiles/me", "{bad")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdateMyProfile_ServiceError(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{updateErr: errors.New("db error")})

	req := authedReq(t, http.MethodPut, "/profiles/me", `{"display_name":"Alice"}`)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

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

// --- GET /profiles/{userID} ---

func TestGetPublicProfile_OK(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{
		profile: &profiles.Profile{
			ID: "p1", UserID: "user-456", DisplayName: "Bob", Bio: "Hello",
			Photos: []profiles.ProfilePhoto{{ID: "ph-1", URL: "https://example.com/1.jpg"}},
		},
	})

	req := authedReq(t, http.MethodGet, "/profiles/user-456", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["display_name"] != "Bob" {
		t.Errorf("display_name = %q, want Bob", resp["display_name"])
	}
	photos, _ := resp["photos"].([]any)
	if len(photos) != 1 {
		t.Errorf("photos len = %d, want 1", len(photos))
	}
}

func TestGetPublicProfile_NotFound(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{publicErr: profiles.ErrNotFound})

	req := authedReq(t, http.MethodGet, "/profiles/unknown-user", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetPublicProfile_Unauthorized(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := httptest.NewRequest(http.MethodGet, "/profiles/user-456", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGetPublicProfile_ServiceError(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{publicErr: errors.New("db error")})

	req := authedReq(t, http.MethodGet, "/profiles/user-456", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- POST /profiles/me/photos ---

func TestAddPhoto_Created(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{
		photo: &profiles.ProfilePhoto{ID: "ph-1", URL: "https://example.com/1.jpg"},
	})

	req := authedReq(t, http.MethodPost, "/profiles/me/photos", `{"url":"https://example.com/1.jpg"}`)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["id"] != "ph-1" {
		t.Errorf("id = %q, want ph-1", resp["id"])
	}
}

func TestAddPhoto_MaxReached(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{addPhotoErr: profiles.ErrInvalidInput})

	req := authedReq(t, http.MethodPost, "/profiles/me/photos", `{"url":"https://example.com/1.jpg"}`)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
}

func TestAddPhoto_MalformedJSON(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := authedReq(t, http.MethodPost, "/profiles/me/photos", "{bad")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAddPhoto_Unauthorized(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := httptest.NewRequest(http.MethodPost, "/profiles/me/photos", strings.NewReader(`{"url":"https://example.com/1.jpg"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAddPhoto_ServiceError(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{addPhotoErr: errors.New("db error")})

	req := authedReq(t, http.MethodPost, "/profiles/me/photos", `{"url":"https://example.com/1.jpg"}`)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- DELETE /profiles/me/photos/{photoID} ---

func TestDeletePhoto_NoContent(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := authedReq(t, http.MethodDelete, "/profiles/me/photos/ph-1", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestDeletePhoto_NotFound(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{deletePhotoErr: profiles.ErrPhotoNotFound})

	req := authedReq(t, http.MethodDelete, "/profiles/me/photos/ph-1", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestDeletePhoto_Unauthorized(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := httptest.NewRequest(http.MethodDelete, "/profiles/me/photos/ph-1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestDeletePhoto_ServiceError(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{deletePhotoErr: errors.New("db error")})

	req := authedReq(t, http.MethodDelete, "/profiles/me/photos/ph-1", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
