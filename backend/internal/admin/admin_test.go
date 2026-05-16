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
	user              *admin.UserRecord
	getUserErr        error
	setStatusErr      error
	suspension        *admin.Suspension
	createSusErr      error
	isActive          bool
	isActiveErr       error
	stats             *admin.Stats
	getStatsErr       error
	users             []*admin.UserRecord
	usersTotal        int
	listUsersErr      error
	reactivateErr     error
	channels          []admin.ChannelRecord
	listChansErr      error
	deleteChansErr    error
	createdChannel    *admin.ChannelRecord
	createChanErr     error
	updateChanErr     error
	hardDeleteErr     error
	setRoleErr        error
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

func (m *mockStore) GetStats(_ context.Context) (*admin.Stats, error) {
	return m.stats, m.getStatsErr
}

func (m *mockStore) ListUsers(_ context.Context, _, _ string, _, _ int) ([]*admin.UserRecord, int, error) {
	return m.users, m.usersTotal, m.listUsersErr
}

func (m *mockStore) ReactivateUser(_ context.Context, _ string) error {
	return m.reactivateErr
}

func (m *mockStore) ListChannels(_ context.Context) ([]admin.ChannelRecord, error) {
	return m.channels, m.listChansErr
}

func (m *mockStore) DeleteChannel(_ context.Context, _ string) error {
	return m.deleteChansErr
}

func (m *mockStore) CreateChannel(_ context.Context, _, _, _ string) (*admin.ChannelRecord, error) {
	return m.createdChannel, m.createChanErr
}

func (m *mockStore) UpdateChannel(_ context.Context, _, _, _ string) error {
	return m.updateChanErr
}

func (m *mockStore) HardDeleteUser(_ context.Context, _ string) error {
	return m.hardDeleteErr
}

func (m *mockStore) SetUserRole(_ context.Context, _, _ string) error {
	return m.setRoleErr
}

// --- SuspendUser ---

func TestService_SuspendUser_Success(t *testing.T) {
	store := &mockStore{
		user:       &admin.UserRecord{ID: "u-1", Status: "active"},
		suspension: &admin.Suspension{ID: "s-1"},
	}
	svc := admin.NewService(store)

	err := svc.SuspendUser(t.Context(), "u-1", "spam", 7, "admin-1")
	if err != nil {
		t.Fatalf("SuspendUser() error = %v, want nil", err)
	}
}

func TestService_SuspendUser_AlreadySuspended(t *testing.T) {
	for _, status := range []string{"suspended", "banned"} {
		store := &mockStore{user: &admin.UserRecord{ID: "u-1", Status: status}}
		svc := admin.NewService(store)

		err := svc.SuspendUser(t.Context(), "u-1", "spam", 7, "admin-1")
		if !errors.Is(err, admin.ErrAlreadySuspended) {
			t.Errorf("status=%q: error = %v, want ErrAlreadySuspended", status, err)
		}
	}
}

