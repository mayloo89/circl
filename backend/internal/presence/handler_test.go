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
	tok, _ := token.Generate(userID, false, testSecret, time.Hour)
	r.Header.Set("Authorization", "Bearer "+tok)
	return r
}

func serveWithAuth(h http.Handler, r *http.Request, rec *httptest.ResponseRecorder) {
	middleware.RequireAuth(testSecret)(h).ServeHTTP(rec, r)
}

// --- Unit-level handler tests (no DB needed) ---

func TestHeartbeat_NoAuth(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	// Use a nil pool — handler should reject before touching DB.
	store := presence.NewStore(rdb, nil)
	h := presence.NewHandler(store, noopNotifier{})

	req := httptest.NewRequest(http.MethodPost, "/heartbeat", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req) // no auth middleware — context has no userID
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestGetPresence_NoAuth(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	store := presence.NewStore(rdb, nil)
	h := presence.NewHandler(store, noopNotifier{})

	req := httptest.NewRequest(http.MethodGet, "/?ids=u-1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestGetPresence_EmptyIDs(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	store := presence.NewStore(rdb, nil)
	h := presence.NewHandler(store, noopNotifier{})

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
