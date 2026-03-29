// Package testutil provides shared helpers for integration tests.
package testutil

import (
	"context"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// OpenDB opens a real PostgreSQL connection pool for integration tests.
// The test is skipped when TEST_DATABASE_URL is not set.
func OpenDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("testutil.OpenDB: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// CreateUser inserts a test user and registers a cleanup to delete it when
// the test finishes. Returns the new user's UUID.
func CreateUser(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(t.Context(),
		`INSERT INTO users (email, password_hash, status)
		 VALUES ($1, 'test-hash', 'active')
		 ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		 RETURNING id`,
		email,
	).Scan(&id)
	if err != nil {
		t.Fatalf("testutil.CreateUser(%q): %v", email, err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) //nolint:errcheck
	})
	return id
}

// NewRedis creates an in-process Redis server and returns a connected client
// and the underlying Miniredis instance (useful for time manipulation).
// Both are closed automatically when the test finishes.
func NewRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() }) //nolint:errcheck
	return rdb, mr
}
