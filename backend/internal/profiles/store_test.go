package profiles

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type mockQuerier struct {
	row rowScanner
	// rows lets the test queue distinct rowScanners so callers that exec a
	// write and then read the row back receive different scans. When non-nil
	// it takes precedence over `row`.
	rows []rowScanner
	// lastExecSQL / lastExecArgs capture the most recent Exec call for
	// tests that want to assert on the dynamic SQL output.
	lastExecSQL  string
	lastExecArgs []any
	execErr      error
}

func (m *mockQuerier) QueryRow(_ context.Context, _ string, _ ...any) rowScanner {
	if len(m.rows) > 0 {
		next := m.rows[0]
		m.rows = m.rows[1:]
		return next
	}
	return m.row
}

func (m *mockQuerier) Exec(_ context.Context, sql string, args ...any) error {
	m.lastExecSQL = sql
	m.lastExecArgs = args
	return m.execErr
}

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

// AddPhoto, DeletePhoto, and ReorderPhotos use pool transactions and are
// covered by the integration tests below (they cannot be tested via mockQuerier).

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

// readbackRow returns a mockRow that scans a "fresh" preferences read so
// the post-Exec GetPreferences call inside UpsertPreferences succeeds.
// Tests don't usually need to assert on these values; the goal is to
// exercise the dynamic SQL path.
func readbackRow() *mockRow {
	return &mockRow{scanFn: func(dest ...any) error {
		*dest[0].(*string) = "user-1"
		// dest[1..3] are **int — leave nil
		*dest[4].(*[]string) = []string{}
		*dest[5].(*string) = "es"
		// dest[6..13] are *bool — defaults to false
		return nil
	}}
}

