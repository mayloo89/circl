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
	profile           *profiles.Profile
	photo             *profiles.ProfilePhoto
	prefs             *profiles.ProfilePreferences
	interests         []profiles.InterestSuggestion
	browsePage        *profiles.BrowsePage
	browseErr         error
	usernameAvailable bool
	getErr            error
	updateErr         error
	updateAvatarErr   error
	publicErr         error
	addPhotoErr       error
	deletePhotoErr    error
	getPrefsErr       error
	updatePrefsErr    error
	searchIntErr      error
	usernameAvailErr  error
}

func (m *mockProfileManager) GetMyProfile(_ context.Context, userID string) (*profiles.Profile, error) {
	return m.profile, m.getErr
}

func (m *mockProfileManager) UpdateMyProfile(_ context.Context, _ string, _ profiles.ProfileInput) (*profiles.Profile, error) {
	return m.profile, m.updateErr
}

func (m *mockProfileManager) GetPublicProfile(_ context.Context, _ string) (*profiles.Profile, error) {
	return m.profile, m.publicErr
}

func (m *mockProfileManager) GetPublicProfileByUsername(_ context.Context, _ string) (*profiles.Profile, error) {
	return m.profile, m.publicErr
}

func (m *mockProfileManager) IsUsernameAvailable(_ context.Context, _ string) (bool, error) {
	return m.usernameAvailable, m.usernameAvailErr
}

func (m *mockProfileManager) AddPhoto(_ context.Context, _, _ string) (*profiles.ProfilePhoto, error) {
	return m.photo, m.addPhotoErr
}

func (m *mockProfileManager) DeletePhoto(_ context.Context, _, _ string) error {
	return m.deletePhotoErr
}

func (m *mockProfileManager) UpdateAvatar(_ context.Context, _, _ string) error {
	return m.updateAvatarErr
}

func (m *mockProfileManager) GetMyPreferences(_ context.Context, _ string) (*profiles.ProfilePreferences, error) {
	if m.getPrefsErr != nil {
		return nil, m.getPrefsErr
	}
	if m.prefs != nil {
		return m.prefs, nil
	}
	return &profiles.ProfilePreferences{GenderPreference: []string{}}, nil
}

func (m *mockProfileManager) UpdateMyPreferences(_ context.Context, _ string, prefs profiles.ProfilePreferences) (*profiles.ProfilePreferences, error) {
	if m.updatePrefsErr != nil {
		return nil, m.updatePrefsErr
	}
	if prefs.GenderPreference == nil {
		prefs.GenderPreference = []string{}
	}
	return &prefs, nil
}

func (m *mockProfileManager) SearchInterests(_ context.Context, _ string) ([]profiles.InterestSuggestion, error) {
	if m.searchIntErr != nil {
		return nil, m.searchIntErr
	}
	if m.interests != nil {
		return m.interests, nil
	}
	return []profiles.InterestSuggestion{}, nil
}

func (m *mockProfileManager) Browse(_ context.Context, _ string, _, _ int, _ bool, _ []string) (*profiles.BrowsePage, error) {
	if m.browseErr != nil {
		return nil, m.browseErr
	}
	if m.browsePage != nil {
		return m.browsePage, nil
	}
	return &profiles.BrowsePage{Profiles: []profiles.BrowseProfile{}, HasMore: false}, nil
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
		profile: &profiles.Profile{ID: "p1", UserID: "user-123", DisplayName: "Alice", Bio: "Hey", Interests: []string{}, Photos: []profiles.ProfilePhoto{}},
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
	if resp["interests"] == nil {
		t.Error("interests field missing from response")
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
		profile: &profiles.Profile{ID: "p1", UserID: "user-123", DisplayName: "Alice", Bio: "Updated", Interests: []string{}, Photos: []profiles.ProfilePhoto{}},
	})

	req := authedReq(t, http.MethodPut, "/profiles/me", `{"display_name":"Alice","bio":"Updated"}`)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestUpdateMyProfile_WithNewFields(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{
		profile: &profiles.Profile{ID: "p1", UserID: "user-123", DisplayName: "Alice", Gender: "female", Interests: []string{"music"}, Photos: []profiles.ProfilePhoto{}},
	})

	body := `{"display_name":"Alice","gender":"female","date_of_birth":"1995-06-15","interests":["music"]}`
	req := authedReq(t, http.MethodPut, "/profiles/me", body)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["gender"] != "female" {
		t.Errorf("gender = %q, want female", resp["gender"])
	}
}

