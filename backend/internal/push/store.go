package push

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// PgStore is the PostgreSQL implementation of Store.
type PgStore struct {
	db querier
}

// NewStore creates a PgStore backed by the given connection pool.
func NewStore(db *pgxpool.Pool) *PgStore {
	return &PgStore{db: db}
}

// Save inserts or updates a push subscription for a user. The UNIQUE constraint
// on endpoint ensures an existing row is replaced rather than duplicated.
func (s *PgStore) Save(ctx context.Context, sub Subscription) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (endpoint) DO UPDATE
		  SET user_id = EXCLUDED.user_id,
		      p256dh  = EXCLUDED.p256dh,
		      auth    = EXCLUDED.auth
	`, sub.UserID, sub.Endpoint, sub.P256DH, sub.Auth)
	return err
}

// Delete removes the subscription identified by endpoint for the given user.
func (s *PgStore) Delete(ctx context.Context, userID, endpoint string) error {
	_, err := s.db.Exec(ctx, `
		DELETE FROM push_subscriptions WHERE user_id = $1 AND endpoint = $2
	`, userID, endpoint)
	return err
}

// ListByUser returns all active subscriptions for a user.
func (s *PgStore) ListByUser(ctx context.Context, userID string) ([]Subscription, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, user_id, endpoint, p256dh, auth
		FROM push_subscriptions
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []Subscription
	for rows.Next() {
		var sub Subscription
		if err := rows.Scan(&sub.ID, &sub.UserID, &sub.Endpoint, &sub.P256DH, &sub.Auth); err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

// DeleteByEndpoint removes a subscription by its endpoint URL, regardless of user.
// Used to clean up expired subscriptions (HTTP 404/410 from the push service).
func (s *PgStore) DeleteByEndpoint(ctx context.Context, endpoint string) error {
	_, err := s.db.Exec(ctx, `
		DELETE FROM push_subscriptions WHERE endpoint = $1
	`, endpoint)
	return err
}

// errNotFound is used internally when a pgx query returns no rows.
var errNotFound = errors.New("not found")

var _ = errNotFound // prevent unused warning — kept for future store methods
