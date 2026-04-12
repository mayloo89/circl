package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// --- mock row / rows ---

type mockRow struct {
	scanFn func(dest ...any) error
}

func (m *mockRow) Scan(dest ...any) error { return m.scanFn(dest...) }

// mockRows implements pgx.Rows for unit tests.
type mockRows struct {
	data    [][]any
	pos     int
	scanErr error
	rowsErr error
}

func (r *mockRows) Next() bool                                   { r.pos++; return r.pos <= len(r.data) }
func (r *mockRows) Close()                                       {}
func (r *mockRows) Err() error                                   { return r.rowsErr }
func (r *mockRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *mockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *mockRows) Values() ([]any, error)                       { return nil, nil }
func (r *mockRows) RawValues() [][]byte                          { return nil }
func (r *mockRows) Conn() *pgx.Conn                              { return nil }
func (r *mockRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	row := r.data[r.pos-1]
	for i, d := range dest {
		switch v := d.(type) {
		case *string:
			*v = row[i].(string)
		case *bool:
			*v = row[i].(bool)
		case *int:
			*v = row[i].(int)
		case *time.Time:
			*v = row[i].(time.Time)
		case **time.Time:
			if row[i] == nil {
				*v = nil
			} else {
				t := row[i].(time.Time)
				*v = &t
			}
		}
	}
	return nil
}

// --- mock querier ---

type mockQuerier struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) rowScanner
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (m *mockQuerier) QueryRow(ctx context.Context, sql string, args ...any) rowScanner {
	return m.queryRowFn(ctx, sql, args...)
}

