package worker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/worker"
)

// fakeEphemeralStore implements worker.EphemeralStore for tests.
type fakeEphemeralStore struct {
	expiredIDs  []string
	expiredErr  error
	deleteRooms map[string]string   // messageID → roomID
	deleteKeys  map[string][]string // messageID → storage keys
	deleteErr   error
	deletedIDs  []string
}

func (f *fakeEphemeralStore) ListExpiredMessages(_ context.Context) ([]string, error) {
	return f.expiredIDs, f.expiredErr
}

func (f *fakeEphemeralStore) TombstoneMessage(_ context.Context, id string) (string, []string, error) {
	if f.deleteErr != nil {
		return "", nil, f.deleteErr
	}
	f.deletedIDs = append(f.deletedIDs, id)
	return f.deleteRooms[id], f.deleteKeys[id], nil
}

// fakeDeletionStorage implements worker.DeletionStorage for tests.
type fakeDeletionStorage struct {
	deletedKeys []string
	deleteErr   error
}

func (f *fakeDeletionStorage) Delete(_ context.Context, key string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deletedKeys = append(f.deletedKeys, key)
	return nil
}

func TestSweep_DeletesExpiredMessages(t *testing.T) {
	store := &fakeEphemeralStore{
		expiredIDs:  []string{"m-1", "m-2"},
		deleteRooms: map[string]string{"m-1": "r-1", "m-2": "r-2"},
		deleteKeys: map[string][]string{
			"m-1": {"uploads/a.jpg", "thumbnails/a.jpg"},
			"m-2": {},
		},
	}
	storage := &fakeDeletionStorage{}

	var notified []string
	cleaner := worker.NewEphemeralCleaner(store, storage, func(roomID, msgID string) {
		notified = append(notified, roomID+":"+msgID)
	}, zerolog.Nop())

	cleaner.Sweep(context.Background())

	if len(store.deletedIDs) != 2 {
		t.Errorf("deleted %d messages, want 2", len(store.deletedIDs))
	}
	if len(storage.deletedKeys) != 2 {
		t.Errorf("deleted %d storage keys, want 2 (uploads+thumbnail)", len(storage.deletedKeys))
	}
	if len(notified) != 2 {
		t.Errorf("notified %d times, want 2", len(notified))
	}
	if notified[0] != "r-1:m-1" {
		t.Errorf("notified[0] = %q, want r-1:m-1", notified[0])
	}
}

func TestSweep_NoExpiredMessages(t *testing.T) {
	store := &fakeEphemeralStore{expiredIDs: nil}
	storage := &fakeDeletionStorage{}
	cleaner := worker.NewEphemeralCleaner(store, storage, nil, zerolog.Nop())

	cleaner.Sweep(context.Background())

	if len(store.deletedIDs) != 0 {
		t.Errorf("expected 0 deletions, got %d", len(store.deletedIDs))
	}
}

func TestSweep_ListError(t *testing.T) {
	store := &fakeEphemeralStore{expiredErr: errors.New("db fail")}
	storage := &fakeDeletionStorage{}
	cleaner := worker.NewEphemeralCleaner(store, storage, nil, zerolog.Nop())

	// Must not panic.
	cleaner.Sweep(context.Background())

	if len(store.deletedIDs) != 0 {
		t.Errorf("expected 0 deletions on list error, got %d", len(store.deletedIDs))
	}
}

func TestSweep_DeleteError(t *testing.T) {
	store := &fakeEphemeralStore{
		expiredIDs:  []string{"m-1"},
		deleteErr:   errors.New("db fail"),
		deleteRooms: map[string]string{"m-1": "r-1"},
		deleteKeys:  map[string][]string{"m-1": {}},
	}
	storage := &fakeDeletionStorage{}

	notified := false
	cleaner := worker.NewEphemeralCleaner(store, storage, func(_ string, _ string) { notified = true }, zerolog.Nop())

	// Must not panic and must not notify on failure.
	cleaner.Sweep(context.Background())

	if notified {
		t.Error("should not notify on DB delete error")
	}
}

func TestSweep_StorageDeleteError(t *testing.T) {
	store := &fakeEphemeralStore{
		expiredIDs:  []string{"m-1"},
		deleteRooms: map[string]string{"m-1": "r-1"},
		deleteKeys:  map[string][]string{"m-1": {"uploads/a.jpg"}},
	}
	storage := &fakeDeletionStorage{deleteErr: errors.New("s3 fail")}

	notified := false
	cleaner := worker.NewEphemeralCleaner(store, storage, func(_ string, _ string) { notified = true }, zerolog.Nop())

	// Storage error must not prevent notification.
	cleaner.Sweep(context.Background())

	if !notified {
		t.Error("should still notify even when storage delete fails")
	}
	if len(store.deletedIDs) != 1 {
		t.Errorf("expected 1 DB deletion, got %d", len(store.deletedIDs))
	}
}

func TestSweep_NilNotify(t *testing.T) {
	store := &fakeEphemeralStore{
		expiredIDs:  []string{"m-1"},
		deleteRooms: map[string]string{"m-1": "r-1"},
		deleteKeys:  map[string][]string{"m-1": {}},
	}
	storage := &fakeDeletionStorage{}
	cleaner := worker.NewEphemeralCleaner(store, storage, nil, zerolog.Nop())

	// Must not panic when notify is nil.
	cleaner.Sweep(context.Background())

	if len(store.deletedIDs) != 1 {
		t.Errorf("expected 1 deletion, got %d", len(store.deletedIDs))
	}
}

func TestSweep_MultipleStorageKeys(t *testing.T) {
	store := &fakeEphemeralStore{
		expiredIDs:  []string{"m-1"},
		deleteRooms: map[string]string{"m-1": "r-1"},
		deleteKeys: map[string][]string{
			"m-1": {"uploads/img.jpg", "thumbnails/img.jpg"},
		},
	}
	storage := &fakeDeletionStorage{}
	cleaner := worker.NewEphemeralCleaner(store, storage, nil, zerolog.Nop())

	cleaner.Sweep(context.Background())

	if len(storage.deletedKeys) != 2 {
		t.Errorf("deleted %d keys, want 2", len(storage.deletedKeys))
	}
}

func TestEphemeralCleaner_Start_StopsOnContextCancel(t *testing.T) {
	store := &fakeEphemeralStore{}
	storage := &fakeDeletionStorage{}
	cleaner := worker.NewEphemeralCleaner(store, storage, nil, zerolog.Nop())

	ctx, cancel := context.WithCancel(context.Background())
	cleaner.Start(ctx)
	cancel()

	// Allow the goroutine to exit.
	time.Sleep(20 * time.Millisecond)
	// No deadlock or panic is the assertion.
}
