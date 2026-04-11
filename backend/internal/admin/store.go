package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// pgxQuerier adapts *pgxpool.Pool to the querier interface.
type pgxQuerier struct{ pool *pgxpool.Pool }

func (q *pgxQuerier) QueryRow(ctx context.Context, sql string, args ...any) rowScanner {
	return q.pool.QueryRow(ctx, sql, args...)
}

func (q *pgxQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return q.pool.Query(ctx, sql, args...)
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

// GetStats returns aggregate counts for the admin dashboard.
func (s *pgStore) GetStats(ctx context.Context) (*Stats, error) {
	var st Stats

	err := s.db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE deleted_at IS NULL),
			COUNT(*) FILTER (WHERE status = 'active' AND deleted_at IS NULL),
			COUNT(*) FILTER (WHERE status = 'suspended' AND deleted_at IS NULL),
			COUNT(*) FILTER (WHERE status = 'banned' AND deleted_at IS NULL),
			COUNT(*) FILTER (WHERE deleted_at IS NOT NULL)
		FROM users`,
	).Scan(&st.TotalUsers, &st.ActiveUsers, &st.SuspendedUsers, &st.BannedUsers, &st.DeletedUsers)
	if err != nil {
		return nil, fmt.Errorf("admin stats users: %w", err)
	}

	err = s.db.QueryRow(ctx,
		`SELECT COUNT(*), COUNT(*) FILTER (WHERE status = 'pending') FROM reports`,
	).Scan(&st.TotalReports, &st.PendingReports)
	if err != nil {
		return nil, fmt.Errorf("admin stats reports: %w", err)
	}

	err = s.db.QueryRow(ctx, `SELECT COUNT(*) FROM rooms`).Scan(&st.TotalRooms)
	if err != nil {
		return nil, fmt.Errorf("admin stats rooms: %w", err)
	}

	return &st, nil
}

// ListUsers returns a paginated list of users with optional email/username search and status filter.
func (s *pgStore) ListUsers(ctx context.Context, query, status string, limit, offset int) ([]*UserRecord, int, error) {
	var conds []string
	var args []any
	n := 1

	conds = append(conds, "u.deleted_at IS NULL")

	if status != "" {
		conds = append(conds, fmt.Sprintf("u.status = $%d", n))
		args = append(args, status)
		n++
	}
	if query != "" {
		pat := "%" + query + "%"
		conds = append(conds, fmt.Sprintf("(u.email ILIKE $%d OR p.username ILIKE $%d OR p.display_name ILIKE $%d)", n, n, n))
		args = append(args, pat)
		n++
	}

	where := strings.Join(conds, " AND ")
	sql := fmt.Sprintf(`
		SELECT u.id, u.email,
		       COALESCE(p.username, '') AS username,
		       COALESCE(p.display_name, '') AS display_name,
		       u.status, u.is_admin, u.created_at,
		       COUNT(*) OVER() AS total_count
		FROM users u
		LEFT JOIN profiles p ON p.user_id = u.id
		WHERE %s
		ORDER BY u.created_at DESC
		LIMIT $%d OFFSET $%d`, where, n, n+1)

	args = append(args, limit, offset)

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("admin list users: %w", err)
	}
	defer rows.Close()

	var users []*UserRecord
	var total int
	for rows.Next() {
		var u UserRecord
		if err := rows.Scan(&u.ID, &u.Email, &u.Username, &u.DisplayName,
			&u.Status, &u.IsAdmin, &u.CreatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("admin list users scan: %w", err)
		}
		users = append(users, &u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("admin list users rows: %w", err)
	}
	if users == nil {
		users = []*UserRecord{}
	}
	return users, total, nil
}

// ReactivateUser sets a user's status back to 'active'.
func (s *pgStore) ReactivateUser(ctx context.Context, userID string) error {
	tag, err := s.db.Exec(ctx, `UPDATE users SET status = 'active' WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("admin reactivate user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}
