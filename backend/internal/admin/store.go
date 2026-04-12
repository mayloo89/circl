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
		`SELECT id, email, status, role, created_at
		   FROM users
		  WHERE id = $1`,
		userID,
	).Scan(&u.ID, &u.Email, &u.Status, &u.Role, &u.CreatedAt)
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
		       u.status, u.role, u.created_at,
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
			&u.Status, &u.Role, &u.CreatedAt, &total); err != nil {
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

// ListChannels returns all public channel rooms ordered by creation date.
func (s *pgStore) ListChannels(ctx context.Context) ([]ChannelRecord, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, name, COALESCE(description, ''), COALESCE(creator_id::text, ''), created_at
		  FROM rooms
		 WHERE type = 'channel'
		 ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("admin list channels: %w", err)
	}
	defer rows.Close()

	var channels []ChannelRecord
	for rows.Next() {
		var c ChannelRecord
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.CreatorID, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("admin list channels scan: %w", err)
		}
		channels = append(channels, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("admin list channels rows: %w", err)
	}
	if channels == nil {
		channels = []ChannelRecord{}
	}
	return channels, nil
}

// DeleteChannel removes a channel room and cascades to its messages.
func (s *pgStore) DeleteChannel(ctx context.Context, channelID string) error {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM rooms WHERE id = $1 AND type = 'channel'`,
		channelID,
	)
	if err != nil {
		return fmt.Errorf("admin delete channel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrChannelNotFound
	}
	return nil
}

// CreateChannel inserts a new public channel room.
func (s *pgStore) CreateChannel(ctx context.Context, adminID, name, description string) (*ChannelRecord, error) {
	var c ChannelRecord
	err := s.db.QueryRow(ctx,
		`INSERT INTO rooms (type, name, description, creator_id)
		 VALUES ('channel', $1, $2, $3)
		 RETURNING id, name, COALESCE(description, ''), COALESCE(creator_id::text, ''), created_at`,
		name, description, adminID,
	).Scan(&c.ID, &c.Name, &c.Description, &c.CreatorID, &c.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrChannelNameTaken
		}
		return nil, fmt.Errorf("admin create channel: %w", err)
	}
	return &c, nil
}

// UpdateChannel changes the name and description of an existing channel.
func (s *pgStore) UpdateChannel(ctx context.Context, channelID, name, description string) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE rooms SET name = $2, description = $3
		  WHERE id = $1 AND type = 'channel'`,
		channelID, name, description,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrChannelNameTaken
		}
		return fmt.Errorf("admin update channel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrChannelNotFound
	}
	return nil
}

// HardDeleteUser immediately purges all user-associated data and anonymizes the
// users row. Unlike the self-delete grace-period flow, this is irreversible.
func (s *pgStore) HardDeleteUser(ctx context.Context, userID string) error {
	stmts := []string{
		`DELETE FROM uploads              WHERE user_id      = $1`,
		`DELETE FROM profile_photos       WHERE user_id      = $1`,
		`DELETE FROM profiles             WHERE user_id      = $1`,
		`DELETE FROM contacts             WHERE requester_id = $1 OR addressee_id = $1`,
		`DELETE FROM push_subscriptions   WHERE user_id      = $1`,
		`DELETE FROM room_members         WHERE user_id      = $1`,
		`DELETE FROM suspensions          WHERE user_id      = $1`,
		`DELETE FROM password_resets      WHERE user_id      = $1`,
		`DELETE FROM email_verifications  WHERE user_id      = $1`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(ctx, stmt, userID); err != nil {
			return fmt.Errorf("hard delete user: %w", err)
		}
	}
	tag, err := s.db.Exec(ctx,
		`UPDATE users
		    SET email         = 'deleted-' || id || '@purged',
		        password_hash = '',
		        status        = 'purged',
		        role          = 'user',
		        deleted_at    = now()
		  WHERE id = $1`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("hard delete user anonymize: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// SetUserRole updates the role column for the given user.
func (s *pgStore) SetUserRole(ctx context.Context, userID, role string) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE users SET role = $2 WHERE id = $1`,
		userID, role,
	)
	if err != nil {
		return fmt.Errorf("set user role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// isUniqueViolation returns true for PostgreSQL unique-constraint errors (code 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
