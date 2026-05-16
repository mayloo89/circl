package appeals

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

// NewStore returns a Postgres-backed appeals Store.
func NewStore(pool *pgxpool.Pool) Store {
	return &pgStore{db: &pgxQuerier{pool}}
}

const appealCols = `id, user_id, suspension_id, status, body, expires_at,
	submitted_at, resolved_at, resolved_by, resolution_note, created_at`

func scanAppeal(row pgx.Row) (*Appeal, error) {
	var a Appeal
	err := row.Scan(&a.ID, &a.UserID, &a.SuspensionID, &a.Status, &a.Body, &a.ExpiresAt,
		&a.SubmittedAt, &a.ResolvedAt, &a.ResolvedBy, &a.ResolutionNote, &a.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (s *pgStore) Create(ctx context.Context, userID, suspensionID, tokenHash string, expiresAt time.Time) (*Appeal, error) {
	q := fmt.Sprintf(`
		INSERT INTO appeals (user_id, suspension_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING %s`, appealCols)
	a, err := scanAppeal(s.db.QueryRow(ctx, q, userID, suspensionID, tokenHash, expiresAt))
	if err != nil {
		return nil, fmt.Errorf("create appeal: %w", err)
	}
	return a, nil
}

func (s *pgStore) GetByTokenHash(ctx context.Context, tokenHash string) (*Appeal, error) {
	q := fmt.Sprintf(`SELECT %s FROM appeals WHERE token_hash = $1`, appealCols)
	a, err := scanAppeal(s.db.QueryRow(ctx, q, tokenHash))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get appeal by token: %w", err)
	}
	return a, nil
}

func (s *pgStore) GetByID(ctx context.Context, id string) (*Appeal, error) {
	q := fmt.Sprintf(`SELECT %s FROM appeals WHERE id = $1`, appealCols)
	a, err := scanAppeal(s.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get appeal: %w", err)
	}
	return a, nil
}

func (s *pgStore) Submit(ctx context.Context, id, body string) (*Appeal, error) {
	q := fmt.Sprintf(`
		UPDATE appeals
		   SET body = $2, status = 'submitted', submitted_at = NOW()
		 WHERE id = $1
		   AND status IN ('open', 'submitted')
		 RETURNING %s`, appealCols)
	a, err := scanAppeal(s.db.QueryRow(ctx, q, id, body))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrAlreadyResolved
		}
		return nil, fmt.Errorf("submit appeal: %w", err)
	}
	return a, nil
}

func (s *pgStore) Resolve(ctx context.Context, id, status, resolvedBy, note string) (*Appeal, error) {
	q := fmt.Sprintf(`
		UPDATE appeals
		   SET status = $2, resolved_at = NOW(), resolved_by = $3, resolution_note = $4
		 WHERE id = $1
		   AND status IN ('open', 'submitted')
		 RETURNING %s`, appealCols)
	a, err := scanAppeal(s.db.QueryRow(ctx, q, id, status, resolvedBy, note))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrAlreadyResolved
		}
		return nil, fmt.Errorf("resolve appeal: %w", err)
	}
	return a, nil
}

func (s *pgStore) List(ctx context.Context, status string) ([]AppealWithUserInfo, error) {
	query := `
		SELECT a.id, a.user_id, a.suspension_id, a.status, a.body, a.expires_at,
		       a.submitted_at, a.resolved_at, a.resolved_by, a.resolution_note, a.created_at,
		       u.email,
		       COALESCE(NULLIF(p.display_name, ''), u.email) AS user_name
		FROM appeals a
		JOIN users u ON u.id = a.user_id
		LEFT JOIN profiles p ON p.user_id = u.id`

	var rows pgx.Rows
	var err error
	if status != "" {
		query += ` WHERE a.status = $1 ORDER BY a.created_at DESC`
		rows, err = s.db.Query(ctx, query, status)
	} else {
		query += ` ORDER BY a.created_at DESC`
		rows, err = s.db.Query(ctx, query)
	}
	if err != nil {
		return nil, fmt.Errorf("list appeals: %w", err)
	}
	defer rows.Close()

	results := []AppealWithUserInfo{}
	for rows.Next() {
		var a AppealWithUserInfo
		if err := rows.Scan(&a.ID, &a.UserID, &a.SuspensionID, &a.Status, &a.Body, &a.ExpiresAt,
			&a.SubmittedAt, &a.ResolvedAt, &a.ResolvedBy, &a.ResolutionNote, &a.CreatedAt,
			&a.UserEmail, &a.UserName); err != nil {
			return nil, fmt.Errorf("scan appeal: %w", err)
		}
		results = append(results, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list appeals: %w", err)
	}
	return results, nil
}
