package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)



// rowScanner is implemented by pgx.Row and allows mocking in tests.
type rowScanner interface {
	Scan(dest ...any) error
}

// querier is the minimal DB interface required by pgStore.
// Keeping it narrow makes it easy to satisfy with a mock in tests.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) rowScanner
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// pgxQuerier adapts *pgxpool.Pool to the querier interface.
// This adapter is needed because pgxpool.Pool.QueryRow returns the
// concrete pgx.Row type, not the rowScanner interface.
type pgxQuerier struct{ pool *pgxpool.Pool }

func (q *pgxQuerier) QueryRow(ctx context.Context, sql string, args ...any) rowScanner {
	return q.pool.QueryRow(ctx, sql, args...)
}

func (q *pgxQuerier) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return q.pool.Exec(ctx, sql, args...)
}

// pgStore implements Store using a querier (backed by pgx in production).
type pgStore struct{ db querier }

// NewStore returns a Store backed by the given pgx pool.
func NewStore(pool *pgxpool.Pool) Store {
	return &pgStore{db: &pgxQuerier{pool: pool}}
}

// GetUserByEmail looks up a user by email and returns the record needed
// for password verification. Returns an error if not found.
func (s *pgStore) GetUserByEmail(ctx context.Context, email string) (*userRecord, error) {
	row := s.db.QueryRow(ctx,
		`SELECT id, email, password_hash, status, is_admin
		   FROM users
		  WHERE email = $1
		  LIMIT 1`,
		email,
	)

	var u userRecord
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Status, &u.IsAdmin); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("query user: %w", err)
	}

	return &u, nil
}

// GetUserByID looks up a user by their UUID. Returns an error if not found.
func (s *pgStore) GetUserByID(ctx context.Context, userID string) (*userRecord, error) {
	row := s.db.QueryRow(ctx,
		`SELECT id, email, password_hash, status, is_admin
		   FROM users
		  WHERE id = $1
		  LIMIT 1`,
		userID,
	)

	var u userRecord
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Status, &u.IsAdmin); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("query user by id: %w", err)
	}

	return &u, nil
}

// UpdatePassword replaces the password hash for the given user.
func (s *pgStore) UpdatePassword(ctx context.Context, userID, newHash string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE users SET password_hash = $1 WHERE id = $2`,
		newHash, userID,
	)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}

// DeleteUser soft-deletes the account by setting status = 'deleted'.
// This prevents future logins while preserving referential integrity.
func (s *pgStore) DeleteUser(ctx context.Context, userID string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE users SET status = 'deleted' WHERE id = $1`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

// CreateUser inserts a new local user and returns the created record.
// Returns ErrEmailTaken if the email is already registered.
func (s *pgStore) CreateUser(ctx context.Context, email, passwordHash string) (*userRecord, error) {
	row := s.db.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, provider, status)
		 VALUES ($1, $2, 'local', 'active')
		 RETURNING id, email, password_hash, status, is_admin`,
		email, passwordHash,
	)

	var u userRecord
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Status, &u.IsAdmin); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	return &u, nil
}
