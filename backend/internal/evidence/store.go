package evidence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgStore struct{ pool *pgxpool.Pool }

// NewStore returns a Postgres-backed Store.
func NewStore(pool *pgxpool.Pool) Store { return &pgStore{pool: pool} }

func collectSources(rows pgx.Rows, kind string) ([]Source, error) {
	defer rows.Close()
	var out []Source
	for rows.Next() {
		src := Source{Kind: kind}
		if err := rows.Scan(&src.UploadID, &src.Key, &src.ContentType, &src.SizeBytes); err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, rows.Err()
}

// MessageMedia reads from the quarantine copy when moderation already moved the
// original there; that object outlives the upload's 'failed' status.
func (s *pgStore) MessageMedia(ctx context.Context, messageID string) ([]Source, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, COALESCE(moderation_quarantine_key, storage_key), content_type, size_bytes
		  FROM uploads
		 WHERE message_id = $1
		   AND (status = 'committed' OR moderation_status = 'quarantined')
		 ORDER BY created_at`, messageID)
	if err != nil {
		return nil, fmt.Errorf("message media: %w", err)
	}
	return collectSources(rows, KindMessageMedia)
}

// ProfilePhotos matches uploads against the URLs the profile shows, so photos
// the user replaced or removed are not swept in.
func (s *pgStore) ProfilePhotos(ctx context.Context, userID, urlPrefix string) ([]Source, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.storage_key, u.content_type, u.size_bytes
		  FROM uploads u
		 WHERE u.user_id = $1
		   AND u.status = 'committed'
		   AND u.category IN ('avatar', 'gallery')
		   AND $2 || u.storage_key IN (
		           SELECT avatar_url FROM profiles WHERE user_id = $1 AND avatar_url IS NOT NULL
		           UNION ALL
		           SELECT url FROM profile_photos WHERE user_id = $1)
		 ORDER BY u.created_at`, userID, urlPrefix)
	if err != nil {
		return nil, fmt.Errorf("profile photos: %w", err)
	}
	return collectSources(rows, KindProfilePhoto)
}

func (s *pgStore) Record(ctx context.Context, f File) error {
	var source *string
	if f.SourceUploadID != "" {
		source = &f.SourceUploadID
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO report_evidence_files
		       (id, report_id, kind, storage_key, content_type, size_bytes, sha256, source_upload_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		f.ID, f.ReportID, f.Kind, f.StorageKey, f.ContentType, f.SizeBytes, f.SHA256, source)
	if err != nil {
		return fmt.Errorf("record evidence: %w", err)
	}
	return nil
}

const fileColumns = `f.id, f.report_id, f.kind, f.content_type, f.size_bytes, f.sha256, f.created_at,
	f.storage_key, r.reason, COALESCE(f.source_upload_id::text, '')`

func scanFile(row pgx.Row) (*File, error) {
	var f File
	err := row.Scan(&f.ID, &f.ReportID, &f.Kind, &f.ContentType, &f.SizeBytes, &f.SHA256, &f.CreatedAt,
		&f.StorageKey, &f.ReportReason, &f.SourceUploadID)
	return &f, err
}

func (s *pgStore) List(ctx context.Context, reportID string) ([]File, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+fileColumns+`
		  FROM report_evidence_files f
		  JOIN reports r ON r.id = f.report_id
		 WHERE f.report_id = $1
		 ORDER BY f.created_at, f.id`, reportID)
	if err != nil {
		return nil, fmt.Errorf("list evidence: %w", err)
	}
	defer rows.Close()
	out := []File{}
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, fmt.Errorf("scan evidence: %w", err)
		}
		out = append(out, *f)
	}
	return out, rows.Err()
}

func (s *pgStore) Get(ctx context.Context, reportID, fileID string) (*File, error) {
	f, err := scanFile(s.pool.QueryRow(ctx, `
		SELECT `+fileColumns+`
		  FROM report_evidence_files f
		  JOIN reports r ON r.id = f.report_id
		 WHERE f.report_id = $1 AND f.id = $2`, reportID, fileID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get evidence: %w", err)
	}
	return f, nil
}

func (s *pgStore) Delete(ctx context.Context, fileID string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM report_evidence_files WHERE id = $1`, fileID); err != nil {
		return fmt.Errorf("delete evidence: %w", err)
	}
	return nil
}

func (s *pgStore) DueForPurge(ctx context.Context, cutoff time.Time, limit int) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.id
		  FROM reports r
		 WHERE (r.status = 'dismissed' OR (r.status = 'reviewed' AND r.reviewed_at < $1))
		   AND (r.message_content IS NOT NULL
		        OR EXISTS (SELECT 1 FROM report_evidence_files f WHERE f.report_id = r.id))
		 ORDER BY r.reviewed_at
		 LIMIT $2`, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("due for purge: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (s *pgStore) MarkPurged(ctx context.Context, reportID string) error {
	if _, err := s.pool.Exec(ctx, `
		UPDATE reports
		   SET message_content    = NULL,
		       evidence_purged_at = COALESCE(evidence_purged_at, NOW())
		 WHERE id = $1`, reportID); err != nil {
		return fmt.Errorf("mark purged: %w", err)
	}
	return nil
}
