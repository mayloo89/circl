package contacts

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

// SendRequest inserts a pending contact row.
func (s *pgStore) SendRequest(ctx context.Context, requesterID, addresseeID string) (*Contact, error) {
	var c Contact
	err := s.db.QueryRow(ctx, `
		INSERT INTO contacts (requester_id, addressee_id)
		VALUES ($1, $2)
		RETURNING id, requester_id, addressee_id, status, created_at, updated_at`,
		requesterID, addresseeID,
	).Scan(&c.ID, &c.RequesterID, &c.AddresseeID, &c.Status, &c.CreatedAt, &c.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrAlreadyExists
		}
		return nil, fmt.Errorf("send request: %w", err)
	}
	return &c, nil
}

// Accept sets status=accepted if the caller is the addressee.
func (s *pgStore) Accept(ctx context.Context, contactID, addresseeID string) (*Contact, error) {
	var c Contact
	err := s.db.QueryRow(ctx, `
		UPDATE contacts
		SET status = 'accepted', updated_at = NOW()
		WHERE id = $1 AND addressee_id = $2 AND status = 'pending'
		RETURNING id, requester_id, addressee_id, status, created_at, updated_at`,
		contactID, addresseeID,
	).Scan(&c.ID, &c.RequesterID, &c.AddresseeID, &c.Status, &c.CreatedAt, &c.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("accept contact: %w", err)
	}
	return &c, nil
}

// Delete removes the contact row if the caller is either participant.
func (s *pgStore) Delete(ctx context.Context, contactID, userID string) (*Contact, error) {
	var c Contact
	err := s.db.QueryRow(ctx, `
		DELETE FROM contacts
		WHERE id = $1 AND (requester_id = $2 OR addressee_id = $2)
		RETURNING id, requester_id, addressee_id, status, created_at, updated_at`,
		contactID, userID,
	).Scan(&c.ID, &c.RequesterID, &c.AddresseeID, &c.Status, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("delete contact: %w", err)
	}
	return &c, nil
}

// ListAccepted returns all accepted contacts for the given user,
// excluding any user who has a block relationship (in either direction).
func (s *pgStore) ListAccepted(ctx context.Context, userID string) ([]AcceptedContact, error) {
	rows, err := s.db.Query(ctx, `
		SELECT c.id, u.id, COALESCE(p.username, ''), u.email,
		       COALESCE(NULLIF(p.display_name, ''), u.email) AS display_name,
		       COALESCE(p.avatar_url, '') AS avatar_url
		FROM contacts c
		JOIN users u ON u.id = CASE
			WHEN c.requester_id = $1 THEN c.addressee_id
			ELSE c.requester_id
		END
		LEFT JOIN profiles p ON p.user_id = u.id
		WHERE (c.requester_id = $1 OR c.addressee_id = $1)
		  AND c.status = 'accepted'
		  AND NOT EXISTS (
		      SELECT 1 FROM blocks b
		      WHERE (b.blocker_id = $1 AND b.blocked_id = u.id)
		         OR (b.blocker_id = u.id AND b.blocked_id = $1)
		  )
		ORDER BY display_name, u.email`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list accepted: %w", err)
	}
	defer rows.Close()
	return scanAcceptedContacts(rows)
}

// ListPending returns incoming pending contact requests for the given user,
// excluding any requester who has a block relationship (in either direction).
func (s *pgStore) ListPending(ctx context.Context, addresseeID string) ([]PendingRequest, error) {
	rows, err := s.db.Query(ctx, `
		SELECT c.id, u.id, COALESCE(p.username, ''), u.email,
		       COALESCE(NULLIF(p.display_name, ''), u.email) AS display_name,
		       COALESCE(p.avatar_url, '') AS avatar_url
		FROM contacts c
		JOIN users u ON u.id = c.requester_id
		LEFT JOIN profiles p ON p.user_id = u.id
		WHERE c.addressee_id = $1 AND c.status = 'pending'
		  AND NOT EXISTS (
		      SELECT 1 FROM blocks b
		      WHERE (b.blocker_id = $1 AND b.blocked_id = u.id)
		         OR (b.blocker_id = u.id AND b.blocked_id = $1)
		  )
		ORDER BY c.created_at DESC`,
		addresseeID,
	)
	if err != nil {
		return nil, fmt.Errorf("list pending: %w", err)
	}
	defer rows.Close()
	return scanPendingRequests(rows)
}

// ListSent returns outgoing pending contact requests sent by the given user,
// excluding any addressee who has a block relationship (in either direction).
func (s *pgStore) ListSent(ctx context.Context, requesterID string) ([]SentRequest, error) {
	rows, err := s.db.Query(ctx, `
		SELECT c.id, u.id, COALESCE(p.username, ''), u.email,
		       COALESCE(NULLIF(p.display_name, ''), u.email) AS display_name,
		       COALESCE(p.avatar_url, '') AS avatar_url
		FROM contacts c
		JOIN users u ON u.id = c.addressee_id
		LEFT JOIN profiles p ON p.user_id = u.id
		WHERE c.requester_id = $1 AND c.status = 'pending'
		  AND NOT EXISTS (
		      SELECT 1 FROM blocks b
		      WHERE (b.blocker_id = $1 AND b.blocked_id = u.id)
		         OR (b.blocker_id = u.id AND b.blocked_id = $1)
		  )
		ORDER BY c.created_at DESC`,
		requesterID,
	)
	if err != nil {
		return nil, fmt.Errorf("list sent: %w", err)
	}
	defer rows.Close()
	return scanSentRequests(rows)
}

