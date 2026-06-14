package worker

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/logger"
)

// EphemeralStore is the subset of the chat.Store interface required to sweep
// expired TTL messages.
type EphemeralStore interface {
	// ListExpiredMessages returns the IDs of messages whose expires_at has elapsed.
	ListExpiredMessages(ctx context.Context) ([]string, error)
	// TombstoneMessage converts an expired message into a tombstone so the chat
	// history retains a placeholder, and returns the room ID and any storage
	// keys to remove from object storage.
	TombstoneMessage(ctx context.Context, messageID string) (roomID string, storageKeys []string, err error)
}

// DeletionStorage is the subset of storage.Storage required to delete files.
type DeletionStorage interface {
	Delete(ctx context.Context, key string) error
}

// EphemeralCleaner runs a periodic sweep that deletes TTL-expired messages
// from the database and object storage, then notifies connected clients.
type EphemeralCleaner struct {
	store   EphemeralStore
	storage DeletionStorage
	log     zerolog.Logger
	// notify, when non-nil, is called after each deleted message so the hub
	// can broadcast a message_deleted event to room members.
	notify func(roomID, messageID string)
}

// NewEphemeralCleaner creates an EphemeralCleaner. notify may be nil.
func NewEphemeralCleaner(store EphemeralStore, storage DeletionStorage, notify func(roomID, messageID string), log zerolog.Logger) *EphemeralCleaner {
	return &EphemeralCleaner{
		store:   store,
		storage: storage,
		notify:  notify,
		log:     log.With().Str("component", "ephemeral_cleaner").Logger(),
	}
}

// Start launches the background sweep goroutine. It runs until ctx is cancelled.
func (e *EphemeralCleaner) Start(ctx context.Context) {
	go func() {
		defer logger.Recover(e.log, "worker.ephemeral")
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				e.Sweep(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Sweep performs a single pass: finds all expired messages, deletes their
// storage files, removes them from the database, and fires notifications.
// It is exported so that it can be called directly in tests.
func (e *EphemeralCleaner) Sweep(ctx context.Context) {
	ids, err := e.store.ListExpiredMessages(ctx)
	if err != nil {
		e.log.Error().Err(err).Msg("list expired messages failed")
		return
	}

	for _, id := range ids {
		roomID, keys, err := e.store.TombstoneMessage(ctx, id)
		if err != nil {
			e.log.Error().Err(err).Str("message_id", id).Msg("tombstone message failed")
			continue
		}

		for _, key := range keys {
			if err := e.storage.Delete(ctx, key); err != nil {
				e.log.Error().Err(err).Str("storage_key", key).Msg("delete storage key failed")
			}
		}

		if e.notify != nil {
			e.notify(roomID, id)
		}
	}
}
