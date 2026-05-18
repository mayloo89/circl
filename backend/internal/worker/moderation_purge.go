package worker

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/uploads"
)

// DefaultModerationRetentionDays is how long rejected (but non-hash-list)
// upload files are kept around so admins can review false positives before
// the cleanup worker purges them.
const DefaultModerationRetentionDays = 30

// RetainedRejectionPurger is the data-access interface for the cleanup of
// rejected-but-retained moderation files. Implemented by uploads.Store.
type RetainedRejectionPurger interface {
	ListExpiredRetained(ctx context.Context, before time.Time, limit int) ([]uploads.RetainedRejection, error)
	ClearRetention(ctx context.Context, uploadID string) error
}

// PurgeExpiredModerationFiles removes storage objects for rejected uploads
// whose retention window has elapsed and flips moderation_file_retained to
// FALSE on the row (so it never appears in the admin queue's "viewable"
// section again). Intended to run on a daily ticker alongside
// PurgeDeletedAccounts. Errors per upload are logged and the loop continues
// — one missing object should not block the rest of the sweep.
func PurgeExpiredModerationFiles(ctx context.Context, log zerolog.Logger, store RetainedRejectionPurger, storage FileStorage, retentionDays int) {
	log = log.With().Str("component", "moderation_purge").Logger()
	if retentionDays <= 0 {
		retentionDays = DefaultModerationRetentionDays
	}
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	items, err := store.ListExpiredRetained(ctx, cutoff, 0)
	if err != nil {
		log.Error().Err(err).Msg("list expired retained failed")
		return
	}
	for _, it := range items {
		if it.StorageKey != "" {
			if err := storage.Delete(ctx, it.StorageKey); err != nil {
				log.Warn().Err(err).Str("storage_key", it.StorageKey).Msg("delete retained original failed")
			}
		}
		if it.ThumbnailKey != nil && *it.ThumbnailKey != "" {
			if err := storage.Delete(ctx, *it.ThumbnailKey); err != nil {
				log.Warn().Err(err).Str("storage_key", *it.ThumbnailKey).Msg("delete retained thumbnail failed")
			}
		}
		if err := store.ClearRetention(ctx, it.UploadID); err != nil {
			log.Error().Err(err).Str("upload_id", it.UploadID).Msg("clear retention failed")
		}
	}
	if len(items) > 0 {
		log.Info().Int("count", len(items)).Msg("purged expired moderation files")
	}
}
