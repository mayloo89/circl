package worker

import (
	"context"
	"time"

	"github.com/rs/zerolog"
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
func PurgeDeletedAccounts(ctx context.Context, log zerolog.Logger, store AccountPurger, storage FileStorage) {
	log = log.With().Str("component", "purge_worker").Logger()
	cutoff := time.Now().Add(-deletionGracePeriod)
	ids, err := store.GetExpiredDeletedUserIDs(ctx, cutoff)
	if err != nil {
		log.Error().Err(err).Msg("get expired user IDs failed")
		return
	}
	for _, userID := range ids {
		if err := purgeUser(ctx, log, store, storage, userID); err != nil {
			log.Error().Err(err).Str("user_id", userID).Msg("purge user failed")
		}
	}
	if len(ids) > 0 {
		log.Info().Int("count", len(ids)).Msg("purged expired deleted accounts")
	}
}

func purgeUser(ctx context.Context, log zerolog.Logger, store AccountPurger, storage FileStorage, userID string) error {
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
			log.Error().Err(err).Str("storage_key", key).Msg("delete storage key failed")
		}
	}
	for _, key := range thumbnailKeys {
		if key == "" {
			continue
		}
		if err := storage.Delete(ctx, key); err != nil {
			log.Error().Err(err).Str("storage_key", key).Msg("delete thumbnail key failed")
		}
	}

	if err := store.DeleteUserData(ctx, userID); err != nil {
		return err
	}
	return store.AnonymizeUser(ctx, userID)
}
