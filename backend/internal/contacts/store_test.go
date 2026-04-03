package contacts

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// --- mock querier ---

type mockRow struct {
	scanFn func(dest ...any) error
}

func (r *mockRow) Scan(dest ...any) error { return r.scanFn(dest...) }

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
		}
	}
	return nil
}

type mockQuerier struct {
	rowFn  func() pgx.Row
	rowsFn func() (pgx.Rows, error)
	execFn func() (pgconn.CommandTag, error)
}

func (m *mockQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return m.rowFn()
}

func (m *mockQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return m.rowsFn()
}

func (m *mockQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return m.execFn()
}

// scanContact is a helper that fills a Contact via Scan dest pointers.
func scanContact(c *Contact) func(dest ...any) error {
	return func(dest ...any) error {
		*dest[0].(*string) = c.ID
		*dest[1].(*string) = c.RequesterID
		*dest[2].(*string) = c.AddresseeID
		*dest[3].(*string) = c.Status
		*dest[4].(*time.Time) = c.CreatedAt
		*dest[5].(*time.Time) = c.UpdatedAt
		return nil
	}
}

// --- unit tests ---

func TestSendRequest_DBSuccess(t *testing.T) {
	want := &Contact{ID: "c-1", RequesterID: "u-1", AddresseeID: "u-2", Status: StatusPending}
	s := &pgStore{db: &mockQuerier{
		rowFn: func() pgx.Row {
			return &mockRow{scanFn: scanContact(want)}
		},
	}}
	got, err := s.SendRequest(t.Context(), "u-1", "u-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "c-1" {
		t.Errorf("ID = %q, want c-1", got.ID)
	}
}

func TestAccept_DBSuccess(t *testing.T) {
	want := &Contact{ID: "c-1", RequesterID: "u-1", AddresseeID: "u-2", Status: StatusAccepted}
	s := &pgStore{db: &mockQuerier{
		rowFn: func() pgx.Row {
			return &mockRow{scanFn: scanContact(want)}
		},
	}}
	got, err := s.Accept(t.Context(), "c-1", "u-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != StatusAccepted {
		t.Errorf("status = %q, want accepted", got.Status)
	}
}

func TestDelete_DBSuccess(t *testing.T) {
	want := &Contact{ID: "c-1", RequesterID: "u-1", AddresseeID: "u-2", Status: StatusAccepted}
	s := &pgStore{db: &mockQuerier{
		rowFn: func() pgx.Row {
			return &mockRow{scanFn: scanContact(want)}
		},
	}}
	got, err := s.Delete(t.Context(), "c-1", "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "c-1" {
		t.Errorf("ID = %q, want c-1", got.ID)
	}
}

func TestSendRequest_DBError(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowFn: func() pgx.Row {
			return &mockRow{scanFn: func(_ ...any) error {
				return errors.New("db error")
			}}
		},
	}}
	_, err := s.SendRequest(t.Context(), "u-1", "u-2")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSendRequest_UniqueViolation(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowFn: func() pgx.Row {
			return &mockRow{scanFn: func(_ ...any) error {
				return &pgconn.PgError{Code: "23505"}
			}}
		},
	}}
	_, err := s.SendRequest(t.Context(), "u-1", "u-2")
	if !errors.Is(err, ErrAlreadyExists) {
		t.Errorf("err = %v, want ErrAlreadyExists", err)
	}
}

func TestAccept_DBError(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowFn: func() pgx.Row {
			return &mockRow{scanFn: func(_ ...any) error {
				return errors.New("db error")
			}}
		},
	}}
	_, err := s.Accept(t.Context(), "c-1", "u-2")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAccept_NotFound(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowFn: func() pgx.Row {
			return &mockRow{scanFn: func(_ ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}}
	_, err := s.Accept(t.Context(), "c-1", "u-2")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDelete_DBError(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowFn: func() pgx.Row {
			return &mockRow{scanFn: func(_ ...any) error {
				return errors.New("db error")
			}}
		},
	}}
	_, err := s.Delete(t.Context(), "c-1", "u-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDelete_NotFound(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowFn: func() pgx.Row {
			return &mockRow{scanFn: func(_ ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}}
	_, err := s.Delete(t.Context(), "c-1", "u-1")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestListAccepted_QueryError(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowsFn: func() (pgx.Rows, error) {
			return nil, errors.New("db error")
		},
	}}
	_, err := s.ListAccepted(t.Context(), "u-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListSent_QueryError(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowsFn: func() (pgx.Rows, error) {
			return nil, errors.New("db error")
		},
	}}
	_, err := s.ListSent(t.Context(), "u-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListSent_Success(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowsFn: func() (pgx.Rows, error) {
			return &mockRows{}, nil
		},
	}}
	results, err := s.ListSent(t.Context(), "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty slice, got %d items", len(results))
	}
}

