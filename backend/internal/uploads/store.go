package uploads

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

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

// GetByStorageKey returns an upload by its storage key. Used by the attach-time
// gate for surfaces (avatar, profile gallery) that reference media by URL
// rather than upload ID.
func (s *pgStore) GetByStorageKey(ctx context.Context, storageKey string) (*Upload, error) {
	var u Upload
	err := s.db.QueryRow(ctx, `
		SELECT id, user_id, storage_key, filename, content_type, size_bytes, category, status,
		       thumbnail_key, created_at, committed_at,
		       moderation_status, moderation_code, moderation_reason, moderated_at
		FROM uploads
		WHERE storage_key = $1`, storageKey,
	).Scan(&u.ID, &u.UserID, &u.StorageKey, &u.Filename, &u.ContentType, &u.SizeBytes, &u.Category, &u.Status,
		&u.ThumbnailKey, &u.CreatedAt, &u.CommittedAt,
		&u.ModerationStatus, &u.ModerationCode, &u.ModerationReason, &u.ModeratedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("uploads: get by storage key: %w", err)
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

// MarkRejected records a moderation rejection plus the audit detail (score,
// categories, whether the storage object was kept for admin review). When
// rec.FileRetained is false the lifecycle status flips to 'failed' since the
// public URL would 404; when true it stays 'committed' so the admin review
// page can stream the file.
func (s *pgStore) MarkRejected(ctx context.Context, rec RejectionRecord) error {
	// Score and Categories are stored as nullable so an admin viewing a
	// hash-list rejection (which has no probability) sees an explicit NULL
	// rather than a misleading "0.0".
	var score any
	if rec.Score > 0 {
		score = rec.Score
	}
	var categories any
	if len(rec.Categories) > 0 {
		categories = rec.Categories
	}
	status := "failed"
	if rec.FileRetained {
		status = "committed"
	}
	tag, err := s.db.Exec(ctx, `
		UPDATE uploads
		   SET moderation_status        = 'rejected',
		       moderation_code          = $2,
		       moderation_reason        = $3,
		       moderation_score         = $4,
		       moderation_categories    = $5,
		       moderation_file_retained = $6,
		       moderated_at             = NOW(),
		       status                   = $7
		 WHERE id = $1`,
		rec.UploadID, rec.Code, rec.Reason, score, categories, rec.FileRetained, status)
	if err != nil {
		return fmt.Errorf("uploads: mark rejected: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_ = rec.Source // recorded in logs at the worker layer; not persisted to keep the schema lean
	return nil
}

// MarkQuarantined records a CSAM-class hit. The original storage object has
// already been moved by the worker to quarantineKey (a restricted, never-served
// prefix); here we set moderation_status to 'quarantined' and flip the
// lifecycle status to 'failed' so the original public URL 404s. The object is
// preserved, not purged — destroying it can itself be unlawful.
func (s *pgStore) MarkQuarantined(ctx context.Context, rec RejectionRecord, quarantineKey string) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE uploads
		   SET moderation_status         = 'quarantined',
		       moderation_code           = $2,
		       moderation_reason         = $3,
		       moderation_quarantine_key = $4,
		       moderation_file_retained  = FALSE,
		       moderated_at              = NOW(),
		       status                    = 'failed'
		 WHERE id = $1`,
		rec.UploadID, rec.Code, rec.Reason, quarantineKey)
	if err != nil {
		return fmt.Errorf("uploads: mark quarantined: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_ = rec.Source // recorded in logs at the worker layer; not persisted to keep the schema lean
	return nil
}

// RetainedRejection describes a rejected upload whose storage object is
// still kept for admin review. Returned by ListExpiredRetained for the
// cleanup worker.
type RetainedRejection struct {
	UploadID     string
	StorageKey   string
	ThumbnailKey *string
}

// ListExpiredRetained returns rejected uploads whose retention window has
// elapsed (moderated_at < before) and whose storage object is still around.
// The cleanup worker uses this to drive object deletion + flag flipping.
func (s *pgStore) ListExpiredRetained(ctx context.Context, before time.Time, limit int) ([]RetainedRejection, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, storage_key, thumbnail_key
		  FROM uploads
		 WHERE moderation_status = 'rejected'
		   AND moderation_file_retained = TRUE
		   AND moderated_at < $1
		 ORDER BY moderated_at
		 LIMIT $2`, before, limit)
	if err != nil {
		return nil, fmt.Errorf("uploads: list expired retained: %w", err)
	}
	defer rows.Close()
	out := []RetainedRejection{}
	for rows.Next() {
		var r RetainedRejection
		if err := rows.Scan(&r.UploadID, &r.StorageKey, &r.ThumbnailKey); err != nil {
			return nil, fmt.Errorf("uploads: scan expired retained: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ClearRetention flips moderation_file_retained to FALSE and the lifecycle
// status to 'failed' after the cleanup worker has purged the storage object.
// Idempotent — re-running on an already-cleared row is a no-op.
func (s *pgStore) ClearRetention(ctx context.Context, uploadID string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE uploads
		   SET moderation_file_retained = FALSE,
		       status                   = 'failed'
		 WHERE id = $1
		   AND moderation_file_retained = TRUE`, uploadID)
	if err != nil {
		return fmt.Errorf("uploads: clear retention: %w", err)
	}
	return nil
}

func randomID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
