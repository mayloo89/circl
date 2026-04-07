package worker

import (
	"context"
	"log"
	"time"
)

const deletionGracePeriod = 30 * 24 * time.Hour

// AccountPurger anonymizes accounts that have been soft-deleted past the grace period.
type AccountPurger interface {
	PurgeExpiredDeletedUsers(ctx context.Context, before time.Time) (int64, error)
}

// PurgeDeletedAccounts anonymizes accounts deleted more than 30 days ago.
// Intended to be called on a daily schedule.
func PurgeDeletedAccounts(ctx context.Context, store AccountPurger) {
	cutoff := time.Now().Add(-deletionGracePeriod)
	n, err := store.PurgeExpiredDeletedUsers(ctx, cutoff)
	if err != nil {
		log.Printf("worker: purge deleted accounts: %v", err)
		return
	}
	if n > 0 {
		log.Printf("worker: purged %d expired deleted account(s)", n)
	}
}
