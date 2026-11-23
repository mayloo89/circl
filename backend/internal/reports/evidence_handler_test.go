package reports_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/reports"
)

type mockKeeper struct {
	preserved, discarded []string
	preserveErr          error
	discardErr           error
}

func (m *mockKeeper) Preserve(_ context.Context, r *reports.Report) error {
	m.preserved = append(m.preserved, r.ID)
	return m.preserveErr
}

func (m *mockKeeper) Discard(_ context.Context, reportID string) error {
	m.discarded = append(m.discarded, reportID)
	return m.discardErr
}

func createWithKeeper(t *testing.T, keeper *mockKeeper) int {
	t.Helper()
	store := &mockHandlerStore{report: &reports.Report{ID: "r-1", ReportedUserID: new("u-2"), Status: reports.StatusPending}}
	h := reports.NewHandler(reports.NewManager(reports.NewService(store), reports.WithEvidence(keeper, http.NotFoundHandler())))
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"reported_user_id":"u-2","reason":"harassment"}`)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)
	return rec.Code
}

func TestCreateReport_PreservesEvidence(t *testing.T) {
	keeper := &mockKeeper{}
	if code := createWithKeeper(t, keeper); code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", code)
	}
	if len(keeper.preserved) != 1 || keeper.preserved[0] != "r-1" {
		t.Errorf("preserved = %v, want [r-1]", keeper.preserved)
	}
}

func TestCreateReport_EvidenceFailureDoesNotFailTheReport(t *testing.T) {
	keeper := &mockKeeper{preserveErr: errors.New("storage offline")}
	if code := createWithKeeper(t, keeper); code != http.StatusCreated {
		t.Errorf("status = %d, want 201 despite the copy failing", code)
	}
}

func updateWithKeeper(t *testing.T, keeper *mockKeeper, status string) int {
	t.Helper()
	store := &mockHandlerStore{report: &reports.Report{ID: "r-1", ReportedUserID: new("u-2"), Status: status}}
	h := reports.NewHandler(reports.NewManager(reports.NewService(store), reports.WithEvidence(keeper, http.NotFoundHandler())))
	req := adminAuthedRequest(httptest.NewRequest(http.MethodPut, "/r-1/status", strings.NewReader(`{"status":"`+status+`"}`)))
	rec := httptest.NewRecorder()
	middleware.RequireAuth(testSecret)(h).ServeHTTP(rec, req)
	return rec.Code
}

func TestUpdateReportStatus_DismissalDiscardsEvidence(t *testing.T) {
	keeper := &mockKeeper{}
	if code := updateWithKeeper(t, keeper, reports.StatusDismissed); code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if len(keeper.discarded) != 1 || keeper.discarded[0] != "r-1" {
		t.Errorf("discarded = %v, want [r-1]", keeper.discarded)
	}

	// A failed discard is left for the sweep; the status change still stands.
	keeper = &mockKeeper{discardErr: errors.New("offline")}
	if code := updateWithKeeper(t, keeper, reports.StatusDismissed); code != http.StatusOK {
		t.Errorf("status = %d, want 200 despite the discard failing", code)
	}
}

func TestUpdateReportStatus_ReviewKeepsEvidence(t *testing.T) {
	keeper := &mockKeeper{}
	if code := updateWithKeeper(t, keeper, reports.StatusReviewed); code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if len(keeper.discarded) != 0 {
		t.Errorf("discarded = %v on review, want none", keeper.discarded)
	}
}

func TestEvidenceRoutes_AdminOnly(t *testing.T) {
	evidence := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := reports.NewHandler(reports.NewManager(reports.NewService(&mockHandlerStore{}), reports.WithEvidence(&mockKeeper{}, evidence)))

	rec := httptest.NewRecorder()
	serve(h, authedRequest(httptest.NewRequest(http.MethodGet, "/r-1/evidence", nil)), rec)
	if rec.Code != http.StatusForbidden {
		t.Errorf("non-admin: status = %d, want 403", rec.Code)
	}

	rec = httptest.NewRecorder()
	serve(h, adminAuthedRequest(httptest.NewRequest(http.MethodGet, "/r-1/evidence/f-1", nil)), rec)
	if rec.Code != http.StatusTeapot {
		t.Errorf("admin: status = %d, want the evidence handler", rec.Code)
	}
}
