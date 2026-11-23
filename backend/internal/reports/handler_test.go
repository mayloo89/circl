package reports_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/reports"
	"github.com/mayloo89/circl/backend/internal/token"
)

const adminUserID = "admin-user-xyz"

// mockModerator is a test double for reports.AdminModerator.
type mockModerator struct {
	suspendErr error
	banErr     error
	suspended  bool
	banned     bool
}

func (m *mockModerator) SuspendUser(_ context.Context, _, _ string, _ int, _ string) error {
	m.suspended = true
	return m.suspendErr
}

func (m *mockModerator) BanUser(_ context.Context, _, _, _ string) error {
	m.banned = true
	return m.banErr
}

// mockLimiter is a test double for reports.RateLimiter.
type mockLimiter struct {
	allowed bool
	err     error
}

func (m *mockLimiter) Allow(_ context.Context, _ string, _ int, _ time.Duration) (bool, error) {
	return m.allowed, m.err
}

// adminAuthedRequest adds a valid admin Bearer token to the request.
func adminAuthedRequest(r *http.Request) *http.Request {
	tok, _ := token.Generate(adminUserID, token.RoleAdmin, testSecret, time.Hour)
	r.Header.Set("Authorization", "Bearer "+tok)
	return r
}

const (
	testSecret = "supersecretfortesting-mustbe32chars!!"
	testUserID = "user-abc-123"
)

// mockHandlerStore is a test double for the reports.Store interface.
type mockHandlerStore struct {
	report      *reports.Report
	reportsList []reports.ReportWithUserInfo
	createErr   error
	listErr     error
	updateErr   error
	getByIDErr  error
	updated     bool
}

func (m *mockHandlerStore) Create(_ context.Context, _, _, _, _, _ string) (*reports.Report, error) {
	return m.report, m.createErr
}

func (m *mockHandlerStore) CreateForMessage(_ context.Context, _, _, _, _, _ string) (*reports.Report, error) {
	return m.report, m.createErr
}

func (m *mockHandlerStore) GetByID(_ context.Context, _ string) (*reports.Report, error) {
	return m.report, m.getByIDErr
}

func (m *mockHandlerStore) List(_ context.Context, _ reports.ListFilter) ([]reports.ReportWithUserInfo, error) {
	return m.reportsList, m.listErr
}

func (m *mockHandlerStore) UpdateStatus(_ context.Context, _, _, _ string) (*reports.Report, error) {
	m.updated = true
	return m.report, m.updateErr
}

// serve wraps the handler with RequireAuth and executes the request.
func serve(h http.Handler, r *http.Request, rec *httptest.ResponseRecorder) {
	middleware.RequireAuth(testSecret)(h).ServeHTTP(rec, r)
}

// authedRequest adds a valid Bearer token for testUserID to the request.
func authedRequest(r *http.Request) *http.Request {
	tok, _ := token.Generate(testUserID, token.RoleUser, testSecret, time.Hour)
	r.Header.Set("Authorization", "Bearer "+tok)
	return r
}

// --- CreateReport ---

func TestCreateReport_Success(t *testing.T) {
	now := time.Now()
	report := &reports.Report{
		ID:             "r-1",
		ReporterID:     new(testUserID),
		ReportedUserID: new("u-2"),
		Reason:         "harassment",
		Description:    "User sent offensive messages",
		Status:         "pending",
		CreatedAt:      now,
	}
	store := &mockHandlerStore{report: report}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	body := `{"reported_user_id":"u-2","reason":"harassment","description":"User sent offensive messages"}`
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	var got reports.Report
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.ID != "r-1" {
		t.Errorf("ID = %q, want r-1", got.ID)
	}
}

