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
		*dest[4].(*string) = ""
		return nil
	}}}}

	p, err := store.GetByUserID(t.Context(), "user-1")
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

	_, err := store.GetByUserID(t.Context(), "user-1")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestPgStore_GetByUserID_QueryError(t *testing.T) {
	store := &pgStore{db: &mockQuerier{row: &mockRow{scanFn: func(_ ...any) error {
		return errors.New("db error")
	}}}}

	_, err := store.GetByUserID(t.Context(), "user-1")
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
		*dest[4].(*string) = ""
		return nil
	}}}}

	p, err := store.Upsert(t.Context(), "user-1", "Alice", "Bio", "")
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

	_, err := store.Upsert(t.Context(), "user-1", "Alice", "Bio", "")
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

// --- CountPhotos ---

func TestPgStore_CountPhotos_Success(t *testing.T) {
	store := &pgStore{db: &mockQuerier{row: &mockRow{scanFn: func(dest ...any) error {
		*dest[0].(*int) = 3
		return nil
	}}}}

	n, err := store.CountPhotos(t.Context(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 3 {
		t.Errorf("count = %d, want 3", n)
	}
}

func TestPgStore_CountPhotos_Error(t *testing.T) {
	store := &pgStore{db: &mockQuerier{row: &mockRow{scanFn: func(_ ...any) error {
		return errors.New("db error")
	}}}}

	_, err := store.CountPhotos(t.Context(), "user-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- AddPhoto ---

func TestPgStore_AddPhoto_Success(t *testing.T) {
	store := &pgStore{db: &mockQuerier{row: &mockRow{scanFn: func(dest ...any) error {
		*dest[0].(*string) = "ph-1"
		*dest[1].(*string) = "https://example.com/1.jpg"
		return nil
	}}}}

	p, err := store.AddPhoto(t.Context(), "user-1", "https://example.com/1.jpg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ID != "ph-1" {
		t.Errorf("ID = %q, want ph-1", p.ID)
	}
}

func TestPgStore_AddPhoto_Error(t *testing.T) {
	store := &pgStore{db: &mockQuerier{row: &mockRow{scanFn: func(_ ...any) error {
		return errors.New("db error")
	}}}}

	_, err := store.AddPhoto(t.Context(), "user-1", "https://example.com/1.jpg")
	if err == nil {
		t.Fatal("expected error, got nil")
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
	err = pool.QueryRow(t.Context(),
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
		p, err := svc.GetMyProfile(t.Context(), userID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.UserID != userID {
			t.Errorf("UserID = %q, want %q", p.UserID, userID)
		}
	})

	t.Run("update and retrieve profile", func(t *testing.T) {
		_, err := svc.UpdateMyProfile(t.Context(), userID, "Alice", "Hello!", "")
		if err != nil {
			t.Fatalf("update error: %v", err)
		}
		p, err := svc.GetMyProfile(t.Context(), userID)
		if err != nil {
			t.Fatalf("get error: %v", err)
		}
		if p.DisplayName != "Alice" {
			t.Errorf("DisplayName = %q, want %q", p.DisplayName, "Alice")
		}
	})

	t.Run("add and delete profile photos", func(t *testing.T) {
		ph, err := svc.AddPhoto(t.Context(), userID, "https://example.com/1.jpg")
		if err != nil {
			t.Fatalf("add photo error: %v", err)
		}
		if ph.ID == "" {
			t.Fatal("expected non-empty photo ID")
		}

		p, err := svc.GetMyProfile(t.Context(), userID)
		if err != nil {
			t.Fatalf("get profile error: %v", err)
		}
		if len(p.Photos) != 1 {
			t.Errorf("photos len = %d, want 1", len(p.Photos))
		}

		if err := svc.DeletePhoto(t.Context(), userID, ph.ID); err != nil {
			t.Fatalf("delete photo error: %v", err)
		}

		p, err = svc.GetMyProfile(t.Context(), userID)
		if err != nil {
			t.Fatalf("get profile after delete error: %v", err)
		}
		if len(p.Photos) != 0 {
			t.Errorf("photos len = %d, want 0 after delete", len(p.Photos))
		}
	})

	t.Run("get public profile", func(t *testing.T) {
		p, err := svc.GetPublicProfile(t.Context(), userID)
		if err != nil {
			t.Fatalf("get public profile error: %v", err)
		}
		if p.UserID != userID {
			t.Errorf("UserID = %q, want %q", p.UserID, userID)
		}
	})
}