func TestService_SuspendUser_UserNotFound(t *testing.T) {
	store := &mockStore{getUserErr: admin.ErrUserNotFound}
	svc := admin.NewService(store)

	err := svc.SuspendUser(t.Context(), "u-1", "spam", 7, "admin-1")
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

	err := svc.SuspendUser(t.Context(), "u-1", "spam", 7, "admin-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- BanUser ---

func TestService_BanUser_Success(t *testing.T) {
	store := &mockStore{
		user:       &admin.UserRecord{ID: "u-1", Email: "u@example.com", Status: "active"},
		suspension: &admin.Suspension{ID: "s-1"},
	}
	svc := admin.NewService(store)

	err := svc.BanUser(t.Context(), "u-1", "severe violation", "admin-1")
	if err != nil {
		t.Fatalf("BanUser() error = %v, want nil", err)
	}
}

func TestService_BanUser_StatusError(t *testing.T) {
	store := &mockStore{
		user:         &admin.UserRecord{ID: "u-1", Email: "u@example.com", Status: "active"},
		setStatusErr: errors.New("db error"),
	}
	svc := admin.NewService(store)

	err := svc.BanUser(t.Context(), "u-1", "severe violation", "admin-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestService_BanUser_UserNotFound(t *testing.T) {
	store := &mockStore{getUserErr: admin.ErrUserNotFound}
	svc := admin.NewService(store)

	err := svc.BanUser(t.Context(), "u-1", "severe violation", "admin-1")
	if !errors.Is(err, admin.ErrUserNotFound) {
		t.Errorf("error = %v, want ErrUserNotFound", err)
	}
}

func TestService_SuspendUser_CallsNotifier(t *testing.T) {
	store := &mockStore{
		user:       &admin.UserRecord{ID: "u-1", Email: "u@example.com", Status: "active"},
		suspension: &admin.Suspension{ID: "s-1"},
	}
	svc := admin.NewService(store)
	n := &capturingNotifier{}
	svc.SetSuspensionNotifier(n)

	if err := svc.SuspendUser(t.Context(), "u-1", "spam", 7, "admin-1"); err != nil {
		t.Fatalf("SuspendUser() error = %v", err)
	}
	if !n.called {
		t.Fatal("expected SuspensionNotifier to be called")
	}
	if n.user.ID != "u-1" || n.suspension.ID != "s-1" {
		t.Errorf("notifier saw user=%q suspension=%q, want u-1/s-1", n.user.ID, n.suspension.ID)
	}
}

func TestService_BanUser_CallsNotifier(t *testing.T) {
	store := &mockStore{
		user:       &admin.UserRecord{ID: "u-1", Email: "u@example.com", Status: "active"},
		suspension: &admin.Suspension{ID: "s-1"},
	}
	svc := admin.NewService(store)
	n := &capturingNotifier{}
	svc.SetSuspensionNotifier(n)

	if err := svc.BanUser(t.Context(), "u-1", "severe", "admin-1"); err != nil {
		t.Fatalf("BanUser() error = %v", err)
	}
	if !n.called {
		t.Fatal("expected SuspensionNotifier to be called")
	}
}

type capturingNotifier struct {
	called     bool
	user       admin.UserRecord
	suspension admin.Suspension
}

func (n *capturingNotifier) NotifyOfSuspension(_ context.Context, u admin.UserRecord, s admin.Suspension) {
	n.called = true
	n.user = u
	n.suspension = s
}

// --- IsActiveUser ---

func TestService_IsActiveUser_True(t *testing.T) {
	store := &mockStore{isActive: true}
	svc := admin.NewService(store)

	active, err := svc.IsActiveUser(t.Context(), "u-1")
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

	active, err := svc.IsActiveUser(t.Context(), "u-1")
	if err != nil {
		t.Fatalf("IsActiveUser() error = %v", err)
	}
	if active {
		t.Error("expected active=false")
	}
}

// --- GetStats ---

func TestService_GetStats_Success(t *testing.T) {
	st := &admin.Stats{TotalUsers: 10, ActiveUsers: 7, PendingReports: 3}
	store := &mockStore{stats: st}
	svc := admin.NewService(store)

	got, err := svc.GetStats(t.Context())
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if got.TotalUsers != 10 {
		t.Errorf("TotalUsers = %d, want 10", got.TotalUsers)
	}
	if got.ActiveUsers != 7 {
		t.Errorf("ActiveUsers = %d, want 7", got.ActiveUsers)
	}
}

func TestService_GetStats_Error(t *testing.T) {
	store := &mockStore{getStatsErr: errors.New("db error")}
	svc := admin.NewService(store)

	_, err := svc.GetStats(t.Context())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- ListUsers ---

func TestService_ListUsers_Success(t *testing.T) {
	users := []*admin.UserRecord{
		{ID: "u-1", Email: "a@example.com", Status: "active"},
		{ID: "u-2", Email: "b@example.com", Status: "suspended"},
	}
	store := &mockStore{users: users, usersTotal: 2}
	svc := admin.NewService(store)

	got, total, err := svc.ListUsers(t.Context(), "", "", 20, 0)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(got) != 2 {
		t.Errorf("len(users) = %d, want 2", len(got))
	}
}

func TestService_ListUsers_Error(t *testing.T) {
	store := &mockStore{listUsersErr: errors.New("db error")}
	svc := admin.NewService(store)

	_, _, err := svc.ListUsers(t.Context(), "", "", 20, 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- ReactivateUser ---

func TestService_ReactivateUser_Success(t *testing.T) {
	store := &mockStore{}
	svc := admin.NewService(store)

	err := svc.ReactivateUser(t.Context(), "u-1")
	if err != nil {
		t.Fatalf("ReactivateUser() error = %v", err)
	}
}

func TestService_ReactivateUser_NotFound(t *testing.T) {
	store := &mockStore{reactivateErr: admin.ErrUserNotFound}
	svc := admin.NewService(store)

	err := svc.ReactivateUser(t.Context(), "u-missing")
	if !errors.Is(err, admin.ErrUserNotFound) {
		t.Errorf("error = %v, want ErrUserNotFound", err)
	}
}

func TestService_ReactivateUser_Error(t *testing.T) {
	store := &mockStore{reactivateErr: errors.New("db error")}
	svc := admin.NewService(store)

	err := svc.ReactivateUser(t.Context(), "u-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- ListChannels ---

func TestService_ListChannels_Success(t *testing.T) {
	channels := []admin.ChannelRecord{
		{ID: "ch-1", Name: "general"},
		{ID: "ch-2", Name: "random"},
	}
	store := &mockStore{channels: channels}
	svc := admin.NewService(store)

	got, err := svc.ListChannels(t.Context())
	if err != nil {
		t.Fatalf("ListChannels() error = %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestService_ListChannels_Error(t *testing.T) {
	store := &mockStore{listChansErr: errors.New("db error")}
	svc := admin.NewService(store)

	_, err := svc.ListChannels(t.Context())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- DeleteChannel ---

func TestService_DeleteChannel_Success(t *testing.T) {
	store := &mockStore{}
	svc := admin.NewService(store)

	if err := svc.DeleteChannel(t.Context(), "ch-1"); err != nil {
		t.Fatalf("DeleteChannel() error = %v", err)
	}
}

func TestService_DeleteChannel_NotFound(t *testing.T) {
	store := &mockStore{deleteChansErr: admin.ErrChannelNotFound}
	svc := admin.NewService(store)

	err := svc.DeleteChannel(t.Context(), "ch-missing")
	if !errors.Is(err, admin.ErrChannelNotFound) {
		t.Errorf("error = %v, want ErrChannelNotFound", err)
	}
}

func TestService_DeleteChannel_Error(t *testing.T) {
	store := &mockStore{deleteChansErr: errors.New("db error")}
	svc := admin.NewService(store)

	if err := svc.DeleteChannel(t.Context(), "ch-1"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- CreateChannel ---

func TestService_CreateChannel_Success(t *testing.T) {
	ch := &admin.ChannelRecord{ID: "ch-1", Name: "general"}
	store := &mockStore{createdChannel: ch}
	svc := admin.NewService(store)

	got, err := svc.CreateChannel(t.Context(), "admin-1", "general", "desc")
	if err != nil {
		t.Fatalf("CreateChannel() error = %v", err)
	}
	if got.ID != "ch-1" {
		t.Errorf("ID = %q, want ch-1", got.ID)
	}
}

func TestService_CreateChannel_NameTaken(t *testing.T) {
	store := &mockStore{createChanErr: admin.ErrChannelNameTaken}
	svc := admin.NewService(store)

	_, err := svc.CreateChannel(t.Context(), "admin-1", "general", "")
	if !errors.Is(err, admin.ErrChannelNameTaken) {
		t.Errorf("error = %v, want ErrChannelNameTaken", err)
	}
}

func TestService_CreateChannel_Error(t *testing.T) {
	store := &mockStore{createChanErr: errors.New("db error")}
	svc := admin.NewService(store)

	if _, err := svc.CreateChannel(t.Context(), "admin-1", "general", ""); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- UpdateChannel ---

func TestService_UpdateChannel_Success(t *testing.T) {
	store := &mockStore{}
	svc := admin.NewService(store)

	if err := svc.UpdateChannel(t.Context(), "ch-1", "new-name", "new-desc"); err != nil {
		t.Fatalf("UpdateChannel() error = %v", err)
	}
}

func TestService_UpdateChannel_NotFound(t *testing.T) {
	store := &mockStore{updateChanErr: admin.ErrChannelNotFound}
	svc := admin.NewService(store)

	err := svc.UpdateChannel(t.Context(), "ch-missing", "x", "")
	if !errors.Is(err, admin.ErrChannelNotFound) {
		t.Errorf("error = %v, want ErrChannelNotFound", err)
	}
}

func TestService_UpdateChannel_NameTaken(t *testing.T) {
	store := &mockStore{updateChanErr: admin.ErrChannelNameTaken}
	svc := admin.NewService(store)

	err := svc.UpdateChannel(t.Context(), "ch-1", "taken", "")
	if !errors.Is(err, admin.ErrChannelNameTaken) {
		t.Errorf("error = %v, want ErrChannelNameTaken", err)
	}
}

func TestService_UpdateChannel_Error(t *testing.T) {
	store := &mockStore{updateChanErr: errors.New("db error")}
	svc := admin.NewService(store)

	if err := svc.UpdateChannel(t.Context(), "ch-1", "x", ""); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- HardDeleteUser ---

func TestService_HardDeleteUser_Success(t *testing.T) {
	store := &mockStore{}
	svc := admin.NewService(store)

	if err := svc.HardDeleteUser(t.Context(), "u-1"); err != nil {
		t.Fatalf("HardDeleteUser() error = %v", err)
	}
}

func TestService_HardDeleteUser_NotFound(t *testing.T) {
	store := &mockStore{hardDeleteErr: admin.ErrUserNotFound}
	svc := admin.NewService(store)

	err := svc.HardDeleteUser(t.Context(), "u-missing")
	if !errors.Is(err, admin.ErrUserNotFound) {
		t.Errorf("error = %v, want ErrUserNotFound", err)
	}
}

func TestService_HardDeleteUser_Error(t *testing.T) {
	store := &mockStore{hardDeleteErr: errors.New("db error")}
	svc := admin.NewService(store)

	if err := svc.HardDeleteUser(t.Context(), "u-1"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- SetUserRole ---

func TestService_SetUserRole_Success(t *testing.T) {
	for _, role := range []string{"user", "admin", "super_admin"} {
		store := &mockStore{}
		svc := admin.NewService(store)

		if err := svc.SetUserRole(t.Context(), "u-1", role); err != nil {
			t.Errorf("role=%q: error = %v, want nil", role, err)
		}
	}
}

func TestService_SetUserRole_InvalidRole(t *testing.T) {
	store := &mockStore{}
	svc := admin.NewService(store)

	err := svc.SetUserRole(t.Context(), "u-1", "owner")
	if !errors.Is(err, admin.ErrInvalidRole) {
		t.Errorf("error = %v, want ErrInvalidRole", err)
	}
}

func TestService_SetUserRole_NotFound(t *testing.T) {
	store := &mockStore{setRoleErr: admin.ErrUserNotFound}
	svc := admin.NewService(store)

	err := svc.SetUserRole(t.Context(), "u-missing", "user")
	if !errors.Is(err, admin.ErrUserNotFound) {
		t.Errorf("error = %v, want ErrUserNotFound", err)
	}
}

func TestService_SetUserRole_Error(t *testing.T) {
	store := &mockStore{setRoleErr: errors.New("db error")}
	svc := admin.NewService(store)

	if err := svc.SetUserRole(t.Context(), "u-1", "admin"); err == nil {
		t.Fatal("expected error, got nil")
	}
}