// TestPgStore_UpsertPreferences_LocaleOnlyEmitsSingleColumnWrite proves the
// dynamic SQL only references the columns that were Set — fixing the prior
// REPLACE bug where a locale-only PUT clobbered every other column.
func TestPgStore_UpsertPreferences_LocaleOnlyEmitsSingleColumnWrite(t *testing.T) {
	mq := &mockQuerier{rows: []rowScanner{readbackRow()}}
	store := &pgStore{db: mq}

	loc := "en"
	if _, err := store.UpsertPreferences(t.Context(), "user-1", PreferencesUpdate{Locale: &loc}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sql := mq.lastExecSQL
	if !strings.Contains(sql, "INSERT INTO profile_preferences (user_id, locale)") {
		t.Errorf("expected INSERT to list only user_id and locale, got SQL:\n%s", sql)
	}
	if !strings.Contains(sql, "DO UPDATE SET updated_at = now(), locale = $2") {
		t.Errorf("expected DO UPDATE SET to touch only locale, got SQL:\n%s", sql)
	}
	for _, col := range []string{"min_age", "max_age", "max_distance_km", "gender_preference",
		"hide_distance_from_non_contacts", "hide_presence", "hide_read_receipts", "hide_typing_indicator",
		"notify_chat_messages", "notify_contact_requests", "notify_channel_mentions", "notify_system"} {
		if strings.Contains(sql, col) {
			t.Errorf("locale-only PUT must not mention %s, got SQL:\n%s", col, sql)
		}
	}
	if len(mq.lastExecArgs) != 2 {
		t.Errorf("expected 2 SQL args (user_id, locale), got %d: %v", len(mq.lastExecArgs), mq.lastExecArgs)
	}
}

// TestPgStore_UpsertPreferences_NoFieldsDoNothingClause exercises the
// degenerate case where the caller PUTs an empty body. The store still has
// to insert a row on first call (so column defaults apply) but must not
// rewrite an existing row.
func TestPgStore_UpsertPreferences_NoFieldsDoNothingClause(t *testing.T) {
	mq := &mockQuerier{rows: []rowScanner{readbackRow()}}
	store := &pgStore{db: mq}

	if _, err := store.UpsertPreferences(t.Context(), "user-1", PreferencesUpdate{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(mq.lastExecSQL, "DO NOTHING") {
		t.Errorf("expected DO NOTHING on empty update, got SQL:\n%s", mq.lastExecSQL)
	}
}

// TestPgStore_UpsertPreferences_ExplicitNullClearsFilterInt proves an
// Optional[int] with Set=true and Value=nil produces a SQL write that sets
// the column to NULL — the browse "Clear filters" path.
func TestPgStore_UpsertPreferences_ExplicitNullClearsFilterInt(t *testing.T) {
	mq := &mockQuerier{rows: []rowScanner{readbackRow()}}
	store := &pgStore{db: mq}

	_, err := store.UpsertPreferences(t.Context(), "user-1", PreferencesUpdate{
		MinAge: Optional[int]{Set: true, Value: nil},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(mq.lastExecSQL, "min_age = $2") {
		t.Errorf("expected min_age write in SQL, got:\n%s", mq.lastExecSQL)
	}
	if len(mq.lastExecArgs) != 2 {
		t.Fatalf("expected 2 SQL args, got %d: %v", len(mq.lastExecArgs), mq.lastExecArgs)
	}
	// The arg for min_age must be a *int that is nil (so pgx encodes NULL).
	got, ok := mq.lastExecArgs[1].(*int)
	if !ok {
		t.Fatalf("min_age arg should be *int, got %T", mq.lastExecArgs[1])
	}
	if got != nil {
		t.Errorf("min_age arg should be nil *int (NULL), got %v", got)
	}
}

func TestPgStore_UpsertPreferences_Error(t *testing.T) {
	mq := &mockQuerier{execErr: errors.New("db error")}
	store := &pgStore{db: mq}

	_, err := store.UpsertPreferences(t.Context(), "user-1", PreferencesUpdate{})
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

	t.Run("add mirrors avatar to first photo", func(t *testing.T) {
		store := NewStore(pool)

		ph1, err := svc.AddPhoto(t.Context(), userID, "https://example.com/1.jpg")
		if err != nil {
			t.Fatalf("add first photo: %v", err)
		}
		p, err := store.GetByUserID(t.Context(), userID)
		if err != nil {
			t.Fatalf("get profile: %v", err)
		}
		if p.AvatarURL != "https://example.com/1.jpg" {
			t.Errorf("avatar_url = %q, want first photo URL after add", p.AvatarURL)
		}

		ph2, err := svc.AddPhoto(t.Context(), userID, "https://example.com/2.jpg")
		if err != nil {
			t.Fatalf("add second photo: %v", err)
		}
		p, err = store.GetByUserID(t.Context(), userID)
		if err != nil {
			t.Fatalf("get profile: %v", err)
		}
		if p.AvatarURL != "https://example.com/1.jpg" {
			t.Errorf("avatar_url = %q, want first photo unchanged after adding second", p.AvatarURL)
		}

		// Reorder so photo 2 is first — avatar should update.
		if err := svc.ReorderPhotos(t.Context(), userID, []string{ph2.ID, ph1.ID}); err != nil {
			t.Fatalf("reorder photos: %v", err)
		}
		p, err = store.GetByUserID(t.Context(), userID)
		if err != nil {
			t.Fatalf("get profile: %v", err)
		}
		if p.AvatarURL != "https://example.com/2.jpg" {
			t.Errorf("avatar_url = %q, want second photo URL after reorder", p.AvatarURL)
		}

		// Delete the first (now photo 2 by position) → photo 1 becomes the only one.
		if err := svc.DeletePhoto(t.Context(), userID, ph2.ID); err != nil {
			t.Fatalf("delete photo: %v", err)
		}
		p, err = store.GetByUserID(t.Context(), userID)
		if err != nil {
			t.Fatalf("get profile: %v", err)
		}
		if p.AvatarURL != "https://example.com/1.jpg" {
			t.Errorf("avatar_url = %q, want remaining photo URL after delete", p.AvatarURL)
		}

		// Delete the last photo — avatar should clear.
		if err := svc.DeletePhoto(t.Context(), userID, ph1.ID); err != nil {
			t.Fatalf("delete last photo: %v", err)
		}
		p, err = store.GetByUserID(t.Context(), userID)
		if err != nil {
			t.Fatalf("get profile: %v", err)
		}
		if p.AvatarURL != "" {
			t.Errorf("avatar_url = %q, want empty after deleting all photos", p.AvatarURL)
		}
	})

	t.Run("reorder with wrong count returns ErrInvalidInput", func(t *testing.T) {
		ph, err := svc.AddPhoto(t.Context(), userID, "https://example.com/x.jpg")
		if err != nil {
			t.Fatalf("add photo: %v", err)
		}
		err = svc.ReorderPhotos(t.Context(), userID, []string{ph.ID, "extra-id"})
		if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("got %v, want ErrInvalidInput", err)
		}
		// cleanup
		_ = svc.DeletePhoto(t.Context(), userID, ph.ID)
	})

	t.Run("reorder with duplicate photo IDs returns ErrInvalidInput", func(t *testing.T) {
		ph1, err := svc.AddPhoto(t.Context(), userID, "https://example.com/dup1.jpg")
		if err != nil {
			t.Fatalf("add photo 1: %v", err)
		}
		ph2, err := svc.AddPhoto(t.Context(), userID, "https://example.com/dup2.jpg")
		if err != nil {
			t.Fatalf("add photo 2: %v", err)
		}
		err = svc.ReorderPhotos(t.Context(), userID, []string{ph1.ID, ph1.ID})
		if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("got %v, want ErrInvalidInput", err)
		}
		// cleanup
		_ = svc.DeletePhoto(t.Context(), userID, ph1.ID)
		_ = svc.DeletePhoto(t.Context(), userID, ph2.ID)
	})

	t.Run("reorder with foreign photo ID returns ErrPhotoNotFound", func(t *testing.T) {
		ph, err := svc.AddPhoto(t.Context(), userID, "https://example.com/y.jpg")
		if err != nil {
			t.Fatalf("add photo: %v", err)
		}
		err = svc.ReorderPhotos(t.Context(), userID, []string{"00000000-0000-0000-0000-000000000000"})
		if !errors.Is(err, ErrPhotoNotFound) {
			t.Errorf("got %v, want ErrPhotoNotFound", err)
		}
		// cleanup
		_ = svc.DeletePhoto(t.Context(), userID, ph.ID)
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
		gp := []string{"female"}
		_, err := svc.UpdateMyPreferences(t.Context(), userID, PreferencesUpdate{
			MinAge:           Optional[int]{Set: true, Value: &minAge},
			MaxAge:           Optional[int]{Set: true, Value: &maxAge},
			MaxDistanceKm:    Optional[int]{Set: true, Value: &dist},
			GenderPreference: &gp,
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
		results, err := store.Browse(t.Context(), requesterID, 20, "", false, nil, "seed1", time.Now())
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
		results, err := store.Browse(t.Context(), requesterID, 20, "", false, nil, "seed1", time.Now())
		if err != nil {
			t.Fatalf("browse: %v", err)
		}
		for _, p := range results {
			if p.UserID == requesterID {
				t.Error("requester should not appear in their own browse results")
			}
		}
	})

	t.Run("cursor pagination", func(t *testing.T) {
		// Fetch page 1 with limit 1, extract cursor, fetch page 2; should not overlap.
		asOf := time.Now()
		page1, err := store.Browse(t.Context(), requesterID, 2, "", false, nil, "test-seed", asOf)
		if err != nil {
			t.Fatalf("browse page 1: %v", err)
		}
		if len(page1) < 2 {
			t.Skip("fewer than 2 results, skipping cursor pagination test")
		}
		cursor := EncodeBrowseCursor(page1[0], false, asOf)
		page2, err := store.Browse(t.Context(), requesterID, 1, cursor, false, nil, "test-seed", asOf)
		if err != nil {
			t.Fatalf("browse page 2: %v", err)
		}
		if len(page2) > 0 && page1[0].ID == page2[0].ID {
			t.Error("cursor page returned the same first profile as page 1")
		}
	})

	t.Run("excludes blocked users", func(t *testing.T) {
		// Block the target user
		_, err := pool.Exec(t.Context(),
			`INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1, $2)
			 ON CONFLICT (blocker_id, blocked_id) DO NOTHING`,
			requesterID, targetID,
		)
		if err != nil {
			t.Fatalf("block user: %v", err)
		}
		t.Cleanup(func() {
			pool.Exec(context.Background(), `DELETE FROM blocks WHERE blocker_id = $1 AND blocked_id = $2`, requesterID, targetID)
		})

		// Browse should not return blocked user
		results, err := store.Browse(t.Context(), requesterID, 20, "", false, nil, "seed1", time.Now())
		if err != nil {
			t.Fatalf("browse: %v", err)
		}
		for _, p := range results {
			if p.UserID == targetID {
				t.Error("blocked user should not appear in browse results")
			}
		}
	})

	t.Run("excludes users who blocked requester", func(t *testing.T) {
		// Clean up any existing blocks first
		pool.Exec(t.Context(), `DELETE FROM blocks WHERE blocker_id = $1 OR blocked_id = $1`, requesterID)

		// Have target block the requester (reverse direction)
		_, err := pool.Exec(t.Context(),
			`INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1, $2)
			 ON CONFLICT (blocker_id, blocked_id) DO NOTHING`,
			targetID, requesterID,
		)
		if err != nil {
			t.Fatalf("block user: %v", err)
		}
		t.Cleanup(func() {
			pool.Exec(context.Background(), `DELETE FROM blocks WHERE blocker_id = $1 AND blocked_id = $2`, targetID, requesterID)
		})

		// Browse should not return target (who blocked requester)
		results, err := store.Browse(t.Context(), requesterID, 20, "", false, nil, "seed1", time.Now())
		if err != nil {
			t.Fatalf("browse: %v", err)
		}
		for _, p := range results {
			if p.UserID == targetID {
				t.Error("user who blocked requester should not appear in browse results")
			}
		}
	})
}

// --- Browse distance privacy (integration) ---

func TestBrowseDistancePrivacy_Integration(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}
	pool, err := pgxpool.New(t.Context(), dbURL)
	if err != nil {
		t.Fatalf("connect to db: %v", err)
	}
	// Registered first so it runs after the row cleanups below (LIFO).
	t.Cleanup(pool.Close)
	store := NewStore(pool)
	dob := time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)

	seed := func(name string, lat, lng float64) string {
		var id string
		if err := pool.QueryRow(t.Context(),
			`INSERT INTO users (email, password_hash, provider, status)
			 VALUES ($1, 'hash', 'local', 'active') RETURNING id`, name+"@distance.example.com",
		).Scan(&id); err != nil {
			t.Fatalf("insert user %s: %v", name, err)
		}
		t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) })
		cLat, cLng := SnapToCell(lat, lng)
		if _, err := store.Upsert(t.Context(), id, ProfileInput{
			Username: name, DisplayName: name, DateOfBirth: &dob, Latitude: &cLat, Longitude: &cLng,
		}); err != nil {
			t.Fatalf("upsert %s: %v", name, err)
		}
		return id
	}

	viewer := seed("dist_viewer", -34.6037, -58.3816)
	near := seed("dist_near", -34.6037, -58.3487)     // ~3 km east
	far := seed("dist_far", -34.9214, -57.9545)       // La Plata, ~50 km
	hidden := seed("dist_hidden", -34.6037, -58.3650) // ~1.5 km east, hides distance
	if _, err := store.UpsertPreferences(t.Context(), hidden, PreferencesUpdate{HideDistanceFromNonContacts: new(true)}); err != nil {
		t.Fatalf("hide distance: %v", err)
	}
	ours := map[string]bool{near: true, far: true, hidden: true}

	browse := func(t *testing.T, cursor string, limit int) []BrowseProfile {
		t.Helper()
		all, err := store.Browse(t.Context(), viewer, limit, cursor, true, nil, "seed", time.Now())
		if err != nil {
			t.Fatalf("browse: %v", err)
		}
		var mine []BrowseProfile
		for _, p := range all {
			if ours[p.UserID] {
				mine = append(mine, p)
			}
		}
		return mine
	}

	t.Run("returns coarse bands and hides the hidden candidate's distance", func(t *testing.T) {
		byUser := map[string]BrowseProfile{}
		for _, p := range browse(t, "", 200) {
			byUser[p.UserID] = p
		}
		if d := byUser[near].DistanceKm; d == nil || *d < 2 || *d > 4 {
			t.Errorf("near band = %v, want 2..4", d)
		}
		if d := byUser[far].DistanceKm; d == nil || *d%5 != 0 || *d < 45 || *d > 55 {
			t.Errorf("far band = %v, want a multiple of 5 in 45..55", d)
		}
		if p, ok := byUser[hidden]; !ok || p.DistanceKm != nil {
			t.Errorf("hidden candidate = %+v, want present with nil distance", p)
		}
	})

	t.Run("hidden candidate sorts last, not by position", func(t *testing.T) {
		got := browse(t, "", 200)
		if len(got) != 3 || got[0].UserID != near || got[1].UserID != far || got[2].UserID != hidden {
			ids := make([]string, len(got))
			for i, p := range got {
				ids[i] = p.Username
			}
			t.Errorf("distance order = %v, want [dist_near dist_far dist_hidden]", ids)
		}
	})

	t.Run("cursor carries only the band", func(t *testing.T) {
		first := browse(t, "", 200)[0]
		cur, err := DecodeBrowseCursor(EncodeBrowseCursor(first, true, time.Now()))
		if err != nil {
			t.Fatalf("decode cursor: %v", err)
		}
		if cur.DistanceKm == nil || *cur.DistanceKm != *first.DistanceKm {
			t.Errorf("cursor distance = %v, want band %v", cur.DistanceKm, *first.DistanceKm)
		}
	})

	t.Run("max distance filter excludes the hidden candidate by flag", func(t *testing.T) {
		if _, err := store.UpsertPreferences(t.Context(), viewer, PreferencesUpdate{MaxDistanceKm: Optional[int]{Set: true, Value: new(10)}}); err != nil {
			t.Fatalf("set max distance: %v", err)
		}
		got := browse(t, "", 200)
		if len(got) != 1 || got[0].UserID != near {
			t.Errorf("within 10 km got %d of ours, want only dist_near", len(got))
		}
	})
}

