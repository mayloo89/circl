package reports

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// querier is the minimal DB interface needed by the store.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// pgxQuerier adapts *pgxpool.Pool to the querier interface.
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

// pgStore is the PostgreSQL implementation of Store.
type pgStore struct{ db querier }

// NewStore returns a Store backed by the given connection pool.
func NewStore(pool *pgxpool.Pool) Store {
	return &pgStore{db: &pgxQuerier{pool}}
}

// Create inserts a new report.
func (s *pgStore) Create(ctx context.Context, reporterID, reportedUserID, reason, description string) (*Report, error) {
	var r Report
	err := s.db.QueryRow(ctx, `
		INSERT INTO reports (reporter_id, reported_user_id, reason, description)
		VALUES ($1, $2, $3, $4)
		RETURNING id, reporter_id, reported_user_id, reason, description, status, created_at, reviewed_at, reviewed_by`,
		reporterID, reportedUserID, reason, description,
	).Scan(&r.ID, &r.ReporterID, &r.ReportedUserID, &r.Reason, &r.Description, &r.Status, &r.CreatedAt, &r.ReviewedAt, &r.ReviewedBy)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" && pgErr.ConstraintName == "no_self_report" {
			return nil, ErrSelfReport
		}
		return nil, fmt.Errorf("create report: %w", err)
	}
	return &r, nil
}

// GetByID retrieves a report by ID.
func (s *pgStore) GetByID(ctx context.Context, id string) (*Report, error) {
	var r Report
	err := s.db.QueryRow(ctx, `
		SELECT id, reporter_id, reported_user_id, reason, description, status, created_at, reviewed_at, reviewed_by
		FROM reports
		WHERE id = $1`,
		id,
	).Scan(&r.ID, &r.ReporterID, &r.ReportedUserID, &r.Reason, &r.Description, &r.Status, &r.CreatedAt, &r.ReviewedAt, &r.ReviewedBy)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get report: %w", err)
	}
	return &r, nil
}

// List returns all reports with optional status filter, newest first.
func (s *pgStore) List(ctx context.Context, status string) ([]ReportWithUserInfo, error) {
	query := `
		SELECT r.id, r.reporter_id, r.reported_user_id,
		       u.email AS reported_email,
		       COALESCE(NULLIF(p.display_name, ''), u.email) AS reported_name,
		       COALESCE(p.avatar_url, '') AS reported_avatar,
		       r.reason, r.description, r.status, r.created_at, r.reviewed_at, r.reviewed_by
		FROM reports r
		JOIN users u ON u.id = r.reported_user_id
		LEFT JOIN profiles p ON p.user_id = u.id`

	var rows pgx.Rows
	var err error

	if status != "" {
		query += ` WHERE r.status = $1 ORDER BY r.created_at DESC`
		rows, err = s.db.Query(ctx, query, status)
	} else {
		query += ` ORDER BY r.created_at DESC`
		rows, err = s.db.Query(ctx, query)
	}

	if err != nil {
		return nil, fmt.Errorf("list reports: %w", err)
	}
	defer rows.Close()

	var results []ReportWithUserInfo
	for rows.Next() {
		var r ReportWithUserInfo
		if err := rows.Scan(&r.ID, &r.ReporterID, &r.ReportedUserID, &r.ReportedEmail, &r.ReportedName, &r.ReportedAvatar,
			&r.Reason, &r.Description, &r.Status, &r.CreatedAt, &r.ReviewedAt, &r.ReviewedBy); err != nil {
			return nil, fmt.Errorf("scan report: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	if results == nil {
		results = []ReportWithUserInfo{}
	}
	return results, nil
}

// UpdateStatus updates a report's status and records who reviewed it.
func (s *pgStore) UpdateStatus(ctx context.Context, id, status, reviewedBy string) (*Report, error) {
	var r Report
	err := s.db.QueryRow(ctx, `
		UPDATE reports
		SET status = $2, reviewed_at = NOW(), reviewed_by = $3
		WHERE id = $1
		RETURNING id, reporter_id, reported_user_id, reason, description, status, created_at, reviewed_at, reviewed_by`,
		id, status, reviewedBy,
	).Scan(&r.ID, &r.ReporterID, &r.ReportedUserID, &r.Reason, &r.Description, &r.Status, &r.CreatedAt, &r.ReviewedAt, &r.ReviewedBy)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update report status: %w", err)
	}
	return &r, nil
}