func TestUpdateMyProfile_InvalidDOBFormat(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := authedReq(t, http.MethodPut, "/profiles/me", `{"display_name":"Alice","date_of_birth":"not-a-date"}`)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
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
			Interests: []string{"travel"},
			Photos:    []profiles.ProfilePhoto{{ID: "ph-1", URL: "https://example.com/1.jpg"}},
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

// --- GET /profiles/me/preferences ---

func TestGetMyPreferences_OK(t *testing.T) {
	minAge := 25
	maxAge := 40
	h := profiles.NewHandler(&mockProfileManager{
		prefs: &profiles.ProfilePreferences{MinAge: &minAge, MaxAge: &maxAge, GenderPreference: []string{"female"}},
	})

	req := authedReq(t, http.MethodGet, "/profiles/me/preferences", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["min_age"] != float64(25) {
		t.Errorf("min_age = %v, want 25", resp["min_age"])
	}
}

func TestGetMyPreferences_Unauthorized(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := httptest.NewRequest(http.MethodGet, "/profiles/me/preferences", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGetMyPreferences_ServiceError(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{getPrefsErr: errors.New("db error")})

	req := authedReq(t, http.MethodGet, "/profiles/me/preferences", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- PUT /profiles/me/preferences ---

func TestUpdateMyPreferences_OK(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	body := `{"min_age":20,"max_age":35,"max_distance_km":50,"gender_preference":["female"]}`
	req := authedReq(t, http.MethodPut, "/profiles/me/preferences", body)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["max_age"] != float64(35) {
		t.Errorf("max_age = %v, want 35", resp["max_age"])
	}
}

func TestUpdateMyPreferences_InvalidInput(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{updatePrefsErr: profiles.ErrInvalidInput})

	req := authedReq(t, http.MethodPut, "/profiles/me/preferences", `{"min_age":10}`)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdateMyPreferences_MalformedJSON(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := authedReq(t, http.MethodPut, "/profiles/me/preferences", "{bad")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdateMyPreferences_Unauthorized(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := httptest.NewRequest(http.MethodPut, "/profiles/me/preferences", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestUpdateMyPreferences_ServiceError(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{updatePrefsErr: errors.New("db error")})

	req := authedReq(t, http.MethodPut, "/profiles/me/preferences", `{}`)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- GET /profiles/interests ---

func TestSearchInterests_OK(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{
		interests: []profiles.InterestSuggestion{{Name: "hiking", Count: 10}, {Name: "history", Count: 3}},
	})

	req := authedReq(t, http.MethodGet, "/profiles/interests?q=hi", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var resp []map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp) != 2 {
		t.Errorf("len = %d, want 2", len(resp))
	}
	if resp[0]["name"] != "hiking" {
		t.Errorf("name = %q, want hiking", resp[0]["name"])
	}
	if resp[0]["count"] != float64(10) {
		t.Errorf("count = %v, want 10", resp[0]["count"])
	}
}

func TestSearchInterests_EmptyQuery(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := authedReq(t, http.MethodGet, "/profiles/interests", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestSearchInterests_Unauthorized(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := httptest.NewRequest(http.MethodGet, "/profiles/interests?q=hi", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestSearchInterests_ServiceError(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{searchIntErr: errors.New("db error")})

	req := authedReq(t, http.MethodGet, "/profiles/interests?q=hi", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- GET /profiles/available ---

func TestCheckUsernameAvailable_Available(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{usernameAvailable: true})

	req := authedReq(t, http.MethodGet, "/profiles/available?username=alice42", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["available"] != true {
		t.Errorf("available = %v, want true", resp["available"])
	}
}

func TestCheckUsernameAvailable_Taken(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{usernameAvailable: false})

	req := authedReq(t, http.MethodGet, "/profiles/available?username=alice42", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["available"] != false {
		t.Errorf("available = %v, want false", resp["available"])
	}
}

func TestCheckUsernameAvailable_MissingParam(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := authedReq(t, http.MethodGet, "/profiles/available", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCheckUsernameAvailable_Unauthorized(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := httptest.NewRequest(http.MethodGet, "/profiles/available?username=alice42", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCheckUsernameAvailable_ServiceError(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{usernameAvailErr: errors.New("db error")})

	req := authedReq(t, http.MethodGet, "/profiles/available?username=alice42", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- GET /profiles/@{username} ---

func TestGetPublicProfileByUsername_OK(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{
		profile: &profiles.Profile{ID: "p1", UserID: "u1", Username: "alice", DisplayName: "Alice", Interests: []string{}, Photos: []profiles.ProfilePhoto{}},
	})

	req := authedReq(t, http.MethodGet, "/profiles/alice", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["username"] != "alice" {
		t.Errorf("username = %v, want alice", resp["username"])
	}
}

func TestGetPublicProfileByRef_ByUUID(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{
		profile: &profiles.Profile{ID: "p1", UserID: "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", DisplayName: "Bob", Interests: []string{}, Photos: []profiles.ProfilePhoto{}},
	})

	req := authedReq(t, http.MethodGet, "/profiles/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestGetPublicProfileByUsername_NotFound(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{publicErr: profiles.ErrNotFound})

	req := authedReq(t, http.MethodGet, "/profiles/nobody", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetPublicProfileByUsername_Unauthorized(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := httptest.NewRequest(http.MethodGet, "/profiles/alice", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// --- GET /profiles/browse ---

func TestBrowseProfiles_OK(t *testing.T) {
	age := 28
	h := profiles.NewHandler(&mockProfileManager{
		browsePage: &profiles.BrowsePage{
			Profiles: []profiles.BrowseProfile{
				{ID: "p1", UserID: "u1", Username: "alice", DisplayName: "Alice", Age: &age, Interests: []string{"hiking"}},
			},
			HasMore: false,
		},
	})

	req := authedReq(t, http.MethodGet, "/profiles/browse", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Profiles []struct {
			ID       string `json:"id"`
			Username string `json:"username"`
			Age      *int   `json:"age"`
		} `json:"profiles"`
		HasMore bool `json:"has_more"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Profiles) != 1 {
		t.Fatalf("profiles len = %d, want 1", len(body.Profiles))
	}
	if body.Profiles[0].Username != "alice" {
		t.Errorf("username = %q, want alice", body.Profiles[0].Username)
	}
	if body.Profiles[0].Age == nil || *body.Profiles[0].Age != 28 {
		t.Errorf("age = %v, want 28", body.Profiles[0].Age)
	}
	if body.HasMore {
		t.Error("has_more should be false")
	}
}

func TestBrowseProfiles_PageAndLimit(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := authedReq(t, http.MethodGet, "/profiles/browse?page=2&limit=5", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Page  int `json:"page"`
		Limit int `json:"limit"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Page != 2 {
		t.Errorf("page = %d, want 2", body.Page)
	}
	if body.Limit != 5 {
		t.Errorf("limit = %d, want 5", body.Limit)
	}
}

func TestBrowseProfiles_Unauthorized(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})
	req := httptest.NewRequest(http.MethodGet, "/profiles/browse", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestBrowseProfiles_ServiceError(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{browseErr: errors.New("db error")})
	req := authedReq(t, http.MethodGet, "/profiles/browse", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

// --- PUT /profiles/me/avatar ---

func TestUpdateAvatar_OK(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := authedReq(t, http.MethodPut, "/profiles/me/avatar", `{"avatar_url":"https://example.com/avatar.jpg"}`)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestUpdateAvatar_Unauthorized(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := httptest.NewRequest(http.MethodPut, "/profiles/me/avatar", strings.NewReader(`{"avatar_url":"https://example.com/avatar.jpg"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestUpdateAvatar_MalformedJSON(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{})

	req := authedReq(t, http.MethodPut, "/profiles/me/avatar", "{bad")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// --- toResponse with DateOfBirth ---

func TestGetMyProfile_ResponseIncludesDateOfBirth(t *testing.T) {
	dob := time.Date(1995, 6, 15, 0, 0, 0, 0, time.UTC)
	h := profiles.NewHandler(&mockProfileManager{
		profile: &profiles.Profile{
			ID: "p1", UserID: "user-123", DisplayName: "Alice",
			DateOfBirth: &dob, Interests: []string{}, Photos: []profiles.ProfilePhoto{},
		},
	})

	req := authedReq(t, http.MethodGet, "/profiles/me", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["date_of_birth"] != "1995-06-15" {
		t.Errorf("date_of_birth = %v, want 1995-06-15", resp["date_of_birth"])
	}
}

// --- toPreferencesResponse with nil GenderPreference ---

func TestGetMyPreferences_NilGenderPreferenceBecomesEmptySlice(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{
		prefs: &profiles.ProfilePreferences{GenderPreference: nil},
	})

	req := authedReq(t, http.MethodGet, "/profiles/me/preferences", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	gp, ok := resp["gender_preference"]
	if !ok {
		t.Fatal("gender_preference field missing")
	}
	// Should be an empty array, not null
	arr, ok := gp.([]any)
	if !ok || len(arr) != 0 {
		t.Errorf("gender_preference = %v, want []", gp)
	}
}

// --- updateAvatar service error ---

func TestUpdateAvatar_ServiceError(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{updateAvatarErr: errors.New("db error")})

	req := authedReq(t, http.MethodPut, "/profiles/me/avatar", `{"avatar_url":"https://example.com/avatar.jpg"}`)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- updateMyProfile username taken conflict ---

func TestUpdateMyProfile_UsernameTaken(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{updateErr: profiles.ErrUsernameTaken})

	req := authedReq(t, http.MethodPut, "/profiles/me", `{"display_name":"Alice","username":"taken"}`)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

// --- isUUID edge cases via GET /profiles/{ref} ---

func TestGetPublicProfile_NonDashAtDashPosition(t *testing.T) {
	// 36 chars, valid hex elsewhere, but 'X' at position 8 instead of '-' → isUUID returns false
	h := profiles.NewHandler(&mockProfileManager{
		profile: &profiles.Profile{ID: "p1", UserID: "u1", DisplayName: "Bob", Interests: []string{}, Photos: []profiles.ProfilePhoto{}},
	})
	// a0eebc99X9c0b-4ef8-bb6d-6bb9bd380a11 is exactly 36 chars, dash expected at pos 8 but has 'X'
	req := authedReq(t, http.MethodGet, "/profiles/a0eebc99X9c0b-4ef8-bb6d-6bb9bd380a11", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)
	// Falls through to username lookup, succeeds with 200
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestGetPublicProfile_InvalidHexCharAt36Len(t *testing.T) {
	// 36 chars, valid dash positions, but 'z' at position 0 (invalid hex)
	h := profiles.NewHandler(&mockProfileManager{
		profile: &profiles.Profile{ID: "p1", UserID: "u1", DisplayName: "Bob", Interests: []string{}, Photos: []profiles.ProfilePhoto{}},
	})
	req := authedReq(t, http.MethodGet, "/profiles/z0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)
	// Falls through to username lookup, succeeds with 200
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// --- toResponse with nil interests ---

func TestGetMyProfile_NilInterestsBecomesEmptySlice(t *testing.T) {
	h := profiles.NewHandler(&mockProfileManager{
		profile: &profiles.Profile{ID: "p1", UserID: "user-123", DisplayName: "Alice", Interests: nil, Photos: []profiles.ProfilePhoto{}},
	})

	req := authedReq(t, http.MethodGet, "/profiles/me", "")
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	interests, ok := resp["interests"]
	if !ok {
		t.Fatal("interests field missing")
	}
	arr, ok := interests.([]any)
	if !ok || len(arr) != 0 {
		t.Errorf("interests = %v, want []", interests)
	}
}