func TestListSent_WithRows(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowsFn: func() (pgx.Rows, error) {
			return &mockRows{
				data: [][]any{
					{"c-1", "u-2", "", "bob@example.com", "Bob", ""},
				},
			}, nil
		},
	}}
	results, err := s.ListSent(t.Context(), "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ContactID != "c-1" || results[0].Email != "bob@example.com" {
		t.Errorf("unexpected result: %+v", results[0])
	}
}

func TestListSent_ScanError(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowsFn: func() (pgx.Rows, error) {
			return &mockRows{
				data:    [][]any{{"c-1", "u-2", "bob@example.com", "Bob"}},
				scanErr: errors.New("scan error"),
			}, nil
		},
	}}
	_, err := s.ListSent(t.Context(), "u-1")
	if err == nil {
		t.Fatal("expected scan error, got nil")
	}
}

func TestListPending_QueryError(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowsFn: func() (pgx.Rows, error) {
			return nil, errors.New("db error")
		},
	}}
	_, err := s.ListPending(t.Context(), "u-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListPending_WithRows(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowsFn: func() (pgx.Rows, error) {
			return &mockRows{
				data: [][]any{
					{"c-1", "u-2", "", "alice@example.com", "Alice", ""},
				},
			}, nil
		},
	}}
	results, err := s.ListPending(t.Context(), "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ContactID != "c-1" || results[0].Email != "alice@example.com" {
		t.Errorf("unexpected result: %+v", results[0])
	}
}

func TestListPending_ScanError(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowsFn: func() (pgx.Rows, error) {
			return &mockRows{
				data:    [][]any{{"c-1", "u-2", "alice@example.com", "Alice"}},
				scanErr: errors.New("scan error"),
			}, nil
		},
	}}
	_, err := s.ListPending(t.Context(), "u-1")
	if err == nil {
		t.Fatal("expected scan error, got nil")
	}
}

func TestSearchUsers_QueryError(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowsFn: func() (pgx.Rows, error) {
			return nil, errors.New("db error")
		},
	}}
	_, err := s.SearchUsers(t.Context(), "alice", "u-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestScanUserSummaries_ScanError(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowsFn: func() (pgx.Rows, error) {
			return &mockRows{
				data:    [][]any{{"id", "email", "name", "", "avatar"}},
				scanErr: errors.New("scan error"),
			}, nil
		},
	}}
	_, err := s.ListAccepted(t.Context(), "u-1")
	if err == nil {
		t.Fatal("expected scan error, got nil")
	}
}

func TestScanUserSummaries_RowsError(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowsFn: func() (pgx.Rows, error) {
			return &mockRows{rowsErr: errors.New("rows error")}, nil
		},
	}}
	_, err := s.ListAccepted(t.Context(), "u-1")
	if err == nil {
		t.Fatal("expected rows error, got nil")
	}
}

func TestListPending_Success(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowsFn: func() (pgx.Rows, error) {
			return &mockRows{}, nil
		},
	}}
	results, err := s.ListPending(t.Context(), "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty slice, got %d items", len(results))
	}
}

func TestSearchUsers_Success(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowsFn: func() (pgx.Rows, error) {
			return &mockRows{}, nil
		},
	}}
	results, err := s.SearchUsers(t.Context(), "alice", "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty slice, got %d items", len(results))
	}
}

func TestScanUserSummaries_WithRows(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowsFn: func() (pgx.Rows, error) {
			return &mockRows{
				data: [][]any{
					{"c-1", "u-2", "", "alice@example.com", "Alice", ""},
				},
			}, nil
		},
	}}
	results, err := s.ListAccepted(t.Context(), "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if results[0].Email != "alice@example.com" {
		t.Errorf("email = %q, want alice@example.com", results[0].Email)
	}
}

