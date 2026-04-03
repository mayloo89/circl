package admin_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mayloo89/circl/backend/internal/admin"
)

// mockStore is a test double for admin.Store.
type mockStore struct {
	user          *admin.UserRecord
	getUserErr    error
	setStatusErr  error
	suspension    *admin.Suspension
	createSusErr  error
	isActive      bool
	isActiveErr   error
}

func (m *mockStore) GetUserByID(_ context.Context, _ string) (*admin.UserRecord, error) {
	return m.user, m.getUserErr
}

func (m *mockStore) SetUserStatus(_ context.Context, _, _ string) error {
	return m.setStatusErr
}

func (m *mockStore) CreateSuspension(_ context.Context, _ string, _ *time.Time, _, _ string) (*admin.Suspension, error) {
	return m.suspension, m.createSusErr
}

func (m *mockStore) IsActiveUser(_ context.Context, _ string) (bool, error) {
	return m.isActive, m.isActiveErr
}

func TestService_SuspendUser_Success(t *testing.T) {
	store := &mockStore{
		user:       &admin.UserRecord{ID: "u-1", Status: "active"},
		suspension: &admin.Suspension{ID: "s-1"},
	}
	svc := admin.NewService(store)

	err := svc.SuspendUser(context.Background(), "u-1", "spam", 7, "admin-1")
	if err != nil {
		t.Fatalf("SuspendUser() error = %v, want nil", err)
	}
}

func TestService_SuspendUser_AlreadySuspended(t *testing.T) {
	for _, status := range []string{"suspended", "banned"} {
		store := &mockStore{user: &admin.UserRecord{ID: "u-1", Status: status}}
		svc := admin.NewService(store)

		err := svc.SuspendUser(context.Background(), "u-1", "spam", 7, "admin-1")
		if !errors.Is(err, admin.ErrAlreadySuspended) {
			t.Errorf("status=%q: error = %v, want ErrAlreadySuspended", status, err)
		}
	}
}

func TestService_SuspendUser_UserNotFound(t *testing.T) {
	store := &mockStore{getUserErr: admin.ErrUserNotFound}
	svc := admin.NewService(store)

	err := svc.SuspendUser(context.Background(), "u-1", "spam", 7, "admin-1")
	if !errors.Is(err, admin.ErrUserNotFound) {
		t.Errorf("error = %v, want ErrUserNotFound", err)
	}
}

func TestService_SuspendUser_StatusError(t *testing.T) {
	store := &mockStore{
		user:         &admin.UserRecord{ID: "u-1", Status: "active"},
		setStatusErr: errors.New("db error"),
	}
	svc := admin.NewService(store)

	err := svc.SuspendUser(context.Background(), "u-1", "spam", 7, "admin-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestService_BanUser_Success(t *testing.T) {
	store := &mockStore{
		suspension: &admin.Suspension{ID: "s-1"},
	}
	svc := admin.NewService(store)

	err := svc.BanUser(context.Background(), "u-1", "severe violation", "admin-1")
	if err != nil {
		t.Fatalf("BanUser() error = %v, want nil", err)
	}
}

func TestService_BanUser_StatusError(t *testing.T) {
	store := &mockStore{setStatusErr: errors.New("db error")}
	svc := admin.NewService(store)

	err := svc.BanUser(context.Background(), "u-1", "severe violation", "admin-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestService_IsActiveUser_True(t *testing.T) {
	store := &mockStore{isActive: true}
	svc := admin.NewService(store)

	active, err := svc.IsActiveUser(context.Background(), "u-1")
	if err != nil {
		t.Fatalf("IsActiveUser() error = %v", err)
	}
	if !active {
		t.Error("expected active=true")
	}
}

func TestService_IsActiveUser_False(t *testing.T) {
	store := &mockStore{isActive: false}
	svc := admin.NewService(store)

	active, err := svc.IsActiveUser(context.Background(), "u-1")
	if err != nil {
		t.Fatalf("IsActiveUser() error = %v", err)
	}
	if active {
		t.Error("expected active=false")
	}
}
