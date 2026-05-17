package exports

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type pgxQuerier struct{ pool *pgxpool.Pool }

func (q *pgxQuerier) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return q.pool.QueryRow(ctx, sql, args...)
}
func (q *pgxQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return q.pool.Query(ctx, sql, args...)
}
func (q *pgxQuerier) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return q.pool.Exec(ctx, sql, args...)
}

type pgStore struct{ db querier }

// NewStore returns a Postgres-backed export-request Store.
func NewStore(pool *pgxpool.Pool) Store {
	return &pgStore{db: &pgxQuerier{pool: pool}}
}

const requestCols = `id, user_id, status, storage_key, error, requested_at, completed_at, expires_at, downloaded_at`

func scanRequest(row pgx.Row) (*Request, error) {
	var r Request
	err := row.Scan(&r.ID, &r.UserID, &r.Status, &r.StorageKey, &r.Error,
		&r.RequestedAt, &r.CompletedAt, &r.ExpiresAt, &r.DownloadedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &r, nil
}

// Create inserts a new pending request after the rate-limit check. We do the
// rate check inside one round trip with `WHERE NOT EXISTS (...)` so two
// concurrent requests from the same user can't race past the limiter.
func (s *pgStore) Create(ctx context.Context, userID string) (*Request, error) {
	q := fmt.Sprintf(`
		INSERT INTO export_requests (user_id)
		SELECT $1::uuid
		WHERE NOT EXISTS (
		  SELECT 1 FROM export_requests
		   WHERE user_id = $1::uuid
		     AND requested_at > NOW() - $2::interval
		)
		RETURNING %s`, requestCols)
	rateWindow := fmt.Sprintf("%d seconds", int(RequestRateLimit.Seconds()))
	r, err := scanRequest(s.db.QueryRow(ctx, q, userID, rateWindow))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// The WHERE NOT EXISTS skipped the insert — user is rate-limited.
			return nil, ErrRateLimited
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// Trips the unique partial index — a build is already in flight.
			return nil, ErrAlreadyPending
		}
		return nil, fmt.Errorf("create export request: %w", err)
	}
	return r, nil
}

func (s *pgStore) LatestForUser(ctx context.Context, userID string) (*Request, error) {
	q := fmt.Sprintf(`SELECT %s FROM export_requests WHERE user_id = $1 ORDER BY requested_at DESC LIMIT 1`, requestCols)
	return scanRequest(s.db.QueryRow(ctx, q, userID))
}

func (s *pgStore) GetByID(ctx context.Context, id string) (*Request, error) {
	q := fmt.Sprintf(`SELECT %s FROM export_requests WHERE id = $1`, requestCols)
	return scanRequest(s.db.QueryRow(ctx, q, id))
}

func (s *pgStore) GetByTokenHash(ctx context.Context, tokenHash string) (*Request, error) {
	q := fmt.Sprintf(`SELECT %s FROM export_requests WHERE token_hash = $1`, requestCols)
	return scanRequest(s.db.QueryRow(ctx, q, tokenHash))
}

func (s *pgStore) MarkProcessing(ctx context.Context, id string) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE export_requests SET status = 'processing' WHERE id = $1 AND status = 'pending'`,
		id)
	if err != nil {
		return fmt.Errorf("mark processing: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgStore) MarkReady(ctx context.Context, id, storageKey, tokenHash string, expiresAt time.Time) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE export_requests
		    SET status = 'ready',
		        storage_key = $2,
		        token_hash = $3,
		        completed_at = NOW(),
		        expires_at = $4
		  WHERE id = $1
		    AND status IN ('pending','processing')`,
		id, storageKey, tokenHash, expiresAt)
	if err != nil {
		return fmt.Errorf("mark ready: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgStore) MarkFailed(ctx context.Context, id, errMsg string) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE export_requests
		    SET status = 'failed', error = $2, completed_at = NOW()
		  WHERE id = $1
		    AND status IN ('pending','processing')`,
		id, errMsg)
	if err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgStore) MarkDownloaded(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE export_requests SET downloaded_at = COALESCE(downloaded_at, NOW()) WHERE id = $1`,
		id)
	if err != nil {
		return fmt.Errorf("mark downloaded: %w", err)
	}
	return nil
}
