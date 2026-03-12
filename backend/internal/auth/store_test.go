package auth

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// mockQuerier is a test double for the querier interface.
type mockQuerier struct{ row rowScanner }

func (m *mockQuerier) QueryRow(_ context.Context, _ string, _ ...any) rowScanner {
	return m.row
}

// mockRow is a test double for rowScanner.
// scanFn receives the dest pointers and populates them (or returns an error).
type mockRow struct {
	scanFn func(dest ...any) error
}

func (r *mockRow) Scan(dest ...any) error { return r.scanFn(dest...) }

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

	record, err := store.GetUserByEmail(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record.ID != "uuid-1" {
		t.Errorf("ID = %q, want %q", record.ID, "uuid-1")
	}
	if record.Status != "active" {
		t.Errorf("Status = %q, want %q", record.Status, "active")
	}
}

func TestPgStore_GetUserByEmail_NotFound(t *testing.T) {
	store := &pgStore{db: &mockQuerier{
		row: &mockRow{scanFn: func(_ ...any) error { return pgx.ErrNoRows }},
	}}

	_, err := store.GetUserByEmail(context.Background(), "nobody@example.com")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPgStore_GetUserByEmail_QueryError(t *testing.T) {
	store := &pgStore{db: &mockQuerier{
		row: &mockRow{scanFn: func(_ ...any) error { return errors.New("connection reset") }},
	}}

	_, err := store.GetUserByEmail(context.Background(), "user@example.com")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestNewStore(t *testing.T) {
	// Verify NewStore returns a non-nil Store without panicking.
	// A nil pool is intentional here — we're only testing construction.
	store := NewStore((*pgxpool.Pool)(nil))
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

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

	hash, _ := bcrypt.GenerateFromPassword([]byte("testpassword"), bcrypt.MinCost)
	_, err = pool.Exec(context.Background(),
		`INSERT INTO users (email, password_hash, provider, status)
		 VALUES ($1, $2, 'local', 'active')
		 ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash`,
		"store_test@example.com", string(hash),
	)
	if err != nil {
		t.Fatalf("seed test user: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE email = $1`, "store_test@example.com")
	})

	store := NewStore(pool)

	t.Run("returns user when found", func(t *testing.T) {
		svc := NewService(store)
		user, err := svc.Login(context.Background(), "store_test@example.com", "testpassword")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Email != "store_test@example.com" {
			t.Errorf("email = %q, want %q", user.Email, "store_test@example.com")
		}
	})

	t.Run("returns error when user not found", func(t *testing.T) {
		svc := NewService(store)
		_, err := svc.Login(context.Background(), "nobody@example.com", "password")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
