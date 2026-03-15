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

func (r *mockRows) Next() bool                        { r.pos++; return r.pos <= len(r.data) }
func (r *mockRows) Close()                            {}
func (r *mockRows) Err() error                        { return r.rowsErr }
func (r *mockRows) CommandTag() pgconn.CommandTag     { return pgconn.CommandTag{} }
func (r *mockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *mockRows) Values() ([]any, error)            { return nil, nil }
func (r *mockRows) RawValues() [][]byte               { return nil }
func (r *mockRows) Conn() *pgx.Conn                   { return nil }
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
					{"c-1", "u-2", "bob@example.com", "Bob", ""},
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
					{"c-1", "u-2", "alice@example.com", "Alice", ""},
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
					{"c-1", "u-2", "alice@example.com", "Alice", ""},
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

	// Search users.
	results, err := store.SearchUsers(ctx, "ci_c2", u1)
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected search result, got none")
	}

	// Delete contact.
	if _, err := store.Delete(ctx, c.ID, u1); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Second delete must return ErrNotFound.
	if _, err := store.Delete(ctx, c.ID, u1); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: err = %v, want ErrNotFound", err)
	}
}
