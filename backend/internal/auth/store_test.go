package auth

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// mockQuerier is a test double for the querier interface.
type mockQuerier struct{ row rowScanner }

func (m *mockQuerier) QueryRow(_ context.Context, _ string, _ ...any) rowScanner {
	return m.row
}

// mockRow is a test double for rowScanner.
type mockRow struct {
	scanFn func(dest ...any) error
}

func (r *mockRow) Scan(dest ...any) error { return r.scanFn(dest...) }

// --- GetUserByEmail ---

func TestPgStore_GetUserByEmail_Success(t *testing.T) {
	store := &pgStore{db: &mockQuerier{
		row: &mockRow{scanFn: func(dest ...any) error {
			*dest[0].(*string) = "uuid-1"
			*dest[1].(*string) = "user@example.com"
			*dest[2].(*string) = "$2a$10$hash"
			*dest[3].(*string) = "active"
			return nil
		}},
	}}

	record, err := store.GetUserByEmail(t.Context(), "user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record.ID != "uuid-1" {
		t.Errorf("ID = %q, want %q", record.ID, "uuid-1")
	}
}

func TestPgStore_GetUserByEmail_NotFound(t *testing.T) {
	store := &pgStore{db: &mockQuerier{
		row: &mockRow{scanFn: func(_ ...any) error { return pgx.ErrNoRows }},
	}}

	_, err := store.GetUserByEmail(t.Context(), "nobody@example.com")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPgStore_GetUserByEmail_QueryError(t *testing.T) {
	store := &pgStore{db: &mockQuerier{
		row: &mockRow{scanFn: func(_ ...any) error { return errors.New("connection reset") }},
	}}

	_, err := store.GetUserByEmail(t.Context(), "user@example.com")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- CreateUser ---

func TestPgStore_CreateUser_Success(t *testing.T) {
	store := &pgStore{db: &mockQuerier{
		row: &mockRow{scanFn: func(dest ...any) error {
			*dest[0].(*string) = "new-uuid"
			*dest[1].(*string) = "new@example.com"
			*dest[2].(*string) = "$2a$10$hash"
			*dest[3].(*string) = "active"
			return nil
		}},
	}}

	record, err := store.CreateUser(t.Context(), "new@example.com", "$2a$10$hash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record.ID != "new-uuid" {
		t.Errorf("ID = %q, want %q", record.ID, "new-uuid")
	}
}

func TestPgStore_CreateUser_EmailTaken(t *testing.T) {
	store := &pgStore{db: &mockQuerier{
		row: &mockRow{scanFn: func(_ ...any) error {
			return &pgconn.PgError{Code: "23505"}
		}},
	}}

	_, err := store.CreateUser(t.Context(), "taken@example.com", "hash")
	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("got %v, want ErrEmailTaken", err)
	}
}

func TestPgStore_CreateUser_QueryError(t *testing.T) {
	store := &pgStore{db: &mockQuerier{
		row: &mockRow{scanFn: func(_ ...any) error { return errors.New("db error") }},
	}}

	_, err := store.CreateUser(t.Context(), "user@example.com", "hash")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- NewStore ---

func TestNewStore(t *testing.T) {
	store := NewStore((*pgxpool.Pool)(nil))
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

// --- Integration ---

func TestStore_Integration(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("connect to db: %v", err)
	}
	defer pool.Close()

	store := NewStore(pool)
	svc := NewService(store)

	const email = "store_integration@example.com"
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE email = $1`, email)
	})

	t.Run("register new user", func(t *testing.T) {
		user, err := svc.Register(t.Context(), email, "securepass")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Email != email {
			t.Errorf("email = %q, want %q", user.Email, email)
		}
	})

	t.Run("login with registered user", func(t *testing.T) {
		user, err := svc.Login(t.Context(), email, "securepass")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Email != email {
			t.Errorf("email = %q, want %q", user.Email, email)
		}
	})

	t.Run("register duplicate email returns ErrEmailTaken", func(t *testing.T) {
		_, err := svc.Register(t.Context(), email, "otherpass")
		if !errors.Is(err, ErrEmailTaken) {
			t.Errorf("got %v, want ErrEmailTaken", err)
		}
	})
}
