package worker

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/logger"
)

// RetentionStore is the subset of the chat.Store interface required to sweep
// retention-expired messages.
type RetentionStore interface {
	ListRetentionEligibleMessages(ctx context.Context) ([]string, error)
	DeleteMessage(ctx context.Context, messageID string) (roomID string, storageKeys []string, err error)
}

// RetentionCleaner runs a periodic sweep that hard-deletes messages past the
// retention window for their room type. Unlike the EphemeralCleaner (which
// tombstones user-set self-destruct / view-once messages), the retention
// cleaner removes the row entirely — no placeholder is left.
type RetentionCleaner struct {
	store   RetentionStore
	storage DeletionStorage
	log     zerolog.Logger
	notify  func(roomID, messageID string)
}

// NewRetentionCleaner creates a RetentionCleaner. notify may be nil.
func NewRetentionCleaner(store RetentionStore, storage DeletionStorage, notify func(roomID, messageID string), log zerolog.Logger) *RetentionCleaner {
	return &RetentionCleaner{
		store:   store,
		storage: storage,
		notify:  notify,
		log:     log.With().Str("component", "retention_cleaner").Logger(),
	}
}

// Start launches the background sweep goroutine. It runs until ctx is cancelled.
func (r *RetentionCleaner) Start(ctx context.Context) {
	go func() {
		defer logger.Recover(r.log, "worker.retention")
		r.Sweep(ctx)
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				r.Sweep(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Sweep performs a single pass: finds all retention-eligible messages,
// hard-deletes them, removes their storage files, and fires notifications.
// It is exported so that it can be called directly in tests.
func (r *RetentionCleaner) Sweep(ctx context.Context) {
	ids, err := r.store.ListRetentionEligibleMessages(ctx)
	if err != nil {
		r.log.Error().Err(err).Msg("list retention eligible messages failed")
		return
	}
	if len(ids) > 0 {
		r.log.Info().Int("count", len(ids)).Msg("retention sweep found eligible messages")
	}

	for _, id := range ids {
		roomID, keys, err := r.store.DeleteMessage(ctx, id)
		if err != nil {
			r.log.Error().Err(err).Str("message_id", id).Msg("retention hard-delete failed")
			continue
		}

		for _, key := range keys {
			if err := r.storage.Delete(ctx, key); err != nil {
				r.log.Error().Err(err).Str("storage_key", key).Msg("retention delete storage key failed")
			}
		}

		if r.notify != nil {
			r.notify(roomID, id)
		}
	}
}