func TestListAccepted_EmptyResult(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowsFn: func() (pgx.Rows, error) {
			return &mockRows{}, nil
		},
	}}
	results, err := s.ListAccepted(t.Context(), "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty slice, got %d items", len(results))
	}
}

// --- Block ---

func TestBlock_DBSuccess(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		execFn: func() (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}}
	if err := s.Block(t.Context(), "u-1", "u-2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBlock_UniqueViolation(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		execFn: func() (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, &pgconn.PgError{Code: "23505"}
		},
	}}
	if err := s.Block(t.Context(), "u-1", "u-2"); !errors.Is(err, ErrAlreadyBlocked) {
		t.Errorf("err = %v, want ErrAlreadyBlocked", err)
	}
}

func TestBlock_DBError(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		execFn: func() (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db error")
		},
	}}
	if err := s.Block(t.Context(), "u-1", "u-2"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUnblock_DBSuccess(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		execFn: func() (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}}
	if err := s.Unblock(t.Context(), "u-1", "u-2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnblock_NotFound(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		execFn: func() (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 0"), nil
		},
	}}
	if err := s.Unblock(t.Context(), "u-1", "u-2"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestUnblock_DBError(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		execFn: func() (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db error")
		},
	}}
	if err := s.Unblock(t.Context(), "u-1", "u-2"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestIsBlocked_True(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowFn: func() pgx.Row {
			return &mockRow{scanFn: func(dest ...any) error {
				*dest[0].(*bool) = true
				return nil
			}}
		},
	}}
	got, err := s.IsBlocked(t.Context(), "u-1", "u-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("expected true, got false")
	}
}

func TestIsBlocked_False(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowFn: func() pgx.Row {
			return &mockRow{scanFn: func(dest ...any) error {
				*dest[0].(*bool) = false
				return nil
			}}
		},
	}}
	got, err := s.IsBlocked(t.Context(), "u-1", "u-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected false, got true")
	}
}

func TestIsBlocked_DBError(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowFn: func() pgx.Row {
			return &mockRow{scanFn: func(_ ...any) error {
				return errors.New("db error")
			}}
		},
	}}
	_, err := s.IsBlocked(t.Context(), "u-1", "u-2")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListBlocked_QueryError(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowsFn: func() (pgx.Rows, error) {
			return nil, errors.New("db error")
		},
	}}
	_, err := s.ListBlocked(t.Context(), "u-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListBlocked_EmptyResult(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowsFn: func() (pgx.Rows, error) {
			return &mockRows{}, nil
		},
	}}
	results, err := s.ListBlocked(t.Context(), "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty slice, got %d", len(results))
	}
}

func TestListBlocked_ScanError(t *testing.T) {
	s := &pgStore{db: &mockQuerier{
		rowsFn: func() (pgx.Rows, error) {
			return &mockRows{
				data:    [][]any{{"b-1", "u-2", "", "bob@example.com", "Bob", "", ""}},
				scanErr: errors.New("scan error"),
			}, nil
		},
	}}
	_, err := s.ListBlocked(t.Context(), "u-1")
	if err == nil {
		t.Fatal("expected scan error, got nil")
	}
}

// --- integration tests ---

func openTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestIntegration_ContactsFlow(t *testing.T) {
	pool := openTestDB(t)
	store := NewStore(pool)
	ctx := t.Context()

	// Create two test users directly via SQL.
	var u1, u2 string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, status) VALUES ('ci_c1@example.com', 'x', 'active') ON CONFLICT (email) DO UPDATE SET email=EXCLUDED.email RETURNING id`,
	).Scan(&u1); err != nil {
		t.Fatalf("create u1: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, status) VALUES ('ci_c2@example.com', 'x', 'active') ON CONFLICT (email) DO UPDATE SET email=EXCLUDED.email RETURNING id`,
	).Scan(&u2); err != nil {
		t.Fatalf("create u2: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id IN ($1,$2)`, u1, u2)
	})

	// Send request.
	c, err := store.SendRequest(ctx, u1, u2)
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if c.Status != StatusPending {
		t.Errorf("status = %q, want %q", c.Status, StatusPending)
	}

	// Duplicate request must fail.
	_, err = store.SendRequest(ctx, u1, u2)
	if !errors.Is(err, ErrAlreadyExists) {
		t.Errorf("duplicate: err = %v, want ErrAlreadyExists", err)
	}

	// List pending for addressee.
	pending, err := store.ListPending(ctx, u2)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(pending) == 0 {
		t.Error("expected pending request, got none")
	}

	// Accept request.
	accepted, err := store.Accept(ctx, c.ID, u2)
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if accepted.Status != StatusAccepted {
		t.Errorf("status = %q, want %q", accepted.Status, StatusAccepted)
	}

	// List accepted contacts.
	contacts, err := store.ListAccepted(ctx, u1)
	if err != nil {
		t.Fatalf("ListAccepted: %v", err)
	}
	if len(contacts) == 0 {
		t.Error("expected accepted contact, got none")
	}

	// Delete contact.
	if _, err := store.Delete(ctx, c.ID, u1); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Search users — contact is gone so u2 must appear in results again.
	results, err := store.SearchUsers(ctx, "ci_c2", u1)
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected search result, got none")
	}

	// Second delete must return ErrNotFound.
	if _, err := store.Delete(ctx, c.ID, u1); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: err = %v, want ErrNotFound", err)
	}
}

