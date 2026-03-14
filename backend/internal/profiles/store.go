package profiles

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type rowScanner interface {
	Scan(dest ...any) error
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) rowScanner
}

type pgxQuerier struct{ pool *pgxpool.Pool }

func (q *pgxQuerier) QueryRow(ctx context.Context, sql string, args ...any) rowScanner {
	return q.pool.QueryRow(ctx, sql, args...)
}

type pgStore struct{ db querier }

// NewStore returns a Store backed by the given pgx pool.
func NewStore(pool *pgxpool.Pool) Store {
	return &pgStore{db: &pgxQuerier{pool: pool}}
}

// GetByUserID returns the profile for the given user ID.
func (s *pgStore) GetByUserID(ctx context.Context, userID string) (*Profile, error) {
	row := s.db.QueryRow(ctx,
		`SELECT id, user_id, display_name, bio
		   FROM profiles
		  WHERE user_id = $1`,
		userID,
	)

	var p Profile
	if err := row.Scan(&p.ID, &p.UserID, &p.DisplayName, &p.Bio); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get profile: %w", err)
	}

	return &p, nil
}

// Upsert inserts or updates the profile for the given user ID.
func (s *pgStore) Upsert(ctx context.Context, userID, displayName, bio string) (*Profile, error) {
	row := s.db.QueryRow(ctx,
		`INSERT INTO profiles (user_id, display_name, bio)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id) DO UPDATE
		    SET display_name = EXCLUDED.display_name,
		        bio          = EXCLUDED.bio,
		        updated_at   = now()
		 RETURNING id, user_id, display_name, bio`,
		userID, displayName, bio,
	)

	var p Profile
	if err := row.Scan(&p.ID, &p.UserID, &p.DisplayName, &p.Bio); err != nil {
		return nil, fmt.Errorf("upsert profile: %w", err)
	}

	return &p, nil
}
