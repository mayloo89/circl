package albums

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgStore struct {
	db *pgxpool.Pool
}

// NewStore returns a Postgres-backed Store.
func NewStore(pool *pgxpool.Pool) Store {
	return &pgStore{db: pool}
}

func (s *pgStore) CreateAlbum(ctx context.Context, ownerID, name, description string) (*Album, error) {
	var a Album
	err := s.db.QueryRow(ctx, `
		INSERT INTO private_albums (owner_id, name, description)
		VALUES ($1, $2, $3)
		RETURNING id, owner_id, name, description, photo_count, created_at, updated_at`,
		ownerID, name, description,
	).Scan(&a.ID, &a.OwnerID, &a.Name, &a.Description, &a.PhotoCount, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("albums: create: %w", err)
	}
	return &a, nil
}

func (s *pgStore) GetAlbum(ctx context.Context, albumID string) (*Album, error) {
	var a Album
	err := s.db.QueryRow(ctx, `
		SELECT id, owner_id, name, description, photo_count, created_at, updated_at
		  FROM private_albums
		 WHERE id = $1`, albumID,
	).Scan(&a.ID, &a.OwnerID, &a.Name, &a.Description, &a.PhotoCount, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("albums: get: %w", err)
	}
	return &a, nil
}

func (s *pgStore) ListAlbumsByOwner(ctx context.Context, ownerID string) ([]Album, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, owner_id, name, description, photo_count, created_at, updated_at
		  FROM private_albums
		 WHERE owner_id = $1
		 ORDER BY created_at DESC`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("albums: list by owner: %w", err)
	}
	defer rows.Close()
	return scanAlbums(rows)
}

func (s *pgStore) ListAlbumsSharedWith(ctx context.Context, granteeID string) ([]Album, error) {
	rows, err := s.db.Query(ctx, `
		SELECT a.id, a.owner_id, a.name, a.description, a.photo_count, a.created_at, a.updated_at,
		       COALESCE(p.display_name, p.username, ''),
		       COALESCE(p.avatar_url, '')
		  FROM private_albums a
		  JOIN private_album_grants g ON g.album_id = a.id
		  LEFT JOIN profiles p ON p.user_id = a.owner_id
		 WHERE g.grantee_id = $1
		   AND g.status = 'active'
		   AND (g.expires_at IS NULL OR g.expires_at > NOW())
		 ORDER BY g.granted_at DESC NULLS LAST, a.created_at DESC`, granteeID)
	if err != nil {
		return nil, fmt.Errorf("albums: list shared: %w", err)
	}
	defer rows.Close()
	out := []Album{}
	for rows.Next() {
		var a Album
		if err := rows.Scan(&a.ID, &a.OwnerID, &a.Name, &a.Description, &a.PhotoCount, &a.CreatedAt, &a.UpdatedAt, &a.OwnerName, &a.OwnerAvatarURL); err != nil {
			return nil, fmt.Errorf("albums: scan shared: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *pgStore) UpdateAlbum(ctx context.Context, albumID, name, description string) (*Album, error) {
	var a Album
	err := s.db.QueryRow(ctx, `
		UPDATE private_albums
		   SET name = $2,
		       description = $3,
		       updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, owner_id, name, description, photo_count, created_at, updated_at`,
		albumID, name, description,
	).Scan(&a.ID, &a.OwnerID, &a.Name, &a.Description, &a.PhotoCount, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("albums: update: %w", err)
	}
	return &a, nil
}

func (s *pgStore) DeleteAlbum(ctx context.Context, albumID string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM private_albums WHERE id = $1`, albumID)
	if err != nil {
		return fmt.Errorf("albums: delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgStore) AddPhoto(ctx context.Context, albumID, uploadID string, position int) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("albums: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		INSERT INTO private_album_photos (album_id, upload_id, position)
		VALUES ($1, $2, $3)`, albumID, uploadID, position); err != nil {
		return fmt.Errorf("albums: add photo: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE private_albums
		   SET photo_count = photo_count + 1,
		       updated_at  = NOW()
		 WHERE id = $1`, albumID); err != nil {
		return fmt.Errorf("albums: bump count: %w", err)
	}
	return tx.Commit(ctx)
}

