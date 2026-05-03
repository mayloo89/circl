package presence_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/notifications"
	"github.com/mayloo89/circl/backend/internal/presence"
	"github.com/mayloo89/circl/backend/internal/token"
)

const testSecret = "supersecretfortesting-mustbe32chars!!"

// noopNotifier satisfies presence.Notifier without doing anything.
type noopNotifier struct{}

func (noopNotifier) Notify(_ string, _ notifications.Event) {}

// captureNotifier records calls to Notify.
type captureNotifier struct {
	events []notifications.Event
}

func (c *captureNotifier) Notify(_ string, e notifications.Event) {
	c.events = append(c.events, e)
}

// stubStore implements presence.PresenceStore for unit tests.
type stubStore struct {
	heartbeatFn   func(ctx context.Context, userID string) (bool, error)
	offlineFn     func(ctx context.Context, userID string) error
	getPresenceFn func(ctx context.Context, userIDs []string) ([]presence.Info, error)
	contactIDsFn  func(ctx context.Context, userID string) ([]string, error)
}

func (s *stubStore) Heartbeat(ctx context.Context, userID string) (bool, error) {
	if s.heartbeatFn != nil {
		return s.heartbeatFn(ctx, userID)
	}
	return false, nil
}

func (s *stubStore) Offline(ctx context.Context, userID string) error {
	if s.offlineFn != nil {
		return s.offlineFn(ctx, userID)
	}
	return nil
}

func (s *stubStore) GetPresence(ctx context.Context, userIDs []string) ([]presence.Info, error) {
	if s.getPresenceFn != nil {
		return s.getPresenceFn(ctx, userIDs)
	}
	return nil, nil
}

func (s *stubStore) ContactIDs(ctx context.Context, userID string) ([]string, error) {
	if s.contactIDsFn != nil {
		return s.contactIDsFn(ctx, userID)
	}
	return nil, nil
}

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

func newTestStore(t *testing.T) (*presence.Store, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	pool := openTestDB(t)
	return presence.NewStore(rdb, pool), mr
}

func authedReq(r *http.Request, userID string) *http.Request {
	tok, _ := token.Generate(userID, token.RoleUser, testSecret, time.Hour)
	r.Header.Set("Authorization", "Bearer "+tok)
	return r
}

func serveWithAuth(h http.Handler, r *http.Request, rec *httptest.ResponseRecorder) {
	middleware.RequireAuth(testSecret)(h).ServeHTTP(rec, r)
}

// --- Unit-level handler tests (no DB needed) ---

func TestHeartbeat_NoAuth(t *testing.T) {
	h := presence.NewHandler(&stubStore{}, noopNotifier{})

	req := httptest.NewRequest(http.MethodPost, "/heartbeat", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req) // no auth middleware — context has no userID
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestGetPresence_NoAuth(t *testing.T) {
	h := presence.NewHandler(&stubStore{}, noopNotifier{})

	req := httptest.NewRequest(http.MethodGet, "/?ids=u-1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestGetPresence_EmptyIDs(t *testing.T) {
	h := presence.NewHandler(&stubStore{}, noopNotifier{})

	req := authedReq(httptest.NewRequest(http.MethodGet, "/", nil), "u-1")
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// --- Integration tests ---

func TestIntegration_HeartbeatAndPresence(t *testing.T) {
	store, mr := newTestStore(t)
	ctx := t.Context()

	// Create a test user.
	pool := openTestDB(t)
	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, status) VALUES ('presence_test@example.com', 'x', 'active')
		 ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email RETURNING id`,
	).Scan(&userID); err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID) //nolint:errcheck
	})

	// First heartbeat — user just came online.
	justOnline, err := store.Heartbeat(ctx, userID)
	if err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if !justOnline {
		t.Error("first heartbeat should return justOnline=true")
	}

	// Second heartbeat — user was already online.
	justOnline2, err := store.Heartbeat(ctx, userID)
	if err != nil {
		t.Fatalf("second heartbeat: %v", err)
	}
	if justOnline2 {
		t.Error("second heartbeat should return justOnline=false")
	}

	// GetPresence should report online=true.
	infos, err := store.GetPresence(ctx, []string{userID})
	if err != nil {
		t.Fatalf("get presence: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("len = %d, want 1", len(infos))
	}
	if !infos[0].Online {
		t.Error("user should be online after heartbeat")
	}
	if infos[0].LastSeenAt == nil {
		t.Error("last_seen_at should be set after heartbeat")
	}

	// Expire the Redis key — user should now appear offline.
	mr.FastForward(40 * time.Second)

	infos2, err := store.GetPresence(ctx, []string{userID})
	if err != nil {
		t.Fatalf("get presence after expiry: %v", err)
	}
	if infos2[0].Online {
		t.Error("user should be offline after TTL expiry")
	}
	// last_seen_at should still be set from the DB.
	if infos2[0].LastSeenAt == nil {
		t.Error("last_seen_at should still be set after going offline")
	}
}

func TestIntegration_GetPresence_EmptyResult(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := t.Context()

	infos, err := store.GetPresence(ctx, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 0 {
		t.Errorf("len = %d, want 0", len(infos))
	}
}

// --- Presence gating (S-4) ---

// TestGetPresence_NonContactSeesOnlineButNotLastSeen verifies that any authenticated
// user can see whether another user is online, but last_seen_at is only visible to
// accepted contacts.
func TestGetPresence_NonContactSeesOnlineButNotLastSeen(t *testing.T) {
	now := time.Now()
	store := &stubStore{
		getPresenceFn: func(_ context.Context, userIDs []string) ([]presence.Info, error) {
			infos := make([]presence.Info, len(userIDs))
			for i, id := range userIDs {
				infos[i] = presence.Info{UserID: id, Online: true, LastSeenAt: &now}
			}
			return infos, nil
		},
		// contactIDsFn is nil → returns empty slice (caller has no contacts)
	}
	h := presence.NewHandler(store, noopNotifier{})

	req := authedReq(httptest.NewRequest(http.MethodGet, "/?ids=user-2", nil), "user-1")
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !containsStr(body, `"user_id":"user-2"`) {
		t.Error("response should include the requested user_id")
	}
	// Real online status must be visible to any authenticated user.
	if !containsStr(body, `"online":true`) {
		t.Error("non-contact should see real online=true status")
	}
	// last_seen_at must be stripped for non-contacts.
	if containsStr(body, `"last_seen_at"`) {
		t.Error("last_seen_at should not be present for non-contacts")
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && func() bool {
		for i := range len(s) - len(sub) + 1 {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}
