package profiles

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type mockQuerier struct{ row rowScanner }

func (m *mockQuerier) QueryRow(_ context.Context, _ string, _ ...any) rowScanner { return m.row }

type mockRow struct{ scanFn func(dest ...any) error }

func (r *mockRow) Scan(dest ...any) error { return r.scanFn(dest...) }

// --- GetByUserID ---

func TestPgStore_GetByUserID_Success(t *testing.T) {
	store := &pgStore{db: &mockQuerier{row: &mockRow{scanFn: func(dest ...any) error {
		*dest[0].(*string) = "prof-1"
		*dest[1].(*string) = "user-1"
		*dest[2].(*string) = "Alice"
		*dest[3].(*string) = "Bio"
		return nil
	}}}}

	p, err := store.GetByUserID(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.DisplayName != "Alice" {
		t.Errorf("DisplayName = %q, want %q", p.DisplayName, "Alice")
	}
}

func TestPgStore_GetByUserID_NotFound(t *testing.T) {
	store := &pgStore{db: &mockQuerier{row: &mockRow{scanFn: func(_ ...any) error {
		return pgx.ErrNoRows
	}}}}

	_, err := store.GetByUserID(context.Background(), "user-1")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestPgStore_GetByUserID_QueryError(t *testing.T) {
	store := &pgStore{db: &mockQuerier{row: &mockRow{scanFn: func(_ ...any) error {
		return errors.New("db error")
	}}}}

	_, err := store.GetByUserID(context.Background(), "user-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- Upsert ---

func TestPgStore_Upsert_Success(t *testing.T) {
	store := &pgStore{db: &mockQuerier{row: &mockRow{scanFn: func(dest ...any) error {
		*dest[0].(*string) = "prof-1"
		*dest[1].(*string) = "user-1"
		*dest[2].(*string) = "Alice"
		*dest[3].(*string) = "Bio"
		return nil
	}}}}

	p, err := store.Upsert(context.Background(), "user-1", "Alice", "Bio")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.DisplayName != "Alice" {
		t.Errorf("DisplayName = %q, want %q", p.DisplayName, "Alice")
	}
}

func TestPgStore_Upsert_QueryError(t *testing.T) {
	store := &pgStore{db: &mockQuerier{row: &mockRow{scanFn: func(_ ...any) error {
		return errors.New("db error")
	}}}}

	_, err := store.Upsert(context.Background(), "user-1", "Alice", "Bio")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestNewStore(t *testing.T) {
	store := NewStore((*pgxpool.Pool)(nil))
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

// --- Integration ---

func TestProfiles_Integration(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("connect to db: %v", err)
	}
	defer pool.Close()

	// Create a temp user for the test
	var userID string
	err = pool.QueryRow(context.Background(),
		`INSERT INTO users (email, password_hash, provider, status)
		 VALUES ('profile_test@example.com', 'hash', 'local', 'active')
		 ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		 RETURNING id`,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})

	svc := NewService(NewStore(pool))

	t.Run("get creates empty profile when not found", func(t *testing.T) {
		p, err := svc.GetMyProfile(context.Background(), userID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.UserID != userID {
			t.Errorf("UserID = %q, want %q", p.UserID, userID)
		}
	})

	t.Run("update and retrieve profile", func(t *testing.T) {
		_, err := svc.UpdateMyProfile(context.Background(), userID, "Alice", "Hello!")
		if err != nil {
			t.Fatalf("update error: %v", err)
		}
		p, err := svc.GetMyProfile(context.Background(), userID)
		if err != nil {
			t.Fatalf("get error: %v", err)
		}
		if p.DisplayName != "Alice" {
			t.Errorf("DisplayName = %q, want %q", p.DisplayName, "Alice")
		}
	})
}
