package reports_test

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
	"github.com/mayloo89/circl/backend/internal/reports"
	"github.com/mayloo89/circl/backend/internal/token"
)

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
}

func (m *mockHandlerStore) Create(_ context.Context, _, _, _, _ string) (*reports.Report, error) {
	return m.report, m.createErr
}

func (m *mockHandlerStore) GetByID(_ context.Context, _ string) (*reports.Report, error) {
	return m.report, m.getByIDErr
}

func (m *mockHandlerStore) List(_ context.Context, _ string) ([]reports.ReportWithUserInfo, error) {
	return m.reportsList, m.listErr
}

func (m *mockHandlerStore) UpdateStatus(_ context.Context, _, _, _ string) (*reports.Report, error) {
	return m.report, m.updateErr
}

// serve wraps the handler with RequireAuth and executes the request.
func serve(h http.Handler, r *http.Request, rec *httptest.ResponseRecorder) {
	middleware.RequireAuth(testSecret)(h).ServeHTTP(rec, r)
}

// authedRequest adds a valid Bearer token for testUserID to the request.
func authedRequest(r *http.Request) *http.Request {
	tok, _ := token.Generate(testUserID, testSecret, time.Hour)
	r.Header.Set("Authorization", "Bearer "+tok)
	return r
}

// --- CreateReport ---

func TestCreateReport_Success(t *testing.T) {
	now := time.Now()
	report := &reports.Report{
		ID:             "r-1",
		ReporterID:     testUserID,
		ReportedUserID: "u-2",
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

// --- ListReports ---

func TestListReports_Success(t *testing.T) {
	now := time.Now()
	reportsList := []reports.ReportWithUserInfo{
		{
			ID:             "r-1",
			ReporterID:     "u-1",
			ReportedUserID: "u-2",
			Reason:         "spam",
			Status:         "pending",
			CreatedAt:      now,
		},
	}
	store := &mockHandlerStore{reportsList: reportsList}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got []reports.ReportWithUserInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len = %d, want 1", len(got))
	}
}

func TestListReports_WithStatusFilter(t *testing.T) {
	reportsList := []reports.ReportWithUserInfo{}
	store := &mockHandlerStore{reportsList: reportsList}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/?status=pending", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
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

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/?status=invalid", nil))
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

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestListReports_NoUserIDInContext(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// --- UpdateReportStatus ---

func TestUpdateReportStatus_Success(t *testing.T) {
	now := time.Now()
	reviewedBy := testUserID
	report := &reports.Report{
		ID:             "r-1",
		ReporterID:     "u-1",
		ReportedUserID: "u-2",
		Reason:         "harassment",
		Status:         "reviewed",
		ReviewedBy:     &reviewedBy,
		ReviewedAt:     &now,
		CreatedAt:      now,
	}
	store := &mockHandlerStore{report: report}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	body := `{"status":"reviewed"}`
	req := authedRequest(httptest.NewRequest(http.MethodPut, "/r-1/status", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got reports.Report
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.Status != "reviewed" {
		t.Errorf("status = %q, want reviewed", got.Status)
	}
}

func TestUpdateReportStatus_Unauthorized(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := httptest.NewRequest(http.MethodPut, "/r-1/status", strings.NewReader(`{"status":"reviewed"}`))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestUpdateReportStatus_MalformedJSON(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := authedRequest(httptest.NewRequest(http.MethodPut, "/r-1/status", strings.NewReader("{bad")))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdateReportStatus_MissingStatus(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := authedRequest(httptest.NewRequest(http.MethodPut, "/r-1/status", strings.NewReader(`{"status":""}`)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdateReportStatus_NotFound(t *testing.T) {
	store := &mockHandlerStore{updateErr: reports.ErrNotFound}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := authedRequest(httptest.NewRequest(http.MethodPut, "/r-1/status", strings.NewReader(`{"status":"reviewed"}`)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestUpdateReportStatus_InvalidStatus(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := authedRequest(httptest.NewRequest(http.MethodPut, "/r-1/status", strings.NewReader(`{"status":"invalid"}`)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdateReportStatus_InternalError(t *testing.T) {
	store := &mockHandlerStore{updateErr: errors.New("db error")}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := authedRequest(httptest.NewRequest(http.MethodPut, "/r-1/status", strings.NewReader(`{"status":"reviewed"}`)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestUpdateReportStatus_NoUserIDInContext(t *testing.T) {
	store := &mockHandlerStore{}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := httptest.NewRequest(http.MethodPut, "/r-1/status", strings.NewReader(`{"status":"reviewed"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// --- Content-Type ---

func TestHandlers_ContentType(t *testing.T) {
	reportsList := []reports.ReportWithUserInfo{}
	store := &mockHandlerStore{reportsList: reportsList}
	svc := reports.NewService(store)
	mgr := reports.NewManager(svc)
	h := reports.NewHandler(mgr)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
