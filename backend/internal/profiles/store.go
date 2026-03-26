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

type pgStore struct {
	db   querier
	pool *pgxpool.Pool
}

// NewStore returns a Store backed by the given pgx pool.
func NewStore(pool *pgxpool.Pool) Store {
	return &pgStore{db: &pgxQuerier{pool: pool}, pool: pool}
}

// GetByUserID returns the profile for the given user ID.
func (s *pgStore) GetByUserID(ctx context.Context, userID string) (*Profile, error) {
	row := s.db.QueryRow(ctx,
		`SELECT id, user_id, display_name, bio, COALESCE(avatar_url, '')
		   FROM profiles
		  WHERE user_id = $1`,
		userID,
	)

	var p Profile
	if err := row.Scan(&p.ID, &p.UserID, &p.DisplayName, &p.Bio, &p.AvatarURL); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get profile: %w", err)
	}

	return &p, nil
}

// Upsert inserts or updates the profile for the given user ID.
func (s *pgStore) Upsert(ctx context.Context, userID, displayName, bio, avatarURL string) (*Profile, error) {
	row := s.db.QueryRow(ctx,
		`INSERT INTO profiles (user_id, display_name, bio, avatar_url)
		 VALUES ($1, $2, $3, NULLIF($4, ''))
		 ON CONFLICT (user_id) DO UPDATE
		    SET display_name = EXCLUDED.display_name,
		        bio          = EXCLUDED.bio,
		        avatar_url   = EXCLUDED.avatar_url,
		        updated_at   = now()
		 RETURNING id, user_id, display_name, bio, COALESCE(avatar_url, '')`,
		userID, displayName, bio, avatarURL,
	)

	var p Profile
	if err := row.Scan(&p.ID, &p.UserID, &p.DisplayName, &p.Bio, &p.AvatarURL); err != nil {
		return nil, fmt.Errorf("upsert profile: %w", err)
	}

	return &p, nil
}

// UpdateAvatar updates only the avatar_url for the given user.
func (s *pgStore) UpdateAvatar(ctx context.Context, userID, avatarURL string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE profiles SET avatar_url = NULLIF($2, ''), updated_at = now() WHERE user_id = $1`,
		userID, avatarURL,
	)
	if err != nil {
		return fmt.Errorf("update avatar: %w", err)
	}
	return nil
}

// GetPhotosByUserID returns all showcase photos for a user ordered by position.
func (s *pgStore) GetPhotosByUserID(ctx context.Context, userID string) ([]ProfilePhoto, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, url FROM profile_photos WHERE user_id = $1 ORDER BY position, created_at`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get profile photos: %w", err)
	}
	defer rows.Close()

	var photos []ProfilePhoto
	for rows.Next() {
		var p ProfilePhoto
		if err := rows.Scan(&p.ID, &p.URL); err != nil {
			return nil, fmt.Errorf("scan profile photo: %w", err)
		}
		photos = append(photos, p)
	}
	if photos == nil {
		photos = []ProfilePhoto{}
	}
	return photos, rows.Err()
}

// CountPhotos returns the number of showcase photos for a user.
func (s *pgStore) CountPhotos(ctx context.Context, userID string) (int, error) {
	var count int
	row := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM profile_photos WHERE user_id = $1`,
		userID,
	)
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("count profile photos: %w", err)
	}
	return count, nil
}

// AddPhoto inserts a new showcase photo and returns it.
func (s *pgStore) AddPhoto(ctx context.Context, userID, url string) (*ProfilePhoto, error) {
	row := s.db.QueryRow(ctx,
		`INSERT INTO profile_photos (user_id, url, position)
		 VALUES ($1, $2, (SELECT COALESCE(MAX(position) + 1, 0) FROM profile_photos WHERE user_id = $1))
		 RETURNING id, url`,
		userID, url,
	)
	var p ProfilePhoto
	if err := row.Scan(&p.ID, &p.URL); err != nil {
		return nil, fmt.Errorf("add profile photo: %w", err)
	}
	return &p, nil
}

// DeletePhoto removes a photo, verifying it belongs to the given user.
func (s *pgStore) DeletePhoto(ctx context.Context, photoID, userID string) error {
	result, err := s.pool.Exec(ctx,
		`DELETE FROM profile_photos WHERE id = $1 AND user_id = $2`,
		photoID, userID,
	)
	if err != nil {
		return fmt.Errorf("delete profile photo: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrPhotoNotFound
	}
	return nil
}
