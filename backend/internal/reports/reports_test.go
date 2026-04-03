package reports_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mayloo89/circl/backend/internal/reports"
)

// mockStore is a test double for reports.Store.
type mockStore struct {
	report     *reports.Report
	reportList []reports.ReportWithUserInfo
	createErr  error
	getErr     error
	listErr    error
	updateErr  error
}

func (m *mockStore) Create(_ context.Context, _, _, _, _ string) (*reports.Report, error) {
	return m.report, m.createErr
}

func (m *mockStore) GetByID(_ context.Context, _ string) (*reports.Report, error) {
	return m.report, m.getErr
}

func (m *mockStore) List(_ context.Context, _ string) ([]reports.ReportWithUserInfo, error) {
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

	got, err := svc.List(t.Context(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 report, got %d", len(got))
	}
}

func TestService_List_WithStatusFilter(t *testing.T) {
	expected := []reports.ReportWithUserInfo{{ID: "r-1"}}
	svc := newService(&mockStore{reportList: expected})

	got, err := svc.List(t.Context(), reports.StatusPending)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 report, got %d", len(got))
	}
}

func TestService_List_InvalidStatus(t *testing.T) {
	svc := newService(&mockStore{})

	_, err := svc.List(t.Context(), "invalid_status")
	if !errors.Is(err, reports.ErrInvalidStatus) {
		t.Errorf("expected ErrInvalidStatus, got: %v", err)
	}
}

func TestService_List_StoreError(t *testing.T) {
	svc := newService(&mockStore{listErr: errors.New("db error")})

	_, err := svc.List(t.Context(), "")
	if err == nil {
		t.Fatal("expected error, got nil")
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
