package appeals_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mayloo89/circl/backend/internal/appeals"
)

type mockStore struct {
	create        *appeals.Appeal
	byToken       *appeals.Appeal
	byID          *appeals.Appeal
	submit        *appeals.Appeal
	resolve       *appeals.Appeal
	list          []appeals.AppealWithUserInfo
	createErr     error
	byTokenErr    error
	byIDErr       error
	submitErr     error
	resolveErr    error
	listErr       error
	lastTokenHash string
}

func (m *mockStore) Create(_ context.Context, _, _, tokenHash string, _ time.Time) (*appeals.Appeal, error) {
	m.lastTokenHash = tokenHash
	return m.create, m.createErr
}

func (m *mockStore) GetByTokenHash(_ context.Context, _ string) (*appeals.Appeal, error) {
	return m.byToken, m.byTokenErr
}

func (m *mockStore) GetByID(_ context.Context, _ string) (*appeals.Appeal, error) {
	return m.byID, m.byIDErr
}

func (m *mockStore) Submit(_ context.Context, _, _ string) (*appeals.Appeal, error) {
	return m.submit, m.submitErr
}

func (m *mockStore) Resolve(_ context.Context, _, _, _, _ string) (*appeals.Appeal, error) {
	return m.resolve, m.resolveErr
}

func (m *mockStore) List(_ context.Context, _ string) ([]appeals.AppealWithUserInfo, error) {
	return m.list, m.listErr
}

type mockReactivator struct {
	called bool
	userID string
	err    error
}

func (m *mockReactivator) ReactivateUser(_ context.Context, userID string) error {
	m.called = true
	m.userID = userID
	return m.err
}

func TestCreateForSuspension_StoresHashedToken(t *testing.T) {
	store := &mockStore{create: &appeals.Appeal{ID: "a-1"}}
	svc := appeals.NewService(store, nil)

	plain, a, err := svc.CreateForSuspension(t.Context(), "u-1", "s-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plain == "" {
		t.Fatal("expected non-empty plaintext token")
	}
	if a.ID != "a-1" {
		t.Errorf("appeal ID = %q, want a-1", a.ID)
	}
	if store.lastTokenHash == plain {
		t.Fatal("token hash must differ from plaintext")
	}
	if appeals.HashToken(plain) != store.lastTokenHash {
		t.Errorf("stored hash does not match SHA-256 of plaintext")
	}
}

func TestGetPublic_InvalidToken(t *testing.T) {
	svc := appeals.NewService(&mockStore{byTokenErr: appeals.ErrNotFound}, nil)

	_, err := svc.GetPublic(t.Context(), "deadbeef")
	if !errors.Is(err, appeals.ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestGetPublic_EmptyToken(t *testing.T) {
	svc := appeals.NewService(&mockStore{}, nil)

	_, err := svc.GetPublic(t.Context(), "")
	if !errors.Is(err, appeals.ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestGetPublic_ExpiredToken(t *testing.T) {
	store := &mockStore{byToken: &appeals.Appeal{ID: "a-1", Status: appeals.StatusOpen, ExpiresAt: time.Now().Add(-time.Hour)}}
	svc := appeals.NewService(store, nil)

	_, err := svc.GetPublic(t.Context(), "anytoken")
	if !errors.Is(err, appeals.ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken for expired, got %v", err)
	}
}

func TestSubmitPublic_HappyPath(t *testing.T) {
	store := &mockStore{
		byToken: &appeals.Appeal{ID: "a-1", Status: appeals.StatusOpen, ExpiresAt: time.Now().Add(time.Hour)},
		submit:  &appeals.Appeal{ID: "a-1", Status: appeals.StatusSubmitted, Body: "please reconsider"},
	}
	svc := appeals.NewService(store, nil)

	view, err := svc.SubmitPublic(t.Context(), "anytoken", "please reconsider")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if view.Status != appeals.StatusSubmitted {
		t.Errorf("status = %q, want submitted", view.Status)
	}
	if view.Body != "please reconsider" {
		t.Errorf("body = %q, want %q", view.Body, "please reconsider")
	}
}

func TestSubmitPublic_AlreadyResolved(t *testing.T) {
	store := &mockStore{byToken: &appeals.Appeal{ID: "a-1", Status: appeals.StatusApproved, ExpiresAt: time.Now().Add(time.Hour)}}
	svc := appeals.NewService(store, nil)

	_, err := svc.SubmitPublic(t.Context(), "anytoken", "hi")
	if !errors.Is(err, appeals.ErrAlreadyResolved) {
		t.Errorf("expected ErrAlreadyResolved, got %v", err)
	}
}

func TestResolve_ApprovedReactivatesUser(t *testing.T) {
	store := &mockStore{
		byID:    &appeals.Appeal{ID: "a-1", Status: appeals.StatusSubmitted, UserID: "u-1"},
		resolve: &appeals.Appeal{ID: "a-1", Status: appeals.StatusApproved, UserID: "u-1"},
	}
	reactivator := &mockReactivator{}
	svc := appeals.NewService(store, reactivator)

	got, err := svc.Resolve(t.Context(), "a-1", appeals.StatusApproved, "admin-1", "looks legit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != appeals.StatusApproved {
		t.Errorf("status = %q, want approved", got.Status)
	}
	if !reactivator.called || reactivator.userID != "u-1" {
		t.Errorf("expected ReactivateUser('u-1'), got called=%v userID=%q", reactivator.called, reactivator.userID)
	}
}

func TestResolve_DeniedDoesNotReactivate(t *testing.T) {
	store := &mockStore{
		byID:    &appeals.Appeal{ID: "a-1", Status: appeals.StatusSubmitted, UserID: "u-1"},
		resolve: &appeals.Appeal{ID: "a-1", Status: appeals.StatusDenied, UserID: "u-1"},
	}
	reactivator := &mockReactivator{}
	svc := appeals.NewService(store, reactivator)

	if _, err := svc.Resolve(t.Context(), "a-1", appeals.StatusDenied, "admin-1", "no"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reactivator.called {
		t.Error("ReactivateUser should not be called for denial")
	}
}

func TestResolve_InvalidStatus(t *testing.T) {
	svc := appeals.NewService(&mockStore{}, nil)

	_, err := svc.Resolve(t.Context(), "a-1", "maybe", "admin-1", "")
	if !errors.Is(err, appeals.ErrInvalidStatus) {
		t.Errorf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestResolve_AlreadyResolved(t *testing.T) {
	store := &mockStore{byID: &appeals.Appeal{ID: "a-1", Status: appeals.StatusApproved}}
	svc := appeals.NewService(store, nil)

	_, err := svc.Resolve(t.Context(), "a-1", appeals.StatusDenied, "admin-1", "")
	if !errors.Is(err, appeals.ErrAlreadyResolved) {
		t.Errorf("expected ErrAlreadyResolved, got %v", err)
	}
}

func TestList_InvalidStatus(t *testing.T) {
	svc := appeals.NewService(&mockStore{}, nil)

	_, err := svc.List(t.Context(), "garbage")
	if !errors.Is(err, appeals.ErrInvalidStatus) {
		t.Errorf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestList_EmptyStatusReturnsAll(t *testing.T) {
	store := &mockStore{list: []appeals.AppealWithUserInfo{{Appeal: appeals.Appeal{ID: "a-1"}}}}
	svc := appeals.NewService(store, nil)

	got, err := svc.List(t.Context(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len = %d, want 1", len(got))
	}
}