// TestSnapMigrationMatchesGo runs the backfill migration against a row holding
// an exact position and checks it lands on the same cell center as SnapToCell.
func TestSnapMigrationMatchesGo_Integration(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}
	pool, err := pgxpool.New(t.Context(), dbURL)
	if err != nil {
		t.Fatalf("connect to db: %v", err)
	}
	// Registered first so it runs after the row cleanups below (LIFO).
	t.Cleanup(pool.Close)
	migration, err := os.ReadFile("../../migrations/000048_snap_profile_locations.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}

	for i, p := range [][2]float64{{-34.603722, -58.381592}, {64.146582, -21.942635}, {-89.9999, -179.9999}} {
		var id string
		if err := pool.QueryRow(t.Context(),
			`INSERT INTO users (email, password_hash, provider, status)
			 VALUES ($1, 'hash', 'local', 'active') RETURNING id`, fmt.Sprintf("snap_parity_%d@example.com", i),
		).Scan(&id); err != nil {
			t.Fatalf("insert user: %v", err)
		}
		t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) })
		if _, err := pool.Exec(t.Context(),
			`INSERT INTO profiles (user_id, display_name, latitude, longitude) VALUES ($1, 'p', $2, $3)
			 ON CONFLICT (user_id) DO UPDATE SET latitude = EXCLUDED.latitude, longitude = EXCLUDED.longitude`, id, p[0], p[1],
		); err != nil {
			t.Fatalf("insert profile: %v", err)
		}
		if _, err := pool.Exec(t.Context(), string(migration)); err != nil {
			t.Fatalf("run migration: %v", err)
		}
		var lat, lng float64
		if err := pool.QueryRow(t.Context(), `SELECT latitude, longitude FROM profiles WHERE user_id = $1`, id).Scan(&lat, &lng); err != nil {
			t.Fatalf("read back: %v", err)
		}
		if wantLat, wantLng := SnapToCell(p[0], p[1]); lat != wantLat || lng != wantLng {
			t.Errorf("migration snapped (%v, %v) to (%v, %v), Go gives (%v, %v)", p[0], p[1], lat, lng, wantLat, wantLng)
		}
	}
}
