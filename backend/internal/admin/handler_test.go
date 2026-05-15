package admin_test

import (
	"context"
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
	tok, _ := token.Generate(testAdminID, token.RoleAdmin, testSecret, time.Hour)
	r.Header.Set("Authorization", "Bearer "+tok)
	return r
}

// superAdminRequest adds a valid super_admin Bearer token to the request.
func superAdminRequest(r *http.Request) *http.Request {
	tok, _ := token.Generate(testAdminID, token.RoleSuperAdmin, testSecret, time.Hour)
	r.Header.Set("Authorization", "Bearer "+tok)
	return r
}

// newHandler builds an admin handler backed by the given mock store.
func newHandler(store admin.Store) http.Handler {
	return admin.NewHandler(admin.NewService(store), nil)
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
	tok, _ := token.Generate("user-1", token.RoleUser, testSecret, time.Hour)
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

// TestListUsers_PresenceOverlay verifies the handler enriches each user
// row with online + last_seen_at from the injected presence lookup, and
// that users absent from the lookup map default to offline / no last-seen.
func TestListUsers_PresenceOverlay(t *testing.T) {
	now := time.Now().UTC()
	users := []*admin.UserRecord{
		{ID: "u-1", Email: "a@example.com", Status: "active"},
		{ID: "u-2", Email: "b@example.com", Status: "active"},
		{ID: "u-3", Email: "c@example.com", Status: "active"},
	}
	store := &mockStore{users: users, usersTotal: 3}
	lookup := admin.PresenceLookupFunc(func(_ context.Context, ids []string) (map[string]admin.UserPresence, error) {
		// Capture the ids the handler passed for assertion.
		if len(ids) != 3 {
			t.Errorf("lookup called with %d ids, want 3", len(ids))
		}
		return map[string]admin.UserPresence{
			"u-1": {Online: true, LastSeenAt: &now},
			"u-2": {Online: false, LastSeenAt: &now},
			// u-3 absent → handler should treat as offline / no last-seen.
		}, nil
	})

	h := admin.NewHandler(admin.NewService(store), lookup)
	req := adminRequest(httptest.NewRequest(http.MethodGet, "/users", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body struct {
		Users []struct {
			ID         string     `json:"id"`
			Online     bool       `json:"online"`
			LastSeenAt *time.Time `json:"last_seen_at"`
		} `json:"users"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(body.Users) != 3 {
		t.Fatalf("len(users) = %d, want 3", len(body.Users))
	}
	if !body.Users[0].Online || body.Users[0].LastSeenAt == nil {
		t.Errorf("u-1 should be online with last_seen, got %+v", body.Users[0])
	}
	if body.Users[1].Online || body.Users[1].LastSeenAt == nil {
		t.Errorf("u-2 should be offline with last_seen, got %+v", body.Users[1])
	}
	if body.Users[2].Online || body.Users[2].LastSeenAt != nil {
		t.Errorf("u-3 should be offline with no last_seen (absent from lookup), got %+v", body.Users[2])
	}
}

// TestListUsers_PresenceLookupError surfaces a 500 if the presence lookup
// fails — admins shouldn't see partial / stale presence silently.
func TestListUsers_PresenceLookupError(t *testing.T) {
	store := &mockStore{
		users:      []*admin.UserRecord{{ID: "u-1", Email: "a@example.com", Status: "active"}},
		usersTotal: 1,
	}
	lookup := admin.PresenceLookupFunc(func(_ context.Context, _ []string) (map[string]admin.UserPresence, error) {
		return nil, errors.New("redis down")
	})

	h := admin.NewHandler(admin.NewService(store), lookup)
	req := adminRequest(httptest.NewRequest(http.MethodGet, "/users", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

// TestListUsers_NilPresenceLookupSkipsEnrichment proves passing nil for
// the lookup is the legacy / no-presence path — users are returned with
// online=false, last_seen_at omitted, and no extra calls.
func TestListUsers_NilPresenceLookupSkipsEnrichment(t *testing.T) {
	store := &mockStore{
		users:      []*admin.UserRecord{{ID: "u-1", Email: "a@example.com", Status: "active"}},
		usersTotal: 1,
	}
	h := admin.NewHandler(admin.NewService(store), nil)
	req := adminRequest(httptest.NewRequest(http.MethodGet, "/users", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body struct {
		Users []struct {
			Online     bool       `json:"online"`
			LastSeenAt *time.Time `json:"last_seen_at"`
		} `json:"users"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Users[0].Online || body.Users[0].LastSeenAt != nil {
		t.Errorf("expected zero-value presence on nil lookup, got %+v", body.Users[0])
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

	tok, _ := token.Generate("user-1", token.RoleUser, testSecret, time.Hour)
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
	store := &mockStore{
		user:       &admin.UserRecord{ID: "u-1", Email: "u@example.com", Status: "active"},
		suspension: &admin.Suspension{ID: "s-1"},
	}
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

	tok, _ := token.Generate("user-1", token.RoleUser, testSecret, time.Hour)
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

// --- GET /channels ---

func TestListChannels_Success(t *testing.T) {
	channels := []admin.ChannelRecord{
		{ID: "ch-1", Name: "general", Description: "General chat"},
		{ID: "ch-2", Name: "random", Description: ""},
	}
	store := &mockStore{channels: channels}
	h := newHandler(store)

	req := adminRequest(httptest.NewRequest(http.MethodGet, "/channels", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []admin.ChannelRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(channels) = %d, want 2", len(got))
	}
}

func TestListChannels_Empty(t *testing.T) {
	store := &mockStore{channels: []admin.ChannelRecord{}}
	h := newHandler(store)

	req := adminRequest(httptest.NewRequest(http.MethodGet, "/channels", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestListChannels_InternalError(t *testing.T) {
	store := &mockStore{listChansErr: errors.New("db error")}
	h := newHandler(store)

	req := adminRequest(httptest.NewRequest(http.MethodGet, "/channels", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestListChannels_Unauthorized(t *testing.T) {
	store := &mockStore{}
	h := newHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/channels", nil)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// --- DELETE /channels/{id} ---

func TestDeleteChannel_Success(t *testing.T) {
	store := &mockStore{}
	h := newHandler(store)

	req := adminRequest(httptest.NewRequest(http.MethodDelete, "/channels/ch-1", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

func TestDeleteChannel_NotFound(t *testing.T) {
	store := &mockStore{deleteChansErr: admin.ErrChannelNotFound}
	h := newHandler(store)

	req := adminRequest(httptest.NewRequest(http.MethodDelete, "/channels/ch-missing", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestDeleteChannel_InternalError(t *testing.T) {
	store := &mockStore{deleteChansErr: errors.New("db error")}
	h := newHandler(store)

	req := adminRequest(httptest.NewRequest(http.MethodDelete, "/channels/ch-1", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestDeleteChannel_Forbidden(t *testing.T) {
	store := &mockStore{}
	h := newHandler(store)

	tok, _ := token.Generate("user-1", token.RoleUser, testSecret, time.Hour)
	req := httptest.NewRequest(http.MethodDelete, "/channels/ch-1", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

// --- POST /channels ---

func TestCreateChannel_Success(t *testing.T) {
	ch := &admin.ChannelRecord{ID: "ch-new", Name: "general", Description: "General chat"}
	store := &mockStore{createdChannel: ch}
	h := newHandler(store)

	body := `{"name":"general","description":"General chat"}`
	req := adminRequest(httptest.NewRequest(http.MethodPost, "/channels", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	var got admin.ChannelRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.ID != "ch-new" {
		t.Errorf("ID = %q, want ch-new", got.ID)
	}
}

func TestCreateChannel_MissingName(t *testing.T) {
	store := &mockStore{}
	h := newHandler(store)

	body := `{"description":"no name here"}`
	req := adminRequest(httptest.NewRequest(http.MethodPost, "/channels", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestCreateChannel_NameTaken(t *testing.T) {
	store := &mockStore{createChanErr: admin.ErrChannelNameTaken}
	h := newHandler(store)

	body := `{"name":"general"}`
	req := adminRequest(httptest.NewRequest(http.MethodPost, "/channels", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

func TestCreateChannel_InternalError(t *testing.T) {
	store := &mockStore{createChanErr: errors.New("db error")}
	h := newHandler(store)

	body := `{"name":"general"}`
	req := adminRequest(httptest.NewRequest(http.MethodPost, "/channels", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestCreateChannel_MalformedJSON(t *testing.T) {
	store := &mockStore{}
	h := newHandler(store)

	req := adminRequest(httptest.NewRequest(http.MethodPost, "/channels", strings.NewReader("{bad")))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestCreateChannel_Forbidden(t *testing.T) {
	store := &mockStore{}
	h := newHandler(store)

	tok, _ := token.Generate("user-1", token.RoleUser, testSecret, time.Hour)
	req := httptest.NewRequest(http.MethodPost, "/channels", strings.NewReader(`{"name":"x"}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

// --- PUT /channels/{id} ---

func TestUpdateChannel_Success(t *testing.T) {
	store := &mockStore{}
	h := newHandler(store)

	body := `{"name":"updated","description":"new desc"}`
	req := adminRequest(httptest.NewRequest(http.MethodPut, "/channels/ch-1", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

func TestUpdateChannel_MissingName(t *testing.T) {
	store := &mockStore{}
	h := newHandler(store)

	body := `{"description":"no name"}`
	req := adminRequest(httptest.NewRequest(http.MethodPut, "/channels/ch-1", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestUpdateChannel_NotFound(t *testing.T) {
	store := &mockStore{updateChanErr: admin.ErrChannelNotFound}
	h := newHandler(store)

	body := `{"name":"x"}`
	req := adminRequest(httptest.NewRequest(http.MethodPut, "/channels/ch-missing", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestUpdateChannel_NameTaken(t *testing.T) {
	store := &mockStore{updateChanErr: admin.ErrChannelNameTaken}
	h := newHandler(store)

	body := `{"name":"taken"}`
	req := adminRequest(httptest.NewRequest(http.MethodPut, "/channels/ch-1", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

func TestUpdateChannel_InternalError(t *testing.T) {
	store := &mockStore{updateChanErr: errors.New("db error")}
	h := newHandler(store)

	body := `{"name":"x"}`
	req := adminRequest(httptest.NewRequest(http.MethodPut, "/channels/ch-1", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestUpdateChannel_MalformedJSON(t *testing.T) {
	store := &mockStore{}
	h := newHandler(store)

	req := adminRequest(httptest.NewRequest(http.MethodPut, "/channels/ch-1", strings.NewReader("{bad")))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestUpdateChannel_Forbidden(t *testing.T) {
	store := &mockStore{}
	h := newHandler(store)

	tok, _ := token.Generate("user-1", token.RoleUser, testSecret, time.Hour)
	req := httptest.NewRequest(http.MethodPut, "/channels/ch-1", strings.NewReader(`{"name":"x"}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

// --- DELETE /users/{id} ---

func TestHardDeleteUser_Success(t *testing.T) {
	store := &mockStore{}
	h := newHandler(store)

	req := superAdminRequest(httptest.NewRequest(http.MethodDelete, "/users/u-1", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

func TestHardDeleteUser_NotFound(t *testing.T) {
	store := &mockStore{hardDeleteErr: admin.ErrUserNotFound}
	h := newHandler(store)

	req := superAdminRequest(httptest.NewRequest(http.MethodDelete, "/users/u-missing", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestHardDeleteUser_InternalError(t *testing.T) {
	store := &mockStore{hardDeleteErr: errors.New("db error")}
	h := newHandler(store)

	req := superAdminRequest(httptest.NewRequest(http.MethodDelete, "/users/u-1", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

// Regular admin cannot hard-delete (only super_admin can).
func TestHardDeleteUser_ForbiddenForAdmin(t *testing.T) {
	store := &mockStore{}
	h := newHandler(store)

	req := adminRequest(httptest.NewRequest(http.MethodDelete, "/users/u-1", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestHardDeleteUser_Unauthorized(t *testing.T) {
	store := &mockStore{}
	h := newHandler(store)

	req := httptest.NewRequest(http.MethodDelete, "/users/u-1", nil)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// --- PUT /users/{id}/role ---

func TestSetUserRole_Success(t *testing.T) {
	for _, role := range []string{"user", "admin", "super_admin"} {
		store := &mockStore{}
		h := newHandler(store)

		body := `{"role":"` + role + `"}`
		req := superAdminRequest(httptest.NewRequest(http.MethodPut, "/users/u-1/role", strings.NewReader(body)))
		rec := httptest.NewRecorder()
		serve(h, req, rec)

		if rec.Code != http.StatusNoContent {
			t.Errorf("role=%q: status = %d, want 204", role, rec.Code)
		}
	}
}

func TestSetUserRole_InvalidRole(t *testing.T) {
	store := &mockStore{}
	h := newHandler(store)

	body := `{"role":"owner"}`
	req := superAdminRequest(httptest.NewRequest(http.MethodPut, "/users/u-1/role", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestSetUserRole_NotFound(t *testing.T) {
	store := &mockStore{setRoleErr: admin.ErrUserNotFound}
	h := newHandler(store)

	body := `{"role":"user"}`
	req := superAdminRequest(httptest.NewRequest(http.MethodPut, "/users/u-missing/role", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestSetUserRole_InternalError(t *testing.T) {
	store := &mockStore{setRoleErr: errors.New("db error")}
	h := newHandler(store)

	body := `{"role":"admin"}`
	req := superAdminRequest(httptest.NewRequest(http.MethodPut, "/users/u-1/role", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestSetUserRole_MalformedJSON(t *testing.T) {
	store := &mockStore{}
	h := newHandler(store)

	req := superAdminRequest(httptest.NewRequest(http.MethodPut, "/users/u-1/role", strings.NewReader("{bad")))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// Regular admin cannot set role (only super_admin can).
func TestSetUserRole_ForbiddenForAdmin(t *testing.T) {
	store := &mockStore{}
	h := newHandler(store)

	body := `{"role":"user"}`
	req := adminRequest(httptest.NewRequest(http.MethodPut, "/users/u-1/role", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestSetUserRole_Unauthorized(t *testing.T) {
	store := &mockStore{}
	h := newHandler(store)

	req := httptest.NewRequest(http.MethodPut, "/users/u-1/role", strings.NewReader(`{"role":"admin"}`))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}