func TestIntegration_BlockFlow(t *testing.T) {
	pool := openTestDB(t)
	store := NewStore(pool)
	ctx := t.Context()

	var u1, u2 string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, status) VALUES ('ci_b1@example.com', 'x', 'active') ON CONFLICT (email) DO UPDATE SET email=EXCLUDED.email RETURNING id`,
	).Scan(&u1); err != nil {
		t.Fatalf("create u1: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, status) VALUES ('ci_b2@example.com', 'x', 'active') ON CONFLICT (email) DO UPDATE SET email=EXCLUDED.email RETURNING id`,
	).Scan(&u2); err != nil {
		t.Fatalf("create u2: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id IN ($1,$2)`, u1, u2) //nolint:errcheck
	})

	// Not blocked initially.
	blocked, err := store.IsBlocked(ctx, u1, u2)
	if err != nil {
		t.Fatalf("IsBlocked: %v", err)
	}
	if blocked {
		t.Error("expected not blocked initially")
	}

	// Block u2.
	if err := store.Block(ctx, u1, u2); err != nil {
		t.Fatalf("Block: %v", err)
	}

	// Now blocked.
	blocked, err = store.IsBlocked(ctx, u1, u2)
	if err != nil {
		t.Fatalf("IsBlocked after block: %v", err)
	}
	if !blocked {
		t.Error("expected blocked after Block()")
	}

	// Also blocked in reverse direction.
	blocked, err = store.IsBlocked(ctx, u2, u1)
	if err != nil {
		t.Fatalf("IsBlocked reverse: %v", err)
	}
	if !blocked {
		t.Error("expected blocked in reverse direction")
	}

	// Duplicate block must fail.
	if err := store.Block(ctx, u1, u2); !errors.Is(err, ErrAlreadyBlocked) {
		t.Errorf("duplicate block: err = %v, want ErrAlreadyBlocked", err)
	}

	// List blocked.
	list, err := store.ListBlocked(ctx, u1)
	if err != nil {
		t.Fatalf("ListBlocked: %v", err)
	}
	if len(list) == 0 {
		t.Error("expected blocked user in list, got none")
	}
	if list[0].UserID != u2 {
		t.Errorf("blocked user_id = %q, want %q", list[0].UserID, u2)
	}

	// Blocked user must not appear in search.
	results, err := store.SearchUsers(ctx, "ci_b2", u1)
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected blocked user to be excluded from search, got %d results", len(results))
	}

	// Test contact list exclusion: create an accepted contact, then block.
	// First unblock to create a fresh contact.
	if err := store.Unblock(ctx, u1, u2); err != nil {
		t.Fatalf("Unblock before contact test: %v", err)
	}

	// Create an accepted contact between u1 and u2.
	contact, err := store.SendRequest(ctx, u1, u2)
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if _, err := store.Accept(ctx, contact.ID, u2); err != nil {
		t.Fatalf("Accept: %v", err)
	}

	// Verify u2 appears in u1's accepted contacts.
	accepted, err := store.ListAccepted(ctx, u1)
	if err != nil {
		t.Fatalf("ListAccepted before block: %v", err)
	}
	found := false
	for _, c := range accepted {
		if c.UserID == u2 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected u2 in u1's accepted contacts before blocking")
	}

	// Verify u1 appears in u2's accepted contacts.
	accepted, err = store.ListAccepted(ctx, u2)
	if err != nil {
		t.Fatalf("ListAccepted (u2) before block: %v", err)
	}
	found = false
	for _, c := range accepted {
		if c.UserID == u1 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected u1 in u2's accepted contacts before blocking")
	}

	// Now u1 blocks u2.
	if err := store.Block(ctx, u1, u2); err != nil {
		t.Fatalf("Block after contact: %v", err)
	}

	// u2 should disappear from u1's accepted contacts.
	accepted, err = store.ListAccepted(ctx, u1)
	if err != nil {
		t.Fatalf("ListAccepted after block: %v", err)
	}
	for _, c := range accepted {
		if c.UserID == u2 {
			t.Error("u2 should not appear in u1's accepted contacts after blocking")
		}
	}

	// u1 should also disappear from u2's accepted contacts (bidirectional).
	accepted, err = store.ListAccepted(ctx, u2)
	if err != nil {
		t.Fatalf("ListAccepted (u2) after block: %v", err)
	}
	for _, c := range accepted {
		if c.UserID == u1 {
			t.Error("u1 should not appear in u2's accepted contacts after being blocked (bidirectional)")
		}
	}

	// Test ListPending and ListSent exclusion.
	// Unblock again.
	if err := store.Unblock(ctx, u1, u2); err != nil {
		t.Fatalf("Unblock for pending test: %v", err)
	}

	// Delete the existing contact.
	if _, err := pool.Exec(ctx, `DELETE FROM contacts WHERE (requester_id = $1 AND addressee_id = $2) OR (requester_id = $2 AND addressee_id = $1)`, u1, u2); err != nil {
		t.Fatalf("delete contact: %v", err)
	}

	// u1 sends a new request to u2.
	contact, err = store.SendRequest(ctx, u1, u2)
	if err != nil {
		t.Fatalf("SendRequest for pending test: %v", err)
	}

	// Verify u1 appears in u2's pending requests.
	pending, err := store.ListPending(ctx, u2)
	if err != nil {
		t.Fatalf("ListPending before block: %v", err)
	}
	found = false
	for _, p := range pending {
		if p.UserID == u1 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected u1 in u2's pending requests before blocking")
	}

	// Verify u2 appears in u1's sent requests.
	sent, err := store.ListSent(ctx, u1)
	if err != nil {
		t.Fatalf("ListSent before block: %v", err)
	}
	found = false
	for _, s := range sent {
		if s.UserID == u2 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected u2 in u1's sent requests before blocking")
	}

	// Now u2 blocks u1.
	if err := store.Block(ctx, u2, u1); err != nil {
		t.Fatalf("Block for pending test: %v", err)
	}

	// u1 should disappear from u2's pending requests.
	pending, err = store.ListPending(ctx, u2)
	if err != nil {
		t.Fatalf("ListPending after block: %v", err)
	}
	for _, p := range pending {
		if p.UserID == u1 {
			t.Error("u1 should not appear in u2's pending requests after blocking")
		}
	}

	// u2 should disappear from u1's sent requests (bidirectional).
	sent, err = store.ListSent(ctx, u1)
	if err != nil {
		t.Fatalf("ListSent after block: %v", err)
	}
	for _, s := range sent {
		if s.UserID == u2 {
			t.Error("u2 should not appear in u1's sent requests after blocking (bidirectional)")
		}
	}

	// Unblock.
	if err := store.Unblock(ctx, u1, u2); err != nil {
		t.Fatalf("Unblock: %v", err)
	}

	// Not blocked anymore.
	blocked, err = store.IsBlocked(ctx, u1, u2)
	if err != nil {
		t.Fatalf("IsBlocked after unblock: %v", err)
	}
	if blocked {
		t.Error("expected not blocked after Unblock()")
	}

	// Double unblock must return ErrNotFound.
	if err := store.Unblock(ctx, u1, u2); !errors.Is(err, ErrNotFound) {
		t.Errorf("double unblock: err = %v, want ErrNotFound", err)
	}
}