func (s *pgStore) RemovePhoto(ctx context.Context, albumID, uploadID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("albums: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `
		DELETE FROM private_album_photos
		 WHERE album_id = $1 AND upload_id = $2`, albumID, uploadID)
	if err != nil {
		return fmt.Errorf("albums: remove photo: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `
		UPDATE private_albums
		   SET photo_count = GREATEST(photo_count - 1, 0),
		       updated_at  = NOW()
		 WHERE id = $1`, albumID); err != nil {
		return fmt.Errorf("albums: drop count: %w", err)
	}
	return tx.Commit(ctx)
}

func (s *pgStore) ListPhotos(ctx context.Context, albumID string) ([]Photo, error) {
	rows, err := s.db.Query(ctx, `
		SELECT p.upload_id, p.album_id, p.position, p.added_at, u.filename, u.content_type
		  FROM private_album_photos p
		  JOIN uploads u ON u.id = p.upload_id
		 WHERE p.album_id = $1
		 ORDER BY p.position ASC, p.added_at ASC`, albumID)
	if err != nil {
		return nil, fmt.Errorf("albums: list photos: %w", err)
	}
	defer rows.Close()
	out := []Photo{}
	for rows.Next() {
		var p Photo
		if err := rows.Scan(&p.UploadID, &p.AlbumID, &p.Position, &p.AddedAt, &p.Filename, &p.ContentType); err != nil {
			return nil, fmt.Errorf("albums: scan photo: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *pgStore) GetPhoto(ctx context.Context, albumID, uploadID string) (*Photo, error) {
	var p Photo
	err := s.db.QueryRow(ctx, `
		SELECT p.upload_id, p.album_id, p.position, p.added_at, u.filename, u.content_type
		  FROM private_album_photos p
		  JOIN uploads u ON u.id = p.upload_id
		 WHERE p.album_id = $1 AND p.upload_id = $2`, albumID, uploadID,
	).Scan(&p.UploadID, &p.AlbumID, &p.Position, &p.AddedAt, &p.Filename, &p.ContentType)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("albums: get photo: %w", err)
	}
	return &p, nil
}

func (s *pgStore) CreateGrant(ctx context.Context, albumID, granterID, granteeID, status, source string, expiresAt *time.Time) (*Grant, error) {
	var g Grant
	if status == GrantActive {
		err := s.db.QueryRow(ctx, `
			INSERT INTO private_album_grants (album_id, granter_id, grantee_id, status, source, granted_at, expires_at)
			VALUES ($1, $2, $3, $4, $5, NOW(), $6)
			RETURNING id, album_id, granter_id, grantee_id, status, source, requested_at, granted_at, revoked_at, expires_at`,
			albumID, granterID, granteeID, status, source, expiresAt,
		).Scan(&g.ID, &g.AlbumID, &g.GranterID, &g.GranteeID, &g.Status, &g.Source, &g.RequestedAt, &g.GrantedAt, &g.RevokedAt, &g.ExpiresAt)
		if err != nil {
			return nil, fmt.Errorf("albums: create grant: %w", err)
		}
		return &g, nil
	}
	err := s.db.QueryRow(ctx, `
		INSERT INTO private_album_grants (album_id, granter_id, grantee_id, status, source, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, album_id, granter_id, grantee_id, status, source, requested_at, granted_at, revoked_at, expires_at`,
		albumID, granterID, granteeID, status, source, expiresAt,
	).Scan(&g.ID, &g.AlbumID, &g.GranterID, &g.GranteeID, &g.Status, &g.Source, &g.RequestedAt, &g.GrantedAt, &g.RevokedAt, &g.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("albums: create grant: %w", err)
	}
	return &g, nil
}

func (s *pgStore) GetGrant(ctx context.Context, grantID string) (*Grant, error) {
	var g Grant
	err := s.db.QueryRow(ctx, `
		SELECT id, album_id, granter_id, grantee_id, status, source, requested_at, granted_at, revoked_at, expires_at
		  FROM private_album_grants
		 WHERE id = $1`, grantID,
	).Scan(&g.ID, &g.AlbumID, &g.GranterID, &g.GranteeID, &g.Status, &g.Source, &g.RequestedAt, &g.GrantedAt, &g.RevokedAt, &g.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("albums: get grant: %w", err)
	}
	return &g, nil
}

func (s *pgStore) UpdateGrantStatus(ctx context.Context, grantID, status string) (*Grant, error) {
	var g Grant
	// granted_at is stamped on first transition to active; revoked_at on
	// first transition to revoked. Both are coalesced — once stamped they
	// stay stamped even if the grant is later updated again.
	err := s.db.QueryRow(ctx, `
		UPDATE private_album_grants
		   SET status     = $2,
		       granted_at = CASE WHEN $2 = 'active'  AND granted_at IS NULL THEN NOW() ELSE granted_at END,
		       revoked_at = CASE WHEN $2 = 'revoked' AND revoked_at IS NULL THEN NOW() ELSE revoked_at END
		 WHERE id = $1
		 RETURNING id, album_id, granter_id, grantee_id, status, source, requested_at, granted_at, revoked_at, expires_at`,
		grantID, status,
	).Scan(&g.ID, &g.AlbumID, &g.GranterID, &g.GranteeID, &g.Status, &g.Source, &g.RequestedAt, &g.GrantedAt, &g.RevokedAt, &g.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("albums: update grant status: %w", err)
	}
	return &g, nil
}

func (s *pgStore) ListGrantsByAlbum(ctx context.Context, albumID string) ([]Grant, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, album_id, granter_id, grantee_id, status, source, requested_at, granted_at, revoked_at, expires_at
		  FROM private_album_grants
		 WHERE album_id = $1
		 ORDER BY requested_at DESC`, albumID)
	if err != nil {
		return nil, fmt.Errorf("albums: list grants: %w", err)
	}
	defer rows.Close()
	out := []Grant{}
	for rows.Next() {
		var g Grant
		if err := rows.Scan(&g.ID, &g.AlbumID, &g.GranterID, &g.GranteeID, &g.Status, &g.Source, &g.RequestedAt, &g.GrantedAt, &g.RevokedAt, &g.ExpiresAt); err != nil {
			return nil, fmt.Errorf("albums: scan grant: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *pgStore) FindOpenGrant(ctx context.Context, albumID, granteeID string) (*Grant, error) {
	var g Grant
	err := s.db.QueryRow(ctx, `
		SELECT id, album_id, granter_id, grantee_id, status, source, requested_at, granted_at, revoked_at, expires_at
		  FROM private_album_grants
		 WHERE album_id = $1 AND grantee_id = $2
		   AND status IN ('pending', 'active')
		   AND (expires_at IS NULL OR expires_at > NOW())
		 LIMIT 1`, albumID, granteeID,
	).Scan(&g.ID, &g.AlbumID, &g.GranterID, &g.GranteeID, &g.Status, &g.Source, &g.RequestedAt, &g.GrantedAt, &g.RevokedAt, &g.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("albums: find open grant: %w", err)
	}
	return &g, nil
}

func (s *pgStore) HasActiveGrant(ctx context.Context, albumID, viewerID string) (bool, error) {
	var n int
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(*)
		  FROM private_album_grants
		 WHERE album_id = $1 AND grantee_id = $2 AND status = 'active'
		   AND (expires_at IS NULL OR expires_at > NOW())`,
		albumID, viewerID,
	).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("albums: has active grant: %w", err)
	}
	return n > 0, nil
}

func (s *pgStore) ExpireAlbumGrants(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `
		UPDATE private_album_grants
		   SET status = 'revoked'
		 WHERE status = 'active'
		   AND expires_at IS NOT NULL
		   AND expires_at <= NOW()`)
	if err != nil {
		return fmt.Errorf("albums: expire grants: %w", err)
	}
	return nil
}

func scanAlbums(rows pgx.Rows) ([]Album, error) {
	out := []Album{}
	for rows.Next() {
		var a Album
		if err := rows.Scan(&a.ID, &a.OwnerID, &a.Name, &a.Description, &a.PhotoCount, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("albums: scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
