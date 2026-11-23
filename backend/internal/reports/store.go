package reports

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

// reportColumns is the RETURNING/SELECT list scanned by scanReport.
const reportColumns = `id, reporter_id, reported_user_id, message_id, reason, priority,
	COALESCE(description, ''), status, created_at, reviewed_at, reviewed_by`

func scanReport(row pgx.Row) (*Report, error) {
	var r Report
	err := row.Scan(&r.ID, &r.ReporterID, &r.ReportedUserID, &r.MessageID, &r.Reason, &r.Priority,
		&r.Description, &r.Status, &r.CreatedAt, &r.ReviewedAt, &r.ReviewedBy)
	return &r, err
}

// classifyCreateErr maps constraint violations from a report insert onto the
// package's sentinel errors.
func classifyCreateErr(err error, notFound error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound
	}
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		switch {
		case pgErr.Code == "23514" && pgErr.ConstraintName == "no_self_report":
			return ErrSelfReport
		case pgErr.Code == "23505" && pgErr.ConstraintName == "idx_reports_reporter_message":
			return ErrDuplicateReport
		case pgErr.Code == "22P02": // malformed UUID
			return notFound
		}
	}
	return fmt.Errorf("create report: %w", err)
}

// Create files a report against a user and snapshots their identity in the
// same statement, so the record survives a later purge of the account.
func (s *pgStore) Create(ctx context.Context, reporterID, reportedUserID, reason, priority, description string) (*Report, error) {
	r, err := scanReport(s.db.QueryRow(ctx, `
		INSERT INTO reports (reporter_id, reported_user_id, reason, priority, description,
		                     reported_username, reported_display_name, reported_email)
		SELECT $1, u.id, $3, $4, $5, p.username, p.display_name, u.email
		  FROM users u
		  LEFT JOIN profiles p ON p.user_id = u.id
		 WHERE u.id = $2
		RETURNING `+reportColumns,
		reporterID, reportedUserID, reason, priority, description,
	))
	if err != nil {
		return nil, classifyCreateErr(err, ErrUserNotFound)
	}
	return r, nil
}

// CreateForMessage files a report against a message. The sender becomes the
// reported user, and the message plus the sender's identity are copied into
// the report. The membership check mirrors chat's IsMember: channel and public
// rooms are open to everyone, other rooms require a room_members row.
func (s *pgStore) CreateForMessage(ctx context.Context, reporterID, messageID, reason, priority, description string) (*Report, error) {
	r, err := scanReport(s.db.QueryRow(ctx, `
		INSERT INTO reports (reporter_id, reported_user_id, reason, priority, description,
		                     reported_username, reported_display_name, reported_email,
		                     message_id, message_room_id, message_type, message_content, message_sent_at)
		SELECT $1, m.sender_id, $3, $4, $5, p.username, p.display_name, u.email,
		       m.id, m.room_id, m.type, m.content, m.created_at
		  FROM messages m
		  JOIN rooms rm ON rm.id = m.room_id
		  JOIN users u ON u.id = m.sender_id
		  LEFT JOIN profiles p ON p.user_id = m.sender_id
		 WHERE m.id = $2
		   AND m.tombstone IS NOT TRUE
		   AND (rm.type IN ('channel', 'public')
		        OR EXISTS (SELECT 1 FROM room_members mem
		                    WHERE mem.room_id = m.room_id AND mem.user_id = $1))
		RETURNING `+reportColumns,
		reporterID, messageID, reason, priority, description,
	))
	if err != nil {
		return nil, classifyCreateErr(err, ErrMessageNotFound)
	}
	return r, nil
}

