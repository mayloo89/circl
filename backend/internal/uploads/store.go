package uploads

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgStore struct {
	db *pgxpool.Pool
}

// NewStore returns a Store backed by PostgreSQL.
func NewStore(pool *pgxpool.Pool) Store {
	return &pgStore{db: pool}
}

// Create inserts a new upload record and populates u.ID and u.CreatedAt.
func (s *pgStore) Create(ctx context.Context, u *Upload) error {
	err := s.db.QueryRow(ctx, `
		INSERT INTO uploads (user_id, storage_key, filename, content_type, size_bytes, category, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`,
		u.UserID, u.StorageKey, u.Filename, u.ContentType, u.SizeBytes, u.Category, u.Status,
	).Scan(&u.ID, &u.CreatedAt)
	if err != nil {
		return fmt.Errorf("uploads: create: %w", err)
	}
	return nil
}

// GetByID returns an upload by its primary key.
func (s *pgStore) GetByID(ctx context.Context, id string) (*Upload, error) {
	var u Upload
	err := s.db.QueryRow(ctx, `
		SELECT id, user_id, storage_key, filename, content_type, size_bytes, category, status,
		       thumbnail_key, created_at, committed_at,
		       moderation_status, moderation_code, moderation_reason, moderated_at
		FROM uploads
		WHERE id = $1`, id,
	).Scan(&u.ID, &u.UserID, &u.StorageKey, &u.Filename, &u.ContentType, &u.SizeBytes, &u.Category, &u.Status,
		&u.ThumbnailKey, &u.CreatedAt, &u.CommittedAt,
		&u.ModerationStatus, &u.ModerationCode, &u.ModerationReason, &u.ModeratedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("uploads: get by id: %w", err)
	}
	return &u, nil
}

// SetThumbnailKey stores the thumbnail storage key after background processing.
func (s *pgStore) SetThumbnailKey(ctx context.Context, id, thumbnailKey string) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE uploads SET thumbnail_key = $2 WHERE id = $1`, id, thumbnailKey)
	if err != nil {
		return fmt.Errorf("uploads: set thumbnail key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Commit transitions an upload from pending to committed.
func (s *pgStore) Commit(ctx context.Context, id string) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE uploads SET status = 'committed', committed_at = NOW()
		WHERE id = $1 AND status = 'pending'`, id)
	if err != nil {
		return fmt.Errorf("uploads: commit: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotPending
	}
	return nil
}

// MarkApproved records that the moderation pipeline cleared this upload.
// Idempotent — re-running an approval is a no-op.
func (s *pgStore) MarkApproved(ctx context.Context, id string) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE uploads
		   SET moderation_status = 'approved',
		       moderated_at = NOW()
		 WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("uploads: mark approved: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkRejected records a moderation rejection. The upload's lifecycle
// status flips to 'failed' since the storage object has been deleted; the
// moderation_status / code / reason fields carry the audit detail.
func (s *pgStore) MarkRejected(ctx context.Context, id, code, reason, source string) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE uploads
		   SET moderation_status = 'rejected',
		       moderation_code   = $2,
		       moderation_reason = $3,
		       moderated_at      = NOW(),
		       status            = 'failed'
		 WHERE id = $1`,
		id, code, reason)
	if err != nil {
		return fmt.Errorf("uploads: mark rejected: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_ = source // recorded in logs at the worker layer; not persisted to keep the schema lean
	return nil
}

func randomID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
