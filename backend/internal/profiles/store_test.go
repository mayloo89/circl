package profiles

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

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
		*dest[2].(*string) = ""      // username
		*dest[3].(*string) = "Alice" // display_name
		*dest[4].(*string) = "Bio"
		*dest[5].(*string) = ""
		// dest[6] = **time.Time (date_of_birth) — leave nil
		*dest[7].(*string) = "" // gender
		*dest[8].(*string) = "" // location_text
		// dest[9] = **float64 (latitude) — leave nil
		// dest[10] = **float64 (longitude) — leave nil
		*dest[11].(*[]string) = []string{} // interests
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
		*dest[2].(*string) = ""      // username
		*dest[3].(*string) = "Alice" // display_name
		*dest[4].(*string) = "Bio"
		*dest[5].(*string) = ""
		// dest[6] = **time.Time (date_of_birth) — leave nil
		*dest[7].(*string) = ""
		*dest[8].(*string) = ""
		// dest[9] = **float64 — leave nil
		// dest[10] = **float64 — leave nil
		return nil
	}}}}

	p, err := store.Upsert(t.Context(), "user-1", ProfileInput{DisplayName: "Alice", Bio: "Bio"})
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

	_, err := store.Upsert(t.Context(), "user-1", ProfileInput{DisplayName: "Alice", Bio: "Bio"})
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

// --- GetPreferences ---