// TestIntegration_IsBlockedInRoom verifies the batch block check for room scenarios.
func TestIntegration_IsBlockedInRoom(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	store := NewStore(pool)
	ctx := context.Background()

	// Create 4 users: u1, u2, u3, u4.
	var u1, u2, u3, u4 string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, status) VALUES ('isblocked-room-1@example.com', 'x', 'active') ON CONFLICT (email) DO UPDATE SET email=EXCLUDED.email RETURNING id`,
	).Scan(&u1); err != nil {
		t.Fatalf("create u1: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, status) VALUES ('isblocked-room-2@example.com', 'x', 'active') ON CONFLICT (email) DO UPDATE SET email=EXCLUDED.email RETURNING id`,
	).Scan(&u2); err != nil {
		t.Fatalf("create u2: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, status) VALUES ('isblocked-room-3@example.com', 'x', 'active') ON CONFLICT (email) DO UPDATE SET email=EXCLUDED.email RETURNING id`,
	).Scan(&u3); err != nil {
		t.Fatalf("create u3: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, status) VALUES ('isblocked-room-4@example.com', 'x', 'active') ON CONFLICT (email) DO UPDATE SET email=EXCLUDED.email RETURNING id`,
	).Scan(&u4); err != nil {
		t.Fatalf("create u4: %v", err)
	}
	defer pool.Exec(context.Background(), `DELETE FROM users WHERE id IN ($1,$2,$3,$4)`, u1, u2, u3, u4)

	// Test 1: No blocks - should return false.
	blocked, err := store.IsBlockedInRoom(ctx, u1, []string{u2, u3, u4})
	if err != nil {
		t.Fatalf("IsBlockedInRoom no blocks: %v", err)
	}
	if blocked {
		t.Error("expected not blocked when no blocks exist")
	}

	// Test 2: Empty list - should return false.
	blocked, err = store.IsBlockedInRoom(ctx, u1, []string{})
	if err != nil {
		t.Fatalf("IsBlockedInRoom empty list: %v", err)
	}
	if blocked {
		t.Error("expected not blocked with empty otherUserIDs list")
	}

	// Test 3: u2 blocks u1 - should return true.
	if err := store.Block(ctx, u2, u1); err != nil {
		t.Fatalf("Block u2->u1: %v", err)
	}
	blocked, err = store.IsBlockedInRoom(ctx, u1, []string{u2, u3, u4})
	if err != nil {
		t.Fatalf("IsBlockedInRoom after u2 blocks u1: %v", err)
	}
	if !blocked {
		t.Error("expected blocked when u2 blocks u1")
	}

	// Test 4: u1 in different list (doesn't include blocker) - should return false.
	blocked, err = store.IsBlockedInRoom(ctx, u1, []string{u3, u4})
	if err != nil {
		t.Fatalf("IsBlockedInRoom without blocker in list: %v", err)
	}
	if blocked {
		t.Error("expected not blocked when blocker not in otherUserIDs list")
	}

	// Test 5: Bidirectional - u1 blocks u3 - should return true.
	if err := store.Block(ctx, u1, u3); err != nil {
		t.Fatalf("Block u1->u3: %v", err)
	}
	blocked, err = store.IsBlockedInRoom(ctx, u1, []string{u3, u4})
	if err != nil {
		t.Fatalf("IsBlockedInRoom bidirectional u1->u3: %v", err)
	}
	if !blocked {
		t.Error("expected blocked when u1 blocks u3 (bidirectional)")
	}

	// Test 6: Multiple blocks - verify ANY block returns true.
	blocked, err = store.IsBlockedInRoom(ctx, u1, []string{u2, u3, u4})
	if err != nil {
		t.Fatalf("IsBlockedInRoom multiple blocks: %v", err)
	}
	if !blocked {
		t.Error("expected blocked when multiple block relationships exist")
	}

	// Test 7: After unblocking all - should return false.
	if err := store.Unblock(ctx, u2, u1); err != nil {
		t.Fatalf("Unblock u2->u1: %v", err)
	}
	if err := store.Unblock(ctx, u1, u3); err != nil {
		t.Fatalf("Unblock u1->u3: %v", err)
	}
	blocked, err = store.IsBlockedInRoom(ctx, u1, []string{u2, u3, u4})
	if err != nil {
		t.Fatalf("IsBlockedInRoom after unblocking all: %v", err)
	}
	if blocked {
		t.Error("expected not blocked after unblocking all relationships")
	}
}
