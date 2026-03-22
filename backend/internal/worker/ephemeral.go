package worker

import (
	"context"
	"log"
	"time"
)

// EphemeralStore is the subset of the chat.Store interface required to sweep
// expired TTL messages.
type EphemeralStore interface {
	// ListExpiredMessages returns the IDs of messages whose expires_at has elapsed.
	ListExpiredMessages(ctx context.Context) ([]string, error)
	// DeleteMessage deletes the message from the database and returns the room ID
	// and any storage keys to remove from object storage.
	DeleteMessage(ctx context.Context, messageID string) (roomID string, storageKeys []string, err error)
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
	// notify, when non-nil, is called after each deleted message so the hub
	// can broadcast a message_deleted event to room members.
	notify func(roomID, messageID string)
}

// NewEphemeralCleaner creates an EphemeralCleaner. notify may be nil.
func NewEphemeralCleaner(store EphemeralStore, storage DeletionStorage, notify func(roomID, messageID string)) *EphemeralCleaner {
	return &EphemeralCleaner{store: store, storage: storage, notify: notify}
}

// Start launches the background sweep goroutine. It runs until ctx is cancelled.
func (e *EphemeralCleaner) Start(ctx context.Context) {
	go func() {
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
		log.Printf("ephemeral cleaner: list expired: %v", err)
		return
	}

	for _, id := range ids {
		roomID, keys, err := e.store.DeleteMessage(ctx, id)
		if err != nil {
			log.Printf("ephemeral cleaner: delete message %s: %v", id, err)
			continue
		}

		for _, key := range keys {
			if err := e.storage.Delete(ctx, key); err != nil {
				log.Printf("ephemeral cleaner: delete storage key %s: %v", key, err)
			}
		}

		if e.notify != nil {
			e.notify(roomID, id)
		}
	}
}
