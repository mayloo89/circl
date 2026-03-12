package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

// migrator is the minimal interface we need from golang-migrate.
// Using an interface here allows tests to inject a mock without a real database.
type migrator interface {
	Up() error
	Close() (source error, database error)
}

// newMigrateFn is a variable so tests can replace it with a mock.
var newMigrateFn func(sourceURL, databaseURL string) (migrator, error) = func(s, d string) (migrator, error) {
	return migrate.New(s, d)
}

// Open creates a pgx connection pool and verifies connectivity.
func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return pool, nil
}

// Migrate applies all pending up-migrations.
// Returns nil if there are no pending migrations.
func Migrate(migrationsPath, databaseURL string) error {
	url := buildMigrateURL(databaseURL)

	m, err := newMigrateFn("file://"+migrationsPath, url)
	if err != nil {
		return fmt.Errorf("migration init: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration up: %w", err)
	}

	return nil
}

// buildMigrateURL converts a standard postgres:// URL to the pgx5:// scheme
// expected by golang-migrate's pgx/v5 driver.
func buildMigrateURL(databaseURL string) string {
	url := strings.Replace(databaseURL, "postgres://", "pgx5://", 1)
	return strings.Replace(url, "postgresql://", "pgx5://", 1)
}
