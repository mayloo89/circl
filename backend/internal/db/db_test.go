package db

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
)

// --- mock migrator ---

type mockMigrator struct {
	upErr    error
	closeErr error
}

func (m *mockMigrator) Up() error                          { return m.upErr }
func (m *mockMigrator) Close() (source error, db error)   { return m.closeErr, nil }

// --- buildMigrateURL ---

func TestBuildMigrateURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "converts postgres:// scheme",
			input: "postgres://user:pass@localhost:5432/mydb?sslmode=disable",
			want:  "pgx5://user:pass@localhost:5432/mydb?sslmode=disable",
		},
		{
			name:  "converts postgresql:// scheme",
			input: "postgresql://user:pass@localhost:5432/mydb",
			want:  "pgx5://user:pass@localhost:5432/mydb",
		},
		{
			name:  "leaves already-converted URL unchanged",
			input: "pgx5://user:pass@localhost:5432/mydb",
			want:  "pgx5://user:pass@localhost:5432/mydb",
		},
		{
			name:  "handles URL without credentials",
			input: "postgres://localhost:5432/mydb",
			want:  "pgx5://localhost:5432/mydb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildMigrateURL(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// --- Migrate ---

func TestMigrate_InitError(t *testing.T) {
	// Real migrate.New with an invalid URL should return an error.
	err := Migrate("../../migrations", "postgres://invalid-host:5432/nodb?sslmode=disable")
	if err == nil {
		t.Fatal("expected error for invalid DB URL, got nil")
	}
}

func TestMigrate_Success(t *testing.T) {
	orig := newMigrateFn
	t.Cleanup(func() { newMigrateFn = orig })

	newMigrateFn = func(_, _ string) (migrator, error) {
		return &mockMigrator{upErr: nil}, nil
	}

	if err := Migrate("migrations", "postgres://localhost/test"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMigrate_ErrNoChange(t *testing.T) {
	orig := newMigrateFn
	t.Cleanup(func() { newMigrateFn = orig })

	newMigrateFn = func(_, _ string) (migrator, error) {
		return &mockMigrator{upErr: migrate.ErrNoChange}, nil
	}

	// ErrNoChange must be treated as success.
	if err := Migrate("migrations", "postgres://localhost/test"); err != nil {
		t.Errorf("ErrNoChange should not be returned as error, got: %v", err)
	}
}

func TestMigrate_UpError(t *testing.T) {
	orig := newMigrateFn
	t.Cleanup(func() { newMigrateFn = orig })

	upErr := errors.New("dirty migration")
	newMigrateFn = func(_, _ string) (migrator, error) {
		return &mockMigrator{upErr: upErr}, nil
	}

	err := Migrate("migrations", "postgres://localhost/test")
	if err == nil {
		t.Fatal("expected error from Up(), got nil")
	}
}

// --- Open ---

func TestOpen_InvalidURL(t *testing.T) {
	_, err := Open(t.Context(), "not-a-valid-url://foo")
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestOpen_UnreachableHost(t *testing.T) {
	_, err := Open(t.Context(), "postgres://user:pass@localhost:1/nodb?sslmode=disable&connect_timeout=1")
	if err == nil {
		t.Fatal("expected error for unreachable host, got nil")
	}
}

func TestOpen_Integration(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	pool, err := Open(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer pool.Close()

	if pool == nil {
		t.Fatal("expected non-nil pool")
	}
}
