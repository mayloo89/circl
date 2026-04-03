package admin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// rowScanner is implemented by pgx.Row and allows mocking in tests.
type rowScanner interface {
	Scan(dest ...any) error
}

// querier is the minimal DB interface required by pgStore.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) rowScanner
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// pgxQuerier adapts *pgxpool.Pool to the querier interface.
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

// GetUserByID retrieves a user record by primary key.
func (s *pgStore) GetUserByID(ctx context.Context, userID string) (*UserRecord, error) {
	var u UserRecord
	err := s.db.QueryRow(ctx,
		`SELECT id, email, status, is_admin, created_at
		   FROM users
		  WHERE id = $1`,
		userID,
	).Scan(&u.ID, &u.Email, &u.Status, &u.IsAdmin, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

// SetUserStatus updates a user's status field.
func (s *pgStore) SetUserStatus(ctx context.Context, userID, status string) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE users SET status = $2 WHERE id = $1`,
		userID, status,
	)
	if err != nil {
		return fmt.Errorf("set user status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// CreateSuspension inserts a new suspension record.
func (s *pgStore) CreateSuspension(ctx context.Context, userID string, suspendedUntil *time.Time, reason, createdBy string) (*Suspension, error) {
	var sus Suspension
	err := s.db.QueryRow(ctx,
		`INSERT INTO suspensions (user_id, suspended_until, reason, created_by)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, user_id, suspended_until, reason, created_by, created_at`,
		userID, suspendedUntil, reason, createdBy,
	).Scan(&sus.ID, &sus.UserID, &sus.SuspendedUntil, &sus.Reason, &sus.CreatedBy, &sus.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create suspension: %w", err)
	}
	return &sus, nil
}

// IsActiveUser returns true when a user with the given ID and status='active' exists.
func (s *pgStore) IsActiveUser(ctx context.Context, userID string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND status = 'active')`,
		userID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("is active user: %w", err)
	}
	return exists, nil
}