func TestCreateReport_Unauthorized(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"reported_user_id":"u-2","reason":"spam"}`))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCreateReport_MalformedJSON(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := authedRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{bad")))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateReport_MissingReportedUserID(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := authedRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"reported_user_id":"","reason":"spam"}`)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateReport_MissingReason(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := authedRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"reported_user_id":"u-2","reason":""}`)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateReport_SelfReport(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := authedRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"reported_user_id":"user-abc-123","reason":"spam"}`)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateReport_InvalidReason(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := authedRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"reported_user_id":"u-2","reason":"invalid_reason"}`)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateReport_InternalError(t *testing.T) {
	store := &mockHandlerStore{createErr: errors.New("db error")}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := authedRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"reported_user_id":"u-2","reason":"spam"}`)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestCreateReport_NoUserIDInContext(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"reported_user_id":"u-2","reason":"spam"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// --- Content-Type ---

func TestHandlers_ContentType(t *testing.T) {
	store := &mockHandlerStore{report: &reports.Report{ID: "r-1", ReporterID: new("u-1"), ReportedUserID: new("u-2"), Reason: "spam", Status: "pending"}}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	body := `{"reported_user_id":"u-2","reason":"spam"}`
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// --- ListReports (admin) ---

func TestListReports_AdminSuccess(t *testing.T) {
	items := []reports.ReportWithUserInfo{
		{ID: "r-1", ReporterID: new("u-1"), ReportedUserID: new("u-2"), Reason: "spam", Status: "pending"},
	}
	store := &mockHandlerStore{reportsList: items}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := adminAuthedRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestListReports_ForbiddenForNonAdmin(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestListReports_Unauthorized(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestListReports_InvalidStatus(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := adminAuthedRequest(httptest.NewRequest(http.MethodGet, "/?status=invalid", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestListReports_PriorityFilter(t *testing.T) {
	items := []reports.ReportWithUserInfo{
		{ID: "r-1", Reason: reports.ReasonNCII, Priority: reports.PriorityCritical, Status: "pending"},
	}
	store := &mockHandlerStore{reportsList: items}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := adminAuthedRequest(httptest.NewRequest(http.MethodGet, "/?priority=critical", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestListReports_InvalidPriority(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := adminAuthedRequest(httptest.NewRequest(http.MethodGet, "/?priority=panic", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestListReports_InternalError(t *testing.T) {
	store := &mockHandlerStore{listErr: errors.New("db error")}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := adminAuthedRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- UpdateReportStatus (admin) ---

func TestUpdateReportStatus_AdminSuccess(t *testing.T) {
	report := &reports.Report{
		ID:             "r-1",
		ReporterID:     new("u-1"),
		ReportedUserID: new("u-2"),
		Status:         "reviewed",
	}
	store := &mockHandlerStore{report: report}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)

	body := `{"status":"reviewed"}`
	req := adminAuthedRequest(httptest.NewRequest(http.MethodPut, "/r-1/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()

	// Route through chi to get URL params.
	router := chi.NewRouter()
	router.Put("/{id}/status", mgr.UpdateReportStatus)
	middleware.RequireAuth(testSecret)(middleware.RequireAdmin(router)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestUpdateReportStatus_ForbiddenForNonAdmin(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	body := `{"status":"reviewed"}`
	req := authedRequest(httptest.NewRequest(http.MethodPut, "/r-1/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestUpdateReportStatus_NotFound(t *testing.T) {
	store := &mockHandlerStore{updateErr: reports.ErrNotFound}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)

	body := `{"status":"reviewed"}`
	req := adminAuthedRequest(httptest.NewRequest(http.MethodPut, "/r-missing/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()

	router := chi.NewRouter()
	router.Put("/{id}/status", mgr.UpdateReportStatus)
	middleware.RequireAuth(testSecret)(middleware.RequireAdmin(router)).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestUpdateReportStatus_InvalidStatus(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)

	body := `{"status":"invalid"}`
	req := adminAuthedRequest(httptest.NewRequest(http.MethodPut, "/r-1/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()

	router := chi.NewRouter()
	router.Put("/{id}/status", mgr.UpdateReportStatus)
	middleware.RequireAuth(testSecret)(middleware.RequireAdmin(router)).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdateReportStatus_InvalidAction(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)

	body := `{"status":"reviewed","action":"delete"}`
	req := adminAuthedRequest(httptest.NewRequest(http.MethodPut, "/r-1/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()

	router := chi.NewRouter()
	router.Put("/{id}/status", mgr.UpdateReportStatus)
	middleware.RequireAuth(testSecret)(middleware.RequireAdmin(router)).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdateReportStatus_SuspendAction(t *testing.T) {
	report := &reports.Report{
		ID:             "r-1",
		ReporterID:     new("u-1"),
		ReportedUserID: new("u-2"),
		Status:         "reviewed",
	}
	store := &mockHandlerStore{report: report}
	svc := reports.NewService(store)
	mod := &mockModerator{}
	mgr := reports.NewManager(svc, reports.WithModerator(mod))

	body := `{"status":"reviewed","action":"suspend","duration_days":7,"reason":"spamming"}`
	req := adminAuthedRequest(httptest.NewRequest(http.MethodPut, "/r-1/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()

	router := chi.NewRouter()
	router.Put("/{id}/status", mgr.UpdateReportStatus)
	middleware.RequireAuth(testSecret)(middleware.RequireAdmin(router)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !mod.suspended {
		t.Error("expected SuspendUser to be called")
	}
}

func TestUpdateReportStatus_BanAction(t *testing.T) {
	report := &reports.Report{
		ID:             "r-1",
		ReporterID:     new("u-1"),
		ReportedUserID: new("u-2"),
		Status:         "reviewed",
	}
	store := &mockHandlerStore{report: report}
	svc := reports.NewService(store)
	mod := &mockModerator{}
	mgr := reports.NewManager(svc, reports.WithModerator(mod))

	body := `{"status":"reviewed","action":"ban","reason":"severe violation"}`
	req := adminAuthedRequest(httptest.NewRequest(http.MethodPut, "/r-1/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()

	router := chi.NewRouter()
	router.Put("/{id}/status", mgr.UpdateReportStatus)
	middleware.RequireAuth(testSecret)(middleware.RequireAdmin(router)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !mod.banned {
		t.Error("expected BanUser to be called")
	}
}

func TestUpdateReportStatus_NoModerator(t *testing.T) {
	report := &reports.Report{
		ID:             "r-1",
		ReporterID:     new("u-1"),
		ReportedUserID: new("u-2"),
		Status:         "reviewed",
	}
	store := &mockHandlerStore{report: report}
	svc := reports.NewService(store)
	// No moderator set.
	mgr := reports.NewManager(svc)

	body := `{"status":"reviewed","action":"suspend","reason":"spam"}`
	req := adminAuthedRequest(httptest.NewRequest(http.MethodPut, "/r-1/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()

	router := chi.NewRouter()
	router.Put("/{id}/status", mgr.UpdateReportStatus)
	middleware.RequireAuth(testSecret)(middleware.RequireAdmin(router)).ServeHTTP(rec, req)

	// Should still return 200 — no moderator means action is silently skipped.
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// --- Rate limiting ---

func TestCreateReport_RateLimited(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	limiter := &mockLimiter{allowed: false}
	mgr := reports.NewManager(svc, reports.WithLimiter(limiter))
	h := reports.NewHandler(mgr)

	body := `{"reported_user_id":"u-2","reason":"spam"}`
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestCreateReport_LimiterError_Continues(t *testing.T) {
	// When the limiter returns an error, the request should still be processed.
	report := &reports.Report{ID: "r-1", ReporterID: new(testUserID), ReportedUserID: new("u-2"), Reason: "spam", Status: "pending"}
	store := &mockHandlerStore{report: report}
	svc := reports.NewService(store)
	limiter := &mockLimiter{allowed: false, err: errors.New("redis down")}
	mgr := reports.NewManager(svc, reports.WithLimiter(limiter))
	h := reports.NewHandler(mgr)

	body := `{"reported_user_id":"u-2","reason":"spam"}`
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	// Limiter error is ignored; request goes through.
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestUpdateReportStatus_MissingStatus(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)

	body := `{"action":"suspend"}`
	req := adminAuthedRequest(httptest.NewRequest(http.MethodPut, "/r-1/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()

	router := chi.NewRouter()
	router.Put("/{id}/status", mgr.UpdateReportStatus)
	middleware.RequireAuth(testSecret)(middleware.RequireAdmin(router)).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdateReportStatus_MalformedJSON(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)

	req := adminAuthedRequest(httptest.NewRequest(http.MethodPut, "/r-1/status", strings.NewReader("{bad")))
	rec := httptest.NewRecorder()

	router := chi.NewRouter()
	router.Put("/{id}/status", mgr.UpdateReportStatus)
	middleware.RequireAuth(testSecret)(middleware.RequireAdmin(router)).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdateReportStatus_InternalError(t *testing.T) {
	store := &mockHandlerStore{updateErr: errors.New("db error")}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)

	body := `{"status":"reviewed"}`
	req := adminAuthedRequest(httptest.NewRequest(http.MethodPut, "/r-1/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()

	router := chi.NewRouter()
	router.Put("/{id}/status", mgr.UpdateReportStatus)
	middleware.RequireAuth(testSecret)(middleware.RequireAdmin(router)).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestUpdateReportStatus_NoUserIDInContext(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)

	// Call handler directly without RequireAuth so no user ID is in context.
	body := `{"status":"reviewed"}`
	req := httptest.NewRequest(http.MethodPut, "/r-1/status", strings.NewReader(body))
	rec := httptest.NewRecorder()

	router := chi.NewRouter()
	router.Put("/{id}/status", mgr.UpdateReportStatus)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// --- Message reports ---

func postReport(t *testing.T, store *mockHandlerStore, body string) *httptest.ResponseRecorder {
	t.Helper()
	h := reports.NewHandler(reports.NewManager(reports.NewService(store)))
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v (%s)", err, rec.Body.String())
	}
	return body.Code
}

func TestCreateReport_MessageSuccess(t *testing.T) {
	store := &mockHandlerStore{report: &reports.Report{ID: "r-9", MessageID: new("m-1"), ReportedUserID: new("u-2")}}
	rec := postReport(t, store, `{"message_id":"m-1","reason":"harassment"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var got reports.Report
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.MessageID == nil || *got.MessageID != "m-1" {
		t.Errorf("message_id = %v, want m-1", got.MessageID)
	}
}

func TestCreateReport_ErrorMapping(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		err      error
		wantCode int
		wantErr  string
	}{
		{"both targets", `{"reported_user_id":"u-2","message_id":"m-1","reason":"spam"}`, nil, http.StatusBadRequest, "invalid_request"},
		{"no target", `{"reason":"spam"}`, nil, http.StatusBadRequest, "invalid_request"},
		{"message not visible", `{"message_id":"m-1","reason":"spam"}`, reports.ErrMessageNotFound, http.StatusNotFound, "message_not_found"},
		{"duplicate", `{"message_id":"m-1","reason":"spam"}`, reports.ErrDuplicateReport, http.StatusConflict, "already_reported"},
		{"own message", `{"message_id":"m-1","reason":"spam"}`, reports.ErrSelfReport, http.StatusBadRequest, "invalid_request"},
		{"unknown user", `{"reported_user_id":"u-404","reason":"spam"}`, reports.ErrUserNotFound, http.StatusNotFound, "user_not_found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postReport(t, &mockHandlerStore{createErr: tc.err}, tc.body)
			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tc.wantCode, rec.Body.String())
			}
			if got := errorCode(t, rec); got != tc.wantErr {
				t.Errorf("error code = %q, want %q", got, tc.wantErr)
			}
		})
	}
}

func TestUpdateReportStatus_ActionOnDeletedAccountIsRejectedBeforeStatusChange(t *testing.T) {
	store := &mockHandlerStore{report: &reports.Report{ID: "r-1", Status: "pending"}}
	mod := &mockModerator{}
	mgr := reports.NewManager(reports.NewService(store), reports.WithModerator(mod))

	body := `{"status":"reviewed","action":"ban","reason":"csam"}`
	req := adminAuthedRequest(httptest.NewRequest(http.MethodPut, "/r-1/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Put("/{id}/status", mgr.UpdateReportStatus)
	middleware.RequireAuth(testSecret)(middleware.RequireAdmin(router)).ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	if got := errorCode(t, rec); got != "reported_account_gone" {
		t.Errorf("error code = %q, want reported_account_gone", got)
	}
	if store.updated || mod.banned {
		t.Error("status must not change and no action may run when the account is gone")
	}
}

func TestUpdateReportStatus_ActionOnMissingReport(t *testing.T) {
	store := &mockHandlerStore{getByIDErr: reports.ErrNotFound}
	mgr := reports.NewManager(reports.NewService(store), reports.WithModerator(&mockModerator{}))

	req := adminAuthedRequest(httptest.NewRequest(http.MethodPut, "/r-x/status", strings.NewReader(`{"status":"reviewed","action":"suspend"}`)))
	rec := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Put("/{id}/status", mgr.UpdateReportStatus)
	middleware.RequireAuth(testSecret)(middleware.RequireAdmin(router)).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestUpdateReportStatus_ActionLookupError(t *testing.T) {
	store := &mockHandlerStore{getByIDErr: errors.New("db down")}
	mgr := reports.NewManager(reports.NewService(store), reports.WithModerator(&mockModerator{}))

	req := adminAuthedRequest(httptest.NewRequest(http.MethodPut, "/r-1/status", strings.NewReader(`{"status":"reviewed","action":"suspend"}`)))
	rec := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Put("/{id}/status", mgr.UpdateReportStatus)
	middleware.RequireAuth(testSecret)(middleware.RequireAdmin(router)).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func putStatus(t *testing.T, mgr *reports.Manager, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := adminAuthedRequest(httptest.NewRequest(http.MethodPut, "/r-1/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Put("/{id}/status", mgr.UpdateReportStatus)
	middleware.RequireAuth(testSecret)(middleware.RequireAdmin(router)).ServeHTTP(rec, req)
	return rec
}

func TestUpdateReportStatus_FailedActionLeavesStatusUntouched(t *testing.T) {
	cases := []struct {
		name     string
		modErr   error
		wantCode int
		wantErr  string
	}{
		{"account purged during review", fmt.Errorf("%w: purged", reports.ErrReportedUserGone), http.StatusConflict, "reported_account_gone"},
		{"already moderated", reports.ErrAlreadyModerated, http.StatusConflict, "already_moderated"},
		{"moderator failure", errors.New("db down"), http.StatusInternalServerError, "internal_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &mockHandlerStore{report: &reports.Report{ID: "r-1", ReportedUserID: new("u-2"), Status: "pending"}}
			mod := &mockModerator{suspendErr: tc.modErr, banErr: tc.modErr}
			mgr := reports.NewManager(reports.NewService(store), reports.WithModerator(mod))

			rec := putStatus(t, mgr, `{"status":"reviewed","action":"ban","reason":"x"}`)
			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tc.wantCode, rec.Body.String())
			}
			if got := errorCode(t, rec); got != tc.wantErr {
				t.Errorf("error code = %q, want %q", got, tc.wantErr)
			}
			if store.updated {
				t.Error("report status changed although the action failed")
			}
		})
	}
}

func TestUpdateReportStatus_ActionRunsBeforeStatusChange(t *testing.T) {
	store := &mockHandlerStore{report: &reports.Report{ID: "r-1", ReportedUserID: new("u-2"), Status: "pending"}}
	mod := &orderModerator{store: store}
	mgr := reports.NewManager(reports.NewService(store), reports.WithModerator(mod))

	if rec := putStatus(t, mgr, `{"status":"reviewed","action":"suspend","duration_days":3}`); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !mod.ran || mod.statusChangedFirst {
		t.Errorf("moderation ran=%v, status changed before it=%v; want the action first", mod.ran, mod.statusChangedFirst)
	}
	if !store.updated {
		t.Error("status was not updated after a successful action")
	}
}

func TestUpdateReportStatus_ActionRequiresReviewed(t *testing.T) {
	store := &mockHandlerStore{report: &reports.Report{ID: "r-1", ReportedUserID: new("u-2"), Status: "pending"}}
	mod := &mockModerator{}
	mgr := reports.NewManager(reports.NewService(store), reports.WithModerator(mod))

	rec := putStatus(t, mgr, `{"status":"dismissed","action":"ban","reason":"x"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if mod.banned || store.updated {
		t.Error("a dismissal must neither run an action nor change status")
	}
}

// orderModerator records whether the report status had already changed when
// the moderation action ran.
type orderModerator struct {
	store              *mockHandlerStore
	ran                bool
	statusChangedFirst bool
}

func (o *orderModerator) SuspendUser(_ context.Context, _, _ string, _ int, _ string) error {
	o.ran, o.statusChangedFirst = true, o.store.updated
	return nil
}

func (o *orderModerator) BanUser(_ context.Context, _, _, _ string) error {
	o.ran, o.statusChangedFirst = true, o.store.updated
	return nil
}