func (m *mockQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return m.queryFn(ctx, sql, args...)
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
				*dest[3].(*string) = "user"
				*dest[4].(*time.Time) = now
				return nil
			}}
		},
	}
	s := &pgStore{db: q}

	u, err := s.GetUserByID(t.Context(), "u-1")
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

	_, err := s.GetUserByID(t.Context(), "u-1")
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

	_, err := s.GetUserByID(t.Context(), "u-1")
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

	if err := s.SetUserStatus(t.Context(), "u-1", "suspended"); err != nil {
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

	err := s.SetUserStatus(t.Context(), "u-missing", "suspended")
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

	err := s.SetUserStatus(t.Context(), "u-1", "suspended")
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

	sus, err := s.CreateSuspension(t.Context(), "u-1", nil, "spam", "admin-1")
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

	_, err := s.CreateSuspension(t.Context(), "u-1", nil, "spam", "admin-1")
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

	active, err := s.IsActiveUser(t.Context(), "u-1")
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

	active, err := s.IsActiveUser(t.Context(), "u-1")
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

	_, err := s.IsActiveUser(t.Context(), "u-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- GetStats ---

func TestPgStore_GetStats_Success(t *testing.T) {
	call := 0
	q := &mockQuerier{
		queryRowFn: func(_ context.Context, _ string, _ ...any) rowScanner {
			call++
			switch call {
			case 1: // user counts
				return &mockRow{scanFn: func(dest ...any) error {
					*dest[0].(*int) = 10
					*dest[1].(*int) = 7
					*dest[2].(*int) = 2
					*dest[3].(*int) = 1
					*dest[4].(*int) = 0
					return nil
				}}
			case 2: // report counts
				return &mockRow{scanFn: func(dest ...any) error {
					*dest[0].(*int) = 5
					*dest[1].(*int) = 3
					return nil
				}}
			default: // room count
				return &mockRow{scanFn: func(dest ...any) error {
					*dest[0].(*int) = 4
					return nil
				}}
			}
		},
	}
	s := &pgStore{db: q}

	st, err := s.GetStats(t.Context())
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if st.TotalUsers != 10 {
		t.Errorf("TotalUsers = %d, want 10", st.TotalUsers)
	}
	if st.ActiveUsers != 7 {
		t.Errorf("ActiveUsers = %d, want 7", st.ActiveUsers)
	}
	if st.SuspendedUsers != 2 {
		t.Errorf("SuspendedUsers = %d, want 2", st.SuspendedUsers)
	}
	if st.BannedUsers != 1 {
		t.Errorf("BannedUsers = %d, want 1", st.BannedUsers)
	}
	if st.TotalReports != 5 {
		t.Errorf("TotalReports = %d, want 5", st.TotalReports)
	}
	if st.PendingReports != 3 {
		t.Errorf("PendingReports = %d, want 3", st.PendingReports)
	}
	if st.TotalRooms != 4 {
		t.Errorf("TotalRooms = %d, want 4", st.TotalRooms)
	}
}

func TestPgStore_GetStats_UserQueryError(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(_ context.Context, _ string, _ ...any) rowScanner {
			return &mockRow{scanFn: func(_ ...any) error { return errors.New("db error") }}
		},
	}
	s := &pgStore{db: q}

	_, err := s.GetStats(t.Context())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPgStore_GetStats_ReportQueryError(t *testing.T) {
	call := 0
	q := &mockQuerier{
		queryRowFn: func(_ context.Context, _ string, _ ...any) rowScanner {
			call++
			if call == 1 {
				return &mockRow{scanFn: func(dest ...any) error {
					*dest[0].(*int) = 0
					*dest[1].(*int) = 0
					*dest[2].(*int) = 0
					*dest[3].(*int) = 0
					*dest[4].(*int) = 0
					return nil
				}}
			}
			return &mockRow{scanFn: func(_ ...any) error { return errors.New("db error") }}
		},
	}
	s := &pgStore{db: q}

	_, err := s.GetStats(t.Context())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPgStore_GetStats_RoomQueryError(t *testing.T) {
	call := 0
	q := &mockQuerier{
		queryRowFn: func(_ context.Context, _ string, _ ...any) rowScanner {
			call++
			if call == 1 {
				return &mockRow{scanFn: func(dest ...any) error {
					*dest[0].(*int) = 0
					*dest[1].(*int) = 0
					*dest[2].(*int) = 0
					*dest[3].(*int) = 0
					*dest[4].(*int) = 0
					return nil
				}}
			}
			if call == 2 {
				return &mockRow{scanFn: func(dest ...any) error {
					*dest[0].(*int) = 0
					*dest[1].(*int) = 0
					return nil
				}}
			}
			return &mockRow{scanFn: func(_ ...any) error { return errors.New("db error") }}
		},
	}
	s := &pgStore{db: q}

	_, err := s.GetStats(t.Context())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- ListUsers ---

func TestPgStore_ListUsers_Success(t *testing.T) {
	now := time.Now()
	rows := &mockRows{
		data: [][]any{
			{"u-1", "a@example.com", "alice", "Alice", "active", "user", now, 2},
			{"u-2", "b@example.com", "bob", "Bob", "suspended", "user", now, 2},
		},
	}
	q := &mockQuerier{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return rows, nil
		},
	}
	s := &pgStore{db: q}

	users, total, err := s.ListUsers(t.Context(), "", "", 20, 0)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(users) != 2 {
		t.Fatalf("len(users) = %d, want 2", len(users))
	}
	if users[0].ID != "u-1" {
		t.Errorf("users[0].ID = %q, want u-1", users[0].ID)
	}
	if users[0].Username != "alice" {
		t.Errorf("users[0].Username = %q, want alice", users[0].Username)
	}
}

func TestPgStore_ListUsers_Empty(t *testing.T) {
	rows := &mockRows{data: [][]any{}}
	q := &mockQuerier{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return rows, nil
		},
	}
	s := &pgStore{db: q}

	users, total, err := s.ListUsers(t.Context(), "", "", 20, 0)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	if len(users) != 0 {
		t.Errorf("len(users) = %d, want 0", len(users))
	}
}

func TestPgStore_ListUsers_WithStatus(t *testing.T) {
	now := time.Now()
	rows := &mockRows{
		data: [][]any{
			{"u-1", "a@example.com", "alice", "Alice", "active", "user", now, 1},
		},
	}
	q := &mockQuerier{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return rows, nil
		},
	}
	s := &pgStore{db: q}

	users, total, err := s.ListUsers(t.Context(), "", "active", 20, 0)
	if err != nil {
		t.Fatalf("ListUsers() with status error = %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(users) != 1 {
		t.Errorf("len(users) = %d, want 1", len(users))
	}
}

func TestPgStore_ListUsers_WithQuery(t *testing.T) {
	now := time.Now()
	rows := &mockRows{
		data: [][]any{
			{"u-1", "alice@example.com", "alice", "Alice", "active", "user", now, 1},
		},
	}
	q := &mockQuerier{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return rows, nil
		},
	}
	s := &pgStore{db: q}

	users, _, err := s.ListUsers(t.Context(), "alice", "", 20, 0)
	if err != nil {
		t.Fatalf("ListUsers() with query error = %v", err)
	}
	if len(users) != 1 {
		t.Errorf("len(users) = %d, want 1", len(users))
	}
}

func TestPgStore_ListUsers_WithStatusAndQuery(t *testing.T) {
	rows := &mockRows{data: [][]any{}}
	q := &mockQuerier{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return rows, nil
		},
	}
	s := &pgStore{db: q}

	_, _, err := s.ListUsers(t.Context(), "alice", "active", 20, 0)
	if err != nil {
		t.Fatalf("ListUsers() with status+query error = %v", err)
	}
}

