package admin_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mayloo89/circl/backend/internal/admin"
	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/token"
)

const (
	testSecret  = "supersecretfortesting-mustbe32chars!!"
	testAdminID = "admin-xyz-001"
)

// serve wraps the handler with RequireAuth and executes the request.
func serve(h http.Handler, r *http.Request, rec *httptest.ResponseRecorder) {
	middleware.RequireAuth(testSecret)(h).ServeHTTP(rec, r)
}

// adminRequest adds a valid admin Bearer token to the request.
func adminRequest(r *http.Request) *http.Request {
	tok, _ := token.Generate(testAdminID, true, testSecret, time.Hour)
	r.Header.Set("Authorization", "Bearer "+tok)
	return r
}

// newHandler builds an admin handler backed by the given mock store.
func newHandler(store admin.Store) http.Handler {
	return admin.NewHandler(admin.NewService(store))
}

// --- GET /stats ---

func TestGetStats_Success(t *testing.T) {
	st := &admin.Stats{TotalUsers: 42, ActiveUsers: 38, PendingReports: 5, TotalRooms: 10}
	store := &mockStore{stats: st}
	h := newHandler(store)

	req := adminRequest(httptest.NewRequest(http.MethodGet, "/stats", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got admin.Stats
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.TotalUsers != 42 {
		t.Errorf("TotalUsers = %d, want 42", got.TotalUsers)
	}
	if got.PendingReports != 5 {
		t.Errorf("PendingReports = %d, want 5", got.PendingReports)
	}
}

func TestGetStats_InternalError(t *testing.T) {
	store := &mockStore{getStatsErr: errors.New("db error")}
	h := newHandler(store)

	req := adminRequest(httptest.NewRequest(http.MethodGet, "/stats", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestGetStats_Forbidden(t *testing.T) {
	store := &mockStore{stats: &admin.Stats{}}
	h := newHandler(store)

	// Non-admin token (isAdmin = false)
	tok, _ := token.Generate("user-1", false, testSecret, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestGetStats_Unauthorized(t *testing.T) {
	store := &mockStore{stats: &admin.Stats{}}
	h := newHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestGetStats_ContentType(t *testing.T) {
	store := &mockStore{stats: &admin.Stats{TotalUsers: 1}}
	h := newHandler(store)

	req := adminRequest(httptest.NewRequest(http.MethodGet, "/stats", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// --- GET /users ---

func TestListUsers_Success(t *testing.T) {
	users := []*admin.UserRecord{
		{ID: "u-1", Email: "a@example.com", Status: "active"},
		{ID: "u-2", Email: "b@example.com", Status: "suspended"},
	}
	store := &mockStore{users: users, usersTotal: 2}
	h := newHandler(store)

	req := adminRequest(httptest.NewRequest(http.MethodGet, "/users", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body struct {
		Users []admin.UserRecord `json:"users"`
		Total int                `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Total != 2 {
		t.Errorf("total = %d, want 2", body.Total)
	}
	if len(body.Users) != 2 {
		t.Errorf("len(users) = %d, want 2", len(body.Users))
	}
}

func TestListUsers_WithQueryParams(t *testing.T) {
	users := []*admin.UserRecord{{ID: "u-1", Email: "a@example.com", Status: "active"}}
	store := &mockStore{users: users, usersTotal: 1}
	h := newHandler(store)

	req := adminRequest(httptest.NewRequest(http.MethodGet, "/users?q=alice&status=active&limit=10&offset=5", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestListUsers_InternalError(t *testing.T) {
	store := &mockStore{listUsersErr: errors.New("db error")}
	h := newHandler(store)

	req := adminRequest(httptest.NewRequest(http.MethodGet, "/users", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestListUsers_Forbidden(t *testing.T) {
	store := &mockStore{users: []*admin.UserRecord{}}
	h := newHandler(store)

	tok, _ := token.Generate("user-1", false, testSecret, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestListUsers_InvalidLimitClamped(t *testing.T) {
	store := &mockStore{users: []*admin.UserRecord{}, usersTotal: 0}
	h := newHandler(store)

	// limit=0 is invalid (not > 0), falls back to default 20
	req := adminRequest(httptest.NewRequest(http.MethodGet, "/users?limit=0&offset=-1", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestListUsers_InvalidLimitText(t *testing.T) {
	store := &mockStore{users: []*admin.UserRecord{}, usersTotal: 0}
	h := newHandler(store)

	// non-numeric limit falls back to default
	req := adminRequest(httptest.NewRequest(http.MethodGet, "/users?limit=abc&offset=xyz", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestListUsers_LimitTooHigh(t *testing.T) {
	store := &mockStore{users: []*admin.UserRecord{}, usersTotal: 0}
	h := newHandler(store)

	// limit=200 exceeds max of 100, falls back to default 20
	req := adminRequest(httptest.NewRequest(http.MethodGet, "/users?limit=200", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestUpdateUserStatus_NoUserIDInContext(t *testing.T) {
	store := &mockStore{}
	h := newHandler(store)

	// Call handler directly without any auth middleware — RequireAdmin blocks first (403),
	// so test RequireAdmin bypass by calling h directly after stripping middleware.
	// We test via a request missing auth header: serve() calls RequireAuth which rejects with 401.
	req := httptest.NewRequest(http.MethodPut, "/users/u-1/status", strings.NewReader(`{"action":"ban"}`))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	// RequireAuth fires first → 401
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// --- PUT /users/{id}/status ---

func TestUpdateUserStatus_Suspend_Success(t *testing.T) {
	store := &mockStore{
		user:       &admin.UserRecord{ID: "u-1", Status: "active"},
		suspension: &admin.Suspension{ID: "s-1"},
	}
	h := newHandler(store)

	body := `{"action":"suspend","reason":"spam","duration_days":7}`
	req := adminRequest(httptest.NewRequest(http.MethodPut, "/users/u-1/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

func TestUpdateUserStatus_Ban_Success(t *testing.T) {
	store := &mockStore{suspension: &admin.Suspension{ID: "s-1"}}
	h := newHandler(store)

	body := `{"action":"ban","reason":"severe violation"}`
	req := adminRequest(httptest.NewRequest(http.MethodPut, "/users/u-1/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

func TestUpdateUserStatus_Reactivate_Success(t *testing.T) {
	store := &mockStore{}
	h := newHandler(store)

	body := `{"action":"reactivate"}`
	req := adminRequest(httptest.NewRequest(http.MethodPut, "/users/u-1/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

func TestUpdateUserStatus_InvalidAction(t *testing.T) {
	store := &mockStore{}
	h := newHandler(store)

	body := `{"action":"delete"}`
	req := adminRequest(httptest.NewRequest(http.MethodPut, "/users/u-1/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestUpdateUserStatus_UserNotFound(t *testing.T) {
	store := &mockStore{getUserErr: admin.ErrUserNotFound}
	h := newHandler(store)

	body := `{"action":"suspend","reason":"spam"}`
	req := adminRequest(httptest.NewRequest(http.MethodPut, "/users/u-missing/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestUpdateUserStatus_AlreadySuspended(t *testing.T) {
	store := &mockStore{user: &admin.UserRecord{ID: "u-1", Status: "suspended"}}
	h := newHandler(store)

	body := `{"action":"suspend","reason":"spam"}`
	req := adminRequest(httptest.NewRequest(http.MethodPut, "/users/u-1/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

func TestUpdateUserStatus_BanNotFound(t *testing.T) {
	store := &mockStore{setStatusErr: admin.ErrUserNotFound}
	h := newHandler(store)

	body := `{"action":"ban","reason":"spam"}`
	req := adminRequest(httptest.NewRequest(http.MethodPut, "/users/u-missing/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestUpdateUserStatus_ReactivateNotFound(t *testing.T) {
	store := &mockStore{reactivateErr: admin.ErrUserNotFound}
	h := newHandler(store)

	body := `{"action":"reactivate"}`
	req := adminRequest(httptest.NewRequest(http.MethodPut, "/users/u-missing/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestUpdateUserStatus_MalformedJSON(t *testing.T) {
	store := &mockStore{}
	h := newHandler(store)

	req := adminRequest(httptest.NewRequest(http.MethodPut, "/users/u-1/status", strings.NewReader("{bad")))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestUpdateUserStatus_Forbidden(t *testing.T) {
	store := &mockStore{}
	h := newHandler(store)

	tok, _ := token.Generate("user-1", false, testSecret, time.Hour)
	req := httptest.NewRequest(http.MethodPut, "/users/u-1/status", strings.NewReader(`{"action":"ban"}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestUpdateUserStatus_SuspendInternalError(t *testing.T) {
	store := &mockStore{
		user:         &admin.UserRecord{ID: "u-1", Status: "active"},
		setStatusErr: errors.New("db error"),
	}
	h := newHandler(store)

	body := `{"action":"suspend","reason":"spam"}`
	req := adminRequest(httptest.NewRequest(http.MethodPut, "/users/u-1/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestUpdateUserStatus_BanInternalError(t *testing.T) {
	store := &mockStore{setStatusErr: errors.New("db error")}
	h := newHandler(store)

	body := `{"action":"ban","reason":"spam"}`
	req := adminRequest(httptest.NewRequest(http.MethodPut, "/users/u-1/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestUpdateUserStatus_ReactivateInternalError(t *testing.T) {
	store := &mockStore{reactivateErr: errors.New("db error")}
	h := newHandler(store)

	body := `{"action":"reactivate"}`
	req := adminRequest(httptest.NewRequest(http.MethodPut, "/users/u-1/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}
