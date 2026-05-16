package reports_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mayloo89/circl/backend/internal/reports"
)

// mockStore is a test double for reports.Store.
type mockStore struct {
	report       *reports.Report
	reportList   []reports.ReportWithUserInfo
	createErr    error
	getErr       error
	listErr      error
	updateErr    error
	lastPriority string
	lastFilter   reports.ListFilter
}

func (m *mockStore) Create(_ context.Context, _, _, _, priority, _ string) (*reports.Report, error) {
	m.lastPriority = priority
	return m.report, m.createErr
}

func (m *mockStore) GetByID(_ context.Context, _ string) (*reports.Report, error) {
	return m.report, m.getErr
}

func (m *mockStore) List(_ context.Context, f reports.ListFilter) ([]reports.ReportWithUserInfo, error) {
	m.lastFilter = f
	return m.reportList, m.listErr
}

func (m *mockStore) UpdateStatus(_ context.Context, _, _, _ string) (*reports.Report, error) {
	return m.report, m.updateErr
}

func newService(store reports.Store) *reports.Service {
	return reports.NewService(store)
}

func TestService_Create_Success(t *testing.T) {
	expected := &reports.Report{ID: "r-1"}
	svc := newService(&mockStore{report: expected})

	got, err := svc.Create(t.Context(), "u-1", "u-2", reports.ReasonHarassment, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != expected {
		t.Error("expected report to be returned")
	}
}

func TestService_Create_SelfReport(t *testing.T) {
	svc := newService(&mockStore{})

	_, err := svc.Create(t.Context(), "u-1", "u-1", reports.ReasonHarassment, "test")
	if !errors.Is(err, reports.ErrSelfReport) {
		t.Errorf("expected ErrSelfReport, got: %v", err)
	}
}

func TestService_Create_InvalidReason(t *testing.T) {
	svc := newService(&mockStore{})

	_, err := svc.Create(t.Context(), "u-1", "u-2", "invalid_reason", "test")
	if !errors.Is(err, reports.ErrInvalidReason) {
		t.Errorf("expected ErrInvalidReason, got: %v", err)
	}
}

func TestService_Create_StoreError(t *testing.T) {
	svc := newService(&mockStore{createErr: errors.New("db error")})

	_, err := svc.Create(t.Context(), "u-1", "u-2", reports.ReasonSpam, "test")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestService_GetByID_Success(t *testing.T) {
	expected := &reports.Report{ID: "r-1"}
	svc := newService(&mockStore{report: expected})

	got, err := svc.GetByID(t.Context(), "r-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != expected {
		t.Error("expected report to be returned")
	}
}

func TestService_GetByID_NotFound(t *testing.T) {
	svc := newService(&mockStore{getErr: reports.ErrNotFound})

	_, err := svc.GetByID(t.Context(), "r-1")
	if !errors.Is(err, reports.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestService_List_Success(t *testing.T) {
	expected := []reports.ReportWithUserInfo{{ID: "r-1"}}
	svc := newService(&mockStore{reportList: expected})

	got, err := svc.List(t.Context(), reports.ListFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 report, got %d", len(got))
	}
}

func TestService_List_WithStatusFilter(t *testing.T) {
	expected := []reports.ReportWithUserInfo{{ID: "r-1"}}
	store := &mockStore{reportList: expected}
	svc := newService(store)

	got, err := svc.List(t.Context(), reports.ListFilter{Status: reports.StatusPending})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 report, got %d", len(got))
	}
	if store.lastFilter.Status != reports.StatusPending {
		t.Errorf("expected status %q passed to store, got %q", reports.StatusPending, store.lastFilter.Status)
	}
}

func TestService_List_WithPriorityFilter(t *testing.T) {
	store := &mockStore{}
	svc := newService(store)

	if _, err := svc.List(t.Context(), reports.ListFilter{Priority: reports.PriorityCritical}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.lastFilter.Priority != reports.PriorityCritical {
		t.Errorf("expected priority passed to store, got %q", store.lastFilter.Priority)
	}
}

func TestService_List_InvalidStatus(t *testing.T) {
	svc := newService(&mockStore{})

	_, err := svc.List(t.Context(), reports.ListFilter{Status: "invalid_status"})
	if !errors.Is(err, reports.ErrInvalidStatus) {
		t.Errorf("expected ErrInvalidStatus, got: %v", err)
	}
}

func TestService_List_InvalidPriority(t *testing.T) {
	svc := newService(&mockStore{})

	_, err := svc.List(t.Context(), reports.ListFilter{Priority: "panic"})
	if !errors.Is(err, reports.ErrInvalidPriority) {
		t.Errorf("expected ErrInvalidPriority, got: %v", err)
	}
}

func TestService_List_StoreError(t *testing.T) {
	svc := newService(&mockStore{listErr: errors.New("db error")})

	_, err := svc.List(t.Context(), reports.ListFilter{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestService_Create_DerivesPriority(t *testing.T) {
	cases := []struct {
		reason   string
		priority string
	}{
		{reports.ReasonCSAM, reports.PriorityCritical},
		{reports.ReasonNCII, reports.PriorityCritical},
		{reports.ReasonGenderViolence, reports.PriorityHigh},
		{reports.ReasonHarassment, reports.PriorityNormal},
		{reports.ReasonOther, reports.PriorityNormal},
	}
	for _, tc := range cases {
		t.Run(tc.reason, func(t *testing.T) {
			store := &mockStore{report: &reports.Report{ID: "r"}}
			svc := newService(store)
			if _, err := svc.Create(t.Context(), "u-1", "u-2", tc.reason, ""); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if store.lastPriority != tc.priority {
				t.Errorf("reason %q: got priority %q, want %q", tc.reason, store.lastPriority, tc.priority)
			}
		})
	}
}

func TestPriorityFor(t *testing.T) {
	if reports.PriorityFor(reports.ReasonCSAM) != reports.PriorityCritical {
		t.Error("CSAM should be critical")
	}
	if reports.PriorityFor(reports.ReasonNCII) != reports.PriorityCritical {
		t.Error("NCII should be critical")
	}
	if reports.PriorityFor(reports.ReasonGenderViolence) != reports.PriorityHigh {
		t.Error("gender violence should be high")
	}
	if reports.PriorityFor("anything-else") != reports.PriorityNormal {
		t.Error("unknown reason should default to normal")
	}
}

func TestService_UpdateStatus_Success(t *testing.T) {
	expected := &reports.Report{ID: "r-1", Status: reports.StatusReviewed}
	svc := newService(&mockStore{report: expected})

	got, err := svc.UpdateStatus(t.Context(), "r-1", reports.StatusReviewed, "u-admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != expected {
		t.Error("expected updated report to be returned")
	}
}

func TestService_UpdateStatus_InvalidStatus(t *testing.T) {
	svc := newService(&mockStore{})

	_, err := svc.UpdateStatus(t.Context(), "r-1", "invalid_status", "u-admin")
	if !errors.Is(err, reports.ErrInvalidStatus) {
		t.Errorf("expected ErrInvalidStatus, got: %v", err)
	}
}

func TestService_UpdateStatus_NotFound(t *testing.T) {
	svc := newService(&mockStore{updateErr: reports.ErrNotFound})

	_, err := svc.UpdateStatus(t.Context(), "r-1", reports.StatusDismissed, "u-admin")
	if !errors.Is(err, reports.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestService_UpdateStatus_StoreError(t *testing.T) {
	svc := newService(&mockStore{updateErr: errors.New("db error")})

	_, err := svc.UpdateStatus(t.Context(), "r-1", reports.StatusReviewed, "u-admin")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
