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
func (s *pgStore) Delete(ctx context.Context, contactID, userID string) error {
	tag, err := s.db.Exec(ctx, `
		DELETE FROM contacts
		WHERE id = $1 AND (requester_id = $2 OR addressee_id = $2)`,
		contactID, userID,
	)
	if err != nil {
		return fmt.Errorf("delete contact: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListAccepted returns a summary of all accepted contacts for the given user.
func (s *pgStore) ListAccepted(ctx context.Context, userID string) ([]UserSummary, error) {
	rows, err := s.db.Query(ctx, `
		SELECT u.id, u.email, COALESCE(p.display_name, '') AS display_name
		FROM contacts c
		JOIN users u ON u.id = CASE
			WHEN c.requester_id = $1 THEN c.addressee_id
			ELSE c.requester_id
		END
		LEFT JOIN profiles p ON p.user_id = u.id
		WHERE (c.requester_id = $1 OR c.addressee_id = $1)
		  AND c.status = 'accepted'
		ORDER BY display_name, u.email`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list accepted: %w", err)
	}
	defer rows.Close()
	return scanUserSummaries(rows)
}

// ListPending returns incoming pending contact requests for the given user.
func (s *pgStore) ListPending(ctx context.Context, addresseeID string) ([]UserSummary, error) {
	rows, err := s.db.Query(ctx, `
		SELECT u.id, u.email, COALESCE(p.display_name, '') AS display_name
		FROM contacts c
		JOIN users u ON u.id = c.requester_id
		LEFT JOIN profiles p ON p.user_id = u.id
		WHERE c.addressee_id = $1 AND c.status = 'pending'
		ORDER BY c.created_at DESC`,
		addresseeID,
	)
	if err != nil {
		return nil, fmt.Errorf("list pending: %w", err)
	}
	defer rows.Close()
	return scanUserSummaries(rows)
}

// SearchUsers finds users by email or display_name prefix, excluding the caller.
func (s *pgStore) SearchUsers(ctx context.Context, query, excludeUserID string) ([]UserSummary, error) {
	rows, err := s.db.Query(ctx, `
		SELECT u.id, u.email, COALESCE(p.display_name, '') AS display_name
		FROM users u
		LEFT JOIN profiles p ON p.user_id = u.id
		WHERE u.id <> $1
		  AND (u.email ILIKE $2 OR p.display_name ILIKE $2)
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

func scanUserSummaries(rows pgx.Rows) ([]UserSummary, error) {
	var results []UserSummary
	for rows.Next() {
		var u UserSummary
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName); err != nil {
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