// GetByID retrieves a report by ID.
func (s *pgStore) GetByID(ctx context.Context, id string) (*Report, error) {
	r, err := scanReport(s.db.QueryRow(ctx, `SELECT `+reportColumns+` FROM reports WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get report: %w", err)
	}
	return r, nil
}

// List returns reports matching the filter. Ordering is critical → high →
// normal, then newest-first, so admin always sees the worst items at the top
// of the table without needing client-side sorting. Identity fields come from
// the live account unless it was purged or deleted, in which case the
// snapshot taken at report time is used.
func (s *pgStore) List(ctx context.Context, f ListFilter) ([]ReportWithUserInfo, error) {
	query := `
		SELECT r.id, r.reporter_id, r.reported_user_id,
		       COALESCE(u.email, r.reported_email, '') AS reported_email,
		       COALESCE(NULLIF(p.display_name, ''), NULLIF(r.reported_display_name, ''),
		                u.email, r.reported_email, '') AS reported_name,
		       COALESCE(p.username, r.reported_username, '') AS reported_username,
		       COALESCE(p.avatar_url, '') AS reported_avatar,
		       u.id IS NULL AS reported_account_gone,
		       r.message_id, r.message_room_id, r.message_type, r.message_content,
		       r.message_sent_at, r.evidence_purged_at,
		       r.reason, r.priority, COALESCE(r.description, ''), r.status, r.created_at, r.reviewed_at, r.reviewed_by
		FROM reports r
		-- A purged account keeps its users row with placeholder PII, so treat it
		-- like a deleted one and fall back to the snapshot.
		LEFT JOIN users u ON u.id = r.reported_user_id AND u.status <> 'purged'
		LEFT JOIN profiles p ON p.user_id = u.id`

	var conds []string
	var args []any
	if f.Status != "" {
		args = append(args, f.Status)
		conds = append(conds, fmt.Sprintf("r.status = $%d", len(args)))
	}
	if f.Priority != "" {
		args = append(args, f.Priority)
		conds = append(conds, fmt.Sprintf("r.priority = $%d", len(args)))
	}
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += ` ORDER BY CASE r.priority
	                       WHEN 'critical' THEN 0
	                       WHEN 'high'     THEN 1
	                       ELSE                 2
	                    END,
	                    r.created_at DESC`

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list reports: %w", err)
	}
	defer rows.Close()

	results := []ReportWithUserInfo{}
	for rows.Next() {
		var (
			r                       ReportWithUserInfo
			msgID, msgRoom, msgType *string
			msgSentAt               *time.Time
			msg                     ReportedMessage
		)
		if err := rows.Scan(&r.ID, &r.ReporterID, &r.ReportedUserID, &r.ReportedEmail, &r.ReportedName,
			&r.ReportedUsername, &r.ReportedAvatar, &r.ReportedAccountGone,
			&msgID, &msgRoom, &msgType, &msg.Content, &msgSentAt, &msg.PurgedAt,
			&r.Reason, &r.Priority, &r.Description, &r.Status, &r.CreatedAt, &r.ReviewedAt, &r.ReviewedBy); err != nil {
			return nil, fmt.Errorf("scan report: %w", err)
		}
		if msgID != nil {
			msg.ID, msg.RoomID, msg.Type, msg.SentAt = *msgID, *msgRoom, *msgType, *msgSentAt
			r.Message = &msg
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return results, nil
}

// ReportedAccountGone uses the same rule as List's reported_account_gone: a
// hard-deleted account (NULL reference) or a purged one.
func (s *pgStore) ReportedAccountGone(ctx context.Context, id string) (bool, error) {
	var gone bool
	err := s.db.QueryRow(ctx, `
		SELECT u.id IS NULL
		  FROM reports r
		  LEFT JOIN users u ON u.id = r.reported_user_id AND u.status <> 'purged'
		 WHERE r.id = $1`, id,
	).Scan(&gone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, ErrNotFound
		}
		return false, fmt.Errorf("check reported account: %w", err)
	}
	return gone, nil
}

// UpdateStatus updates a report's status and records who reviewed it. A
// dismissal wipes the message copy: evidence is kept only for reports that
// were acted on or are still open.
func (s *pgStore) UpdateStatus(ctx context.Context, id, status, reviewedBy string) (*Report, error) {
	r, err := scanReport(s.db.QueryRow(ctx, `
		UPDATE reports
		   SET status = $2::text, reviewed_at = NOW(), reviewed_by = $3,
		       message_content = CASE WHEN $2::text = 'dismissed' THEN NULL ELSE message_content END,
		       evidence_purged_at = CASE
		           WHEN $2::text = 'dismissed' AND message_id IS NOT NULL
		           THEN COALESCE(evidence_purged_at, NOW())
		           ELSE evidence_purged_at
		       END
		 WHERE id = $1
		RETURNING `+reportColumns,
		id, status, reviewedBy,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update report status: %w", err)
	}
	return r, nil
}