func TestPgStore_GetPreferences_NotFound(t *testing.T) {
	store := &pgStore{db: &mockQuerier{row: &mockRow{scanFn: func(_ ...any) error {
		return pgx.ErrNoRows
	}}}}

	p, err := store.GetPreferences(t.Context(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.UserID != "user-1" {
		t.Errorf("UserID = %q, want user-1", p.UserID)
	}
	if len(p.GenderPreference) != 0 {
		t.Errorf("GenderPreference = %v, want empty", p.GenderPreference)
	}
}

func TestPgStore_GetPreferences_Error(t *testing.T) {
	store := &pgStore{db: &mockQuerier{row: &mockRow{scanFn: func(_ ...any) error {
		return errors.New("db error")
	}}}}

	_, err := store.GetPreferences(t.Context(), "user-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPgStore_GetPreferences_Success(t *testing.T) {
	minAge := 25
	maxAge := 40
	dist := 100
	store := &pgStore{db: &mockQuerier{row: &mockRow{scanFn: func(dest ...any) error {
		*dest[0].(*string) = "user-1"
		*dest[1].(**int) = &minAge
		*dest[2].(**int) = &maxAge
		*dest[3].(**int) = &dist
		*dest[4].(*[]string) = []string{"female"}
		return nil
	}}}}

	p, err := store.GetPreferences(t.Context(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.MinAge == nil || *p.MinAge != 25 {
		t.Errorf("MinAge = %v, want 25", p.MinAge)
	}
}

// --- UpsertPreferences ---

func TestPgStore_UpsertPreferences_Success(t *testing.T) {
	minAge := 20
	maxAge := 35
	dist := 50
	store := &pgStore{db: &mockQuerier{row: &mockRow{scanFn: func(dest ...any) error {
		*dest[0].(*string) = "user-1"
		*dest[1].(**int) = &minAge
		*dest[2].(**int) = &maxAge
		*dest[3].(**int) = &dist
		*dest[4].(*[]string) = []string{"female"}
		return nil
	}}}}

	p, err := store.UpsertPreferences(t.Context(), "user-1", ProfilePreferences{
		MinAge: &minAge, MaxAge: &maxAge, MaxDistanceKm: &dist, GenderPreference: []string{"female"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.MaxAge == nil || *p.MaxAge != 35 {
		t.Errorf("MaxAge = %v, want 35", p.MaxAge)
	}
}

func TestPgStore_UpsertPreferences_Error(t *testing.T) {
	store := &pgStore{db: &mockQuerier{row: &mockRow{scanFn: func(_ ...any) error {
		return errors.New("db error")
	}}}}

	_, err := store.UpsertPreferences(t.Context(), "user-1", ProfilePreferences{})
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
		if p.Interests == nil {
			t.Error("Interests must not be nil")
		}
	})

	t.Run("update profile with new fields", func(t *testing.T) {
		dob := time.Date(1995, 6, 15, 0, 0, 0, 0, time.UTC)
		_, err := svc.UpdateMyProfile(t.Context(), userID, ProfileInput{
			DisplayName:  "Alice",
			Bio:          "Hello!",
			Gender:       "female",
			DateOfBirth:  &dob,
			LocationText: "Paris",
			Interests:    []string{"hiking", "music"},
		})
		if err != nil {
			t.Fatalf("update error: %v", err)
		}
		p, err := svc.GetMyProfile(t.Context(), userID)
		if err != nil {
			t.Fatalf("get error: %v", err)
		}
		if p.DisplayName != "Alice" {
			t.Errorf("DisplayName = %q, want Alice", p.DisplayName)
		}
		if p.Gender != "female" {
			t.Errorf("Gender = %q, want female", p.Gender)
		}
		if p.LocationText != "Paris" {
			t.Errorf("LocationText = %q, want Paris", p.LocationText)
		}
		if len(p.Interests) != 2 {
			t.Errorf("Interests len = %d, want 2", len(p.Interests))
		}
		if p.DateOfBirth == nil {
			t.Error("DateOfBirth must not be nil")
		}
	})

	t.Run("sync and search interests", func(t *testing.T) {
		store := NewStore(pool)

		if err := store.SyncInterests(t.Context(), userID, []string{"hiking", "music", "travel"}); err != nil {
			t.Fatalf("sync interests: %v", err)
		}

		p, err := store.GetByUserID(t.Context(), userID)
		if err != nil {
			t.Fatalf("get profile: %v", err)
		}
		if len(p.Interests) != 3 {
			t.Errorf("interests len = %d, want 3", len(p.Interests))
		}

		// Replace with a subset — verify old ones are removed
		if err := store.SyncInterests(t.Context(), userID, []string{"hiking"}); err != nil {
			t.Fatalf("sync interests (replace): %v", err)
		}
		p, err = store.GetByUserID(t.Context(), userID)
		if err != nil {
			t.Fatalf("get profile after replace: %v", err)
		}
		if len(p.Interests) != 1 || p.Interests[0] != "hiking" {
			t.Errorf("interests = %v, want [hiking]", p.Interests)
		}

		// Search — "hik" should match "hiking"
		suggestions, err := store.SearchInterests(t.Context(), "hik", 10)
		if err != nil {
			t.Fatalf("search interests: %v", err)
		}
		if len(suggestions) == 0 || suggestions[0].Name != "hiking" {
			t.Errorf("suggestions = %v, want hiking first", suggestions)
		}
		if suggestions[0].Count < 1 {
			t.Errorf("count = %d, want >= 1", suggestions[0].Count)
		}

		// Empty query returns popular interests
		all, err := store.SearchInterests(t.Context(), "", 10)
		if err != nil {
			t.Fatalf("search all interests: %v", err)
		}
		if len(all) == 0 {
			t.Error("expected at least one interest from empty query")
		}

		// Clear interests
		if err := store.SyncInterests(t.Context(), userID, []string{}); err != nil {
			t.Fatalf("clear interests: %v", err)
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

	t.Run("get public profile by ID", func(t *testing.T) {
		p, err := svc.GetPublicProfile(t.Context(), userID)
		if err != nil {
			t.Fatalf("get public profile error: %v", err)
		}
		if p.UserID != userID {
			t.Errorf("UserID = %q, want %q", p.UserID, userID)
		}
	})

	t.Run("username availability and lookup", func(t *testing.T) {
		store := NewStore(pool)

		// Not taken yet
		available, err := store.IsUsernameAvailable(t.Context(), "testuser42")
		if err != nil {
			t.Fatalf("availability check error: %v", err)
		}
		if !available {
			t.Error("expected username to be available")
		}

		// Set username via upsert
		dob := time.Date(1995, 6, 15, 0, 0, 0, 0, time.UTC)
		_, err = store.Upsert(t.Context(), userID, ProfileInput{
			Username: "testuser42", DisplayName: "Alice", DateOfBirth: &dob,
		})
		if err != nil {
			t.Fatalf("upsert with username error: %v", err)
		}

		// Now it should be taken
		available, err = store.IsUsernameAvailable(t.Context(), "testuser42")
		if err != nil {
			t.Fatalf("availability check error: %v", err)
		}
		if available {
			t.Error("expected username to be taken")
		}

		// Lookup by username
		p, err := store.GetByUsername(t.Context(), "testuser42")
		if err != nil {
			t.Fatalf("get by username error: %v", err)
		}
		if p.UserID != userID {
			t.Errorf("UserID = %q, want %q", p.UserID, userID)
		}
		if p.Username != "testuser42" {
			t.Errorf("Username = %q, want testuser42", p.Username)
		}

		// Non-existent username returns ErrNotFound
		_, err = store.GetByUsername(t.Context(), "doesnotexist")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("got %v, want ErrNotFound", err)
		}
	})

	t.Run("upsert and retrieve preferences", func(t *testing.T) {
		minAge := 22
		maxAge := 38
		dist := 100
		_, err := svc.UpdateMyPreferences(t.Context(), userID, ProfilePreferences{
			MinAge: &minAge, MaxAge: &maxAge, MaxDistanceKm: &dist, GenderPreference: []string{"female"},
		})
		if err != nil {
			t.Fatalf("update preferences error: %v", err)
		}
		p, err := svc.GetMyPreferences(t.Context(), userID)
		if err != nil {
			t.Fatalf("get preferences error: %v", err)
		}
		if p.MinAge == nil || *p.MinAge != 22 {
			t.Errorf("MinAge = %v, want 22", p.MinAge)
		}
		if len(p.GenderPreference) != 1 {
			t.Errorf("GenderPreference len = %d, want 1", len(p.GenderPreference))
		}
	})
}

// --- Browse Integration ---

func TestBrowse_Integration(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("connect to db: %v", err)
	}
	defer pool.Close()

	insertUser := func(email string) string {
		var id string
		if err := pool.QueryRow(t.Context(),
			`INSERT INTO users (email, password_hash, provider, status)
			 VALUES ($1, 'hash', 'local', 'active')
			 ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
			 RETURNING id`, email,
		).Scan(&id); err != nil {
			t.Fatalf("insert user %s: %v", email, err)
		}
		t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) })
		return id
	}

	requesterID := insertUser("browse_requester@example.com")
	targetID := insertUser("browse_target@example.com")

	store := NewStore(pool)
	dob := time.Date(1995, 6, 15, 0, 0, 0, 0, time.UTC)

	// Seed both profiles with username + DOB (required for browse)
	if _, err := store.Upsert(t.Context(), requesterID, ProfileInput{Username: "requester_browse", DisplayName: "Req", DateOfBirth: &dob}); err != nil {
		t.Fatalf("upsert requester: %v", err)
	}
	if _, err := store.Upsert(t.Context(), targetID, ProfileInput{Username: "target_browse", DisplayName: "Target", DateOfBirth: &dob}); err != nil {
		t.Fatalf("upsert target: %v", err)
	}

	t.Run("returns other users with username and DOB", func(t *testing.T) {
		results, err := store.Browse(t.Context(), requesterID, 20, 0)
		if err != nil {
			t.Fatalf("browse: %v", err)
		}
		found := false
		for _, p := range results {
			if p.UserID == targetID {
				found = true
				if p.Username != "target_browse" {
					t.Errorf("username = %q, want target_browse", p.Username)
				}
			}
		}
		if !found {
			t.Error("expected target profile in results")
		}
	})

	t.Run("excludes requester from results", func(t *testing.T) {
		results, err := store.Browse(t.Context(), requesterID, 20, 0)
		if err != nil {
			t.Fatalf("browse: %v", err)
		}
		for _, p := range results {
			if p.UserID == requesterID {
				t.Error("requester should not appear in their own browse results")
			}
		}
	})

	t.Run("pagination: limit and offset", func(t *testing.T) {
		// Fetch page 0 with limit 1, then page 1; should not overlap
		page0, err := store.Browse(t.Context(), requesterID, 1, 0)
		if err != nil {
			t.Fatalf("browse page 0: %v", err)
		}
		if len(page0) == 0 {
			t.Skip("no results, skipping pagination test")
		}
		page1, err := store.Browse(t.Context(), requesterID, 1, 1)
		if err != nil {
			t.Fatalf("browse page 1: %v", err)
		}
		if len(page1) > 0 && page0[0].ID == page1[0].ID {
			t.Error("page 0 and page 1 returned the same profile")
		}
	})
}