func TestPgStore_ListUsers_QueryError(t *testing.T) {
	q := &mockQuerier{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("db error")
		},
	}
	s := &pgStore{db: q}

	_, _, err := s.ListUsers(t.Context(), "", "", 20, 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPgStore_ListUsers_ScanError(t *testing.T) {
	rows := &mockRows{
		data:    [][]any{{"u-1", "a@example.com", "alice", "Alice", "active", "user", time.Now(), 1}},
		scanErr: errors.New("scan error"),
	}
	q := &mockQuerier{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return rows, nil
		},
	}
	s := &pgStore{db: q}

	_, _, err := s.ListUsers(t.Context(), "", "", 20, 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPgStore_ListUsers_RowsError(t *testing.T) {
	rows := &mockRows{
		data:    [][]any{},
		rowsErr: errors.New("rows error"),
	}
	q := &mockQuerier{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return rows, nil
		},
	}
	s := &pgStore{db: q}

	_, _, err := s.ListUsers(t.Context(), "", "", 20, 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- ReactivateUser ---

func TestPgStore_ReactivateUser_Success(t *testing.T) {
	q := &mockQuerier{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	s := &pgStore{db: q}

	if err := s.ReactivateUser(t.Context(), "u-1"); err != nil {
		t.Fatalf("ReactivateUser() error = %v", err)
	}
}

func TestPgStore_ReactivateUser_NotFound(t *testing.T) {
	q := &mockQuerier{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	s := &pgStore{db: q}

	err := s.ReactivateUser(t.Context(), "u-missing")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("error = %v, want ErrUserNotFound", err)
	}
}

func TestPgStore_ReactivateUser_Error(t *testing.T) {
	q := &mockQuerier{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db error")
		},
	}
	s := &pgStore{db: q}

	err := s.ReactivateUser(t.Context(), "u-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- ListChannels ---

func TestPgStore_ListChannels_Success(t *testing.T) {
	now := time.Now()
	rows := &mockRows{
		data: [][]any{
			{"ch-1", "general", "General chat", "u-1", now},
			{"ch-2", "random", "", "", now},
		},
	}
	q := &mockQuerier{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return rows, nil
		},
	}
	s := &pgStore{db: q}

	channels, err := s.ListChannels(t.Context())
	if err != nil {
		t.Fatalf("ListChannels() error = %v", err)
	}
	if len(channels) != 2 {
		t.Fatalf("len(channels) = %d, want 2", len(channels))
	}
	if channels[0].ID != "ch-1" || channels[0].Name != "general" {
		t.Errorf("channels[0] = %+v, want {ID:ch-1 Name:general}", channels[0])
	}
}

func TestPgStore_ListChannels_Empty(t *testing.T) {
	rows := &mockRows{data: [][]any{}}
	q := &mockQuerier{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return rows, nil
		},
	}
	s := &pgStore{db: q}

	channels, err := s.ListChannels(t.Context())
	if err != nil {
		t.Fatalf("ListChannels() error = %v", err)
	}
	if len(channels) != 0 {
		t.Errorf("expected empty slice, got %d channels", len(channels))
	}
}

func TestPgStore_ListChannels_QueryError(t *testing.T) {
	q := &mockQuerier{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("db error")
		},
	}
	s := &pgStore{db: q}

	_, err := s.ListChannels(t.Context())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- DeleteChannel ---

func TestPgStore_DeleteChannel_Success(t *testing.T) {
	q := &mockQuerier{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	s := &pgStore{db: q}

	if err := s.DeleteChannel(t.Context(), "ch-1"); err != nil {
		t.Fatalf("DeleteChannel() error = %v", err)
	}
}

func TestPgStore_DeleteChannel_NotFound(t *testing.T) {
	q := &mockQuerier{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 0"), nil
		},
	}
	s := &pgStore{db: q}

	err := s.DeleteChannel(t.Context(), "ch-missing")
	if !errors.Is(err, ErrChannelNotFound) {
		t.Errorf("error = %v, want ErrChannelNotFound", err)
	}
}

func TestPgStore_DeleteChannel_Error(t *testing.T) {
	q := &mockQuerier{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db error")
		},
	}
	s := &pgStore{db: q}

	if err := s.DeleteChannel(t.Context(), "ch-1"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- HardDeleteUser ---

func TestPgStore_HardDeleteUser_Success(t *testing.T) {
	call := 0
	q := &mockQuerier{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			call++
			if call <= 9 { // 9 DELETE statements
				return pgconn.NewCommandTag("DELETE 1"), nil
			}
			return pgconn.NewCommandTag("UPDATE 1"), nil // anonymize
		},
	}
	s := &pgStore{db: q}

	if err := s.HardDeleteUser(t.Context(), "u-1"); err != nil {
		t.Fatalf("HardDeleteUser() error = %v", err)
	}
}

func TestPgStore_HardDeleteUser_DeleteStmtError(t *testing.T) {
	q := &mockQuerier{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db error")
		},
	}
	s := &pgStore{db: q}

	if err := s.HardDeleteUser(t.Context(), "u-1"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPgStore_HardDeleteUser_AnonymizeError(t *testing.T) {
	call := 0
	q := &mockQuerier{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			call++
			if call <= 9 {
				return pgconn.NewCommandTag("DELETE 1"), nil
			}
			return pgconn.CommandTag{}, errors.New("db error")
		},
	}
	s := &pgStore{db: q}

	if err := s.HardDeleteUser(t.Context(), "u-1"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPgStore_HardDeleteUser_NotFound(t *testing.T) {
	call := 0
	q := &mockQuerier{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			call++
			if call <= 9 {
				return pgconn.NewCommandTag("DELETE 0"), nil
			}
			return pgconn.NewCommandTag("UPDATE 0"), nil // user row not found
		},
	}
	s := &pgStore{db: q}

	err := s.HardDeleteUser(t.Context(), "u-missing")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("error = %v, want ErrUserNotFound", err)
	}
}

// --- SetUserRole ---

func TestPgStore_SetUserRole_Success(t *testing.T) {
	q := &mockQuerier{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	s := &pgStore{db: q}

	if err := s.SetUserRole(t.Context(), "u-1", "admin"); err != nil {
		t.Fatalf("SetUserRole() error = %v", err)
	}
}

func TestPgStore_SetUserRole_NotFound(t *testing.T) {
	q := &mockQuerier{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	s := &pgStore{db: q}

	err := s.SetUserRole(t.Context(), "u-missing", "user")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("error = %v, want ErrUserNotFound", err)
	}
}

func TestPgStore_SetUserRole_Error(t *testing.T) {
	q := &mockQuerier{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db error")
		},
	}
	s := &pgStore{db: q}

	if err := s.SetUserRole(t.Context(), "u-1", "admin"); err == nil {
		t.Fatal("expected error, got nil")
	}
}
