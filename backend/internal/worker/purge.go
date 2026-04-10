package worker

import (
	"context"
	"log"
	"time"
)

const deletionGracePeriod = 30 * 24 * time.Hour

// AccountPurger is the data-access interface required to purge expired deleted accounts.
type AccountPurger interface {
	GetExpiredDeletedUserIDs(ctx context.Context, before time.Time) ([]string, error)
	GetUserUploadKeys(ctx context.Context, userID string) (storageKeys, thumbnailKeys []string, err error)
	DeleteUserData(ctx context.Context, userID string) error
	AnonymizeUser(ctx context.Context, userID string) error
}

// FileStorage can delete a single object from object storage by key.
type FileStorage interface {
	Delete(ctx context.Context, key string) error
}

// PurgeDeletedAccounts removes all data for accounts that have been soft-deleted
// past the 30-day grace period. Intended to be called on a daily schedule.
func PurgeDeletedAccounts(ctx context.Context, store AccountPurger, storage FileStorage) {
	cutoff := time.Now().Add(-deletionGracePeriod)
	ids, err := store.GetExpiredDeletedUserIDs(ctx, cutoff)
	if err != nil {
		log.Printf("worker: purge: get expired user IDs: %v", err)
		return
	}
	for _, userID := range ids {
		if err := purgeUser(ctx, store, storage, userID); err != nil {
			log.Printf("worker: purge: user %s: %v", userID, err)
		}
	}
	if len(ids) > 0 {
		log.Printf("worker: purged %d expired deleted account(s)", len(ids))
	}
}

func purgeUser(ctx context.Context, store AccountPurger, storage FileStorage, userID string) error {
	storageKeys, thumbnailKeys, err := store.GetUserUploadKeys(ctx, userID)
	if err != nil {
		return err
	}

	// Delete files from object storage. Failures are logged but don't abort the
	// DB cleanup — a missing file is not a reason to leave PII in the database.
	for _, key := range storageKeys {
		if key == "" {
			continue
		}
		if err := storage.Delete(ctx, key); err != nil {
			log.Printf("worker: purge: delete storage key %q: %v", key, err)
		}
	}
	for _, key := range thumbnailKeys {
		if key == "" {
			continue
		}
		if err := storage.Delete(ctx, key); err != nil {
			log.Printf("worker: purge: delete thumbnail key %q: %v", key, err)
		}
	}

	if err := store.DeleteUserData(ctx, userID); err != nil {
		return err
	}
	return store.AnonymizeUser(ctx, userID)
}
