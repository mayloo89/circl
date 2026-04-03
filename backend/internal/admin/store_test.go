package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// --- mock querier ---

type mockRow struct {
	scanFn func(dest ...any) error
}

func (m *mockRow) Scan(dest ...any) error {
	return m.scanFn(dest...)
}

type mockQuerier struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) rowScanner
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (m *mockQuerier) QueryRow(ctx context.Context, sql string, args ...any) rowScanner {
	return m.queryRowFn(ctx, sql, args...)
}

func (m *mockQuerier) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return m.execFn(ctx, sql, args...)
}

// --- GetUserByID ---

func TestPgStore_GetUserByID_Success(t *testing.T) {
	now := time.Now()
	q := &mockQuerier{
		queryRowFn: func(_ context.Context, _ string, _ ...any) rowScanner {
			return &mockRow{scanFn: func(dest ...any) error {
				*dest[0].(*string) = "u-1"
				*dest[1].(*string) = "user@example.com"
				*dest[2].(*string) = "active"
				*dest[3].(*bool) = false
				*dest[4].(*time.Time) = now
				return nil
			}}
		},
	}
	s := &pgStore{db: q}

	u, err := s.GetUserByID(context.Background(), "u-1")
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if u.ID != "u-1" {
		t.Errorf("ID = %q, want u-1", u.ID)
	}
	if u.Email != "user@example.com" {
		t.Errorf("Email = %q, want user@example.com", u.Email)
	}
}

func TestPgStore_GetUserByID_NotFound(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(_ context.Context, _ string, _ ...any) rowScanner {
			return &mockRow{scanFn: func(_ ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}
	s := &pgStore{db: q}

	_, err := s.GetUserByID(context.Background(), "u-1")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("error = %v, want ErrUserNotFound", err)
	}
}

func TestPgStore_GetUserByID_Error(t *testing.T) {
	dbErr := errors.New("connection lost")
	q := &mockQuerier{
		queryRowFn: func(_ context.Context, _ string, _ ...any) rowScanner {
			return &mockRow{scanFn: func(_ ...any) error { return dbErr }}
		},
	}
	s := &pgStore{db: q}

	_, err := s.GetUserByID(context.Background(), "u-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, ErrUserNotFound) {
		t.Error("expected wrapped error, not ErrUserNotFound")
	}
}

// --- SetUserStatus ---

func TestPgStore_SetUserStatus_Success(t *testing.T) {
	q := &mockQuerier{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	s := &pgStore{db: q}

	if err := s.SetUserStatus(context.Background(), "u-1", "suspended"); err != nil {
		t.Fatalf("SetUserStatus() error = %v", err)
	}
}

func TestPgStore_SetUserStatus_NotFound(t *testing.T) {
	q := &mockQuerier{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	s := &pgStore{db: q}

	err := s.SetUserStatus(context.Background(), "u-missing", "suspended")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("error = %v, want ErrUserNotFound", err)
	}
}

func TestPgStore_SetUserStatus_Error(t *testing.T) {
	q := &mockQuerier{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db error")
		},
	}
	s := &pgStore{db: q}

	err := s.SetUserStatus(context.Background(), "u-1", "suspended")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- CreateSuspension ---

func TestPgStore_CreateSuspension_Success(t *testing.T) {
	now := time.Now()
	q := &mockQuerier{
		queryRowFn: func(_ context.Context, _ string, _ ...any) rowScanner {
			return &mockRow{scanFn: func(dest ...any) error {
				*dest[0].(*string) = "sus-1"
				*dest[1].(*string) = "u-1"
				*dest[2].(**time.Time) = nil
				*dest[3].(*string) = "spam"
				*dest[4].(*string) = "admin-1"
				*dest[5].(*time.Time) = now
				return nil
			}}
		},
	}
	s := &pgStore{db: q}

	sus, err := s.CreateSuspension(context.Background(), "u-1", nil, "spam", "admin-1")
	if err != nil {
		t.Fatalf("CreateSuspension() error = %v", err)
	}
	if sus.ID != "sus-1" {
		t.Errorf("ID = %q, want sus-1", sus.ID)
	}
}

func TestPgStore_CreateSuspension_Error(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(_ context.Context, _ string, _ ...any) rowScanner {
			return &mockRow{scanFn: func(_ ...any) error { return errors.New("db error") }}
		},
	}
	s := &pgStore{db: q}

	_, err := s.CreateSuspension(context.Background(), "u-1", nil, "spam", "admin-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- IsActiveUser ---

func TestPgStore_IsActiveUser_True(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(_ context.Context, _ string, _ ...any) rowScanner {
			return &mockRow{scanFn: func(dest ...any) error {
				*dest[0].(*bool) = true
				return nil
			}}
		},
	}
	s := &pgStore{db: q}

	active, err := s.IsActiveUser(context.Background(), "u-1")
	if err != nil {
		t.Fatalf("IsActiveUser() error = %v", err)
	}
	if !active {
		t.Error("expected active=true")
	}
}

func TestPgStore_IsActiveUser_False(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(_ context.Context, _ string, _ ...any) rowScanner {
			return &mockRow{scanFn: func(dest ...any) error {
				*dest[0].(*bool) = false
				return nil
			}}
		},
	}
	s := &pgStore{db: q}

	active, err := s.IsActiveUser(context.Background(), "u-1")
	if err != nil {
		t.Fatalf("IsActiveUser() error = %v", err)
	}
	if active {
		t.Error("expected active=false")
	}
}

func TestPgStore_IsActiveUser_Error(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(_ context.Context, _ string, _ ...any) rowScanner {
			return &mockRow{scanFn: func(_ ...any) error { return errors.New("db error") }}
		},
	}
	s := &pgStore{db: q}

	_, err := s.IsActiveUser(context.Background(), "u-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