// SearchUsers finds users by email, display_name, or username prefix, excluding
// the caller, any user who already has a contact relationship with the caller,
// and any user who has a block relationship (in either direction) with the caller.
func (s *pgStore) SearchUsers(ctx context.Context, query, excludeUserID string) ([]UserSummary, error) {
	rows, err := s.db.Query(ctx, `
		SELECT u.id, COALESCE(p.username, ''), u.email,
		       COALESCE(NULLIF(p.display_name, ''), u.email) AS display_name,
		       COALESCE(p.avatar_url, '') AS avatar_url
		FROM users u
		LEFT JOIN profiles p ON p.user_id = u.id
		WHERE u.id <> $1
		  AND (u.email ILIKE $2 OR p.display_name ILIKE $2 OR p.username ILIKE $2)
		  AND NOT EXISTS (
		    SELECT 1 FROM contacts c
		    WHERE (c.requester_id = $1 AND c.addressee_id = u.id)
		       OR (c.requester_id = u.id AND c.addressee_id = $1)
		  )
		  AND NOT EXISTS (
		    SELECT 1 FROM blocks b
		    WHERE (b.blocker_id = $1 AND b.blocked_id = u.id)
		       OR (b.blocker_id = u.id AND b.blocked_id = $1)
		  )
		ORDER BY display_name, u.email
		LIMIT 20`,
		excludeUserID, "%"+query+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	defer rows.Close()
	return scanUserSummaries(rows)
}

// Block inserts a block row from blockerID to blockedID.
func (s *pgStore) Block(ctx context.Context, blockerID, blockedID string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO blocks (blocker_id, blocked_id)
		VALUES ($1, $2)`,
		blockerID, blockedID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrAlreadyBlocked
		}
		return fmt.Errorf("block user: %w", err)
	}
	return nil
}

// Unblock removes the block row from blockerID to blockedID.
func (s *pgStore) Unblock(ctx context.Context, blockerID, blockedID string) error {
	tag, err := s.db.Exec(ctx, `
		DELETE FROM blocks
		WHERE blocker_id = $1 AND blocked_id = $2`,
		blockerID, blockedID,
	)
	if err != nil {
		return fmt.Errorf("unblock user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListBlocked returns all users that blockerID has blocked, newest first.
func (s *pgStore) ListBlocked(ctx context.Context, blockerID string) ([]BlockedUser, error) {
	rows, err := s.db.Query(ctx, `
		SELECT b.id, u.id, COALESCE(p.username, ''), u.email,
		       COALESCE(NULLIF(p.display_name, ''), u.email) AS display_name,
		       COALESCE(p.avatar_url, '') AS avatar_url,
		       b.created_at
		FROM blocks b
		JOIN users u ON u.id = b.blocked_id
		LEFT JOIN profiles p ON p.user_id = u.id
		WHERE b.blocker_id = $1
		ORDER BY b.created_at DESC`,
		blockerID,
	)
	if err != nil {
		return nil, fmt.Errorf("list blocked: %w", err)
	}
	defer rows.Close()
	return scanBlockedUsers(rows)
}

// IsBlocked returns true if userA has blocked userB or userB has blocked userA.
func (s *pgStore) IsBlocked(ctx context.Context, userA, userB string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx, `
		SELECT EXISTS(
		    SELECT 1 FROM blocks
		    WHERE (blocker_id = $1 AND blocked_id = $2)
		       OR (blocker_id = $2 AND blocked_id = $1)
		)`,
		userA, userB,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("is blocked: %w", err)
	}
	return exists, nil
}

func scanBlockedUsers(rows pgx.Rows) ([]BlockedUser, error) {
	var results []BlockedUser
	for rows.Next() {
		var u BlockedUser
		if err := rows.Scan(&u.BlockID, &u.UserID, &u.Username, &u.Email, &u.DisplayName, &u.AvatarURL, &u.BlockedAt); err != nil {
			return nil, fmt.Errorf("scan blocked user: %w", err)
		}
		results = append(results, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	if results == nil {
		results = []BlockedUser{}
	}
	return results, nil
}

func scanAcceptedContacts(rows pgx.Rows) ([]AcceptedContact, error) {
	var results []AcceptedContact
	for rows.Next() {
		var c AcceptedContact
		if err := rows.Scan(&c.ContactID, &c.UserID, &c.Username, &c.Email, &c.DisplayName, &c.AvatarURL); err != nil {
			return nil, fmt.Errorf("scan accepted contact: %w", err)
		}
		results = append(results, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	if results == nil {
		results = []AcceptedContact{}
	}
	return results, nil
}

func scanPendingRequests(rows pgx.Rows) ([]PendingRequest, error) {
	var results []PendingRequest
	for rows.Next() {
		var r PendingRequest
		if err := rows.Scan(&r.ContactID, &r.UserID, &r.Username, &r.Email, &r.DisplayName, &r.AvatarURL); err != nil {
			return nil, fmt.Errorf("scan pending request: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	if results == nil {
		results = []PendingRequest{}
	}
	return results, nil
}

func scanSentRequests(rows pgx.Rows) ([]SentRequest, error) {
	var results []SentRequest
	for rows.Next() {
		var r SentRequest
		if err := rows.Scan(&r.ContactID, &r.UserID, &r.Username, &r.Email, &r.DisplayName, &r.AvatarURL); err != nil {
			return nil, fmt.Errorf("scan sent request: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	if results == nil {
		results = []SentRequest{}
	}
	return results, nil
}

func scanUserSummaries(rows pgx.Rows) ([]UserSummary, error) {
	var results []UserSummary
	for rows.Next() {
		var u UserSummary
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.AvatarURL); err != nil {
			return nil, fmt.Errorf("scan user summary: %w", err)
		}
		results = append(results, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	if results == nil {
		results = []UserSummary{}
	}
	return results, nil
}
