package worker_test

import (
	"context"
	"errors"
	"testing"

	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/worker"
)

type fakeRetentionStore struct {
	eligibleIDs []string
	eligibleErr error
	deleteRooms map[string]string
	deleteKeys  map[string][]string
	deleteErr   error
	deletedIDs  []string
}

func (f *fakeRetentionStore) ListRetentionEligibleMessages(_ context.Context) ([]string, error) {
	return f.eligibleIDs, f.eligibleErr
}

func (f *fakeRetentionStore) DeleteMessage(_ context.Context, id string) (string, []string, error) {
	if f.deleteErr != nil {
		return "", nil, f.deleteErr
	}
	f.deletedIDs = append(f.deletedIDs, id)
	return f.deleteRooms[id], f.deleteKeys[id], nil
}

func TestRetentionSweep_DeletesEligibleMessages(t *testing.T) {
	store := &fakeRetentionStore{
		eligibleIDs: []string{"m-1", "m-2"},
		deleteRooms: map[string]string{"m-1": "r-1", "m-2": "r-2"},
		deleteKeys: map[string][]string{
			"m-1": {"uploads/a.jpg", "thumbnails/a.jpg"},
			"m-2": {},
		},
	}
	storage := &fakeDeletionStorage{}

	var notified []string
	cleaner := worker.NewRetentionCleaner(store, storage, func(roomID, msgID string) {
		notified = append(notified, roomID+":"+msgID)
	}, zerolog.Nop())

	cleaner.Sweep(t.Context())

	if len(store.deletedIDs) != 2 {
		t.Errorf("deleted %d messages, want 2", len(store.deletedIDs))
	}
	if len(storage.deletedKeys) != 2 {
		t.Errorf("deleted %d storage keys, want 2", len(storage.deletedKeys))
	}
	if len(notified) != 2 {
		t.Errorf("notified %d times, want 2", len(notified))
	}
	if notified[0] != "r-1:m-1" {
		t.Errorf("notified[0] = %q, want r-1:m-1", notified[0])
	}
}

func TestRetentionSweep_NoEligibleMessages(t *testing.T) {
	store := &fakeRetentionStore{eligibleIDs: nil}
	storage := &fakeDeletionStorage{}
	cleaner := worker.NewRetentionCleaner(store, storage, nil, zerolog.Nop())

	cleaner.Sweep(t.Context())

	if len(store.deletedIDs) != 0 {
		t.Errorf("expected 0 deletions, got %d", len(store.deletedIDs))
	}
}

func TestRetentionSweep_ListError(t *testing.T) {
	store := &fakeRetentionStore{eligibleErr: errors.New("db fail")}
	storage := &fakeDeletionStorage{}
	cleaner := worker.NewRetentionCleaner(store, storage, nil, zerolog.Nop())

	cleaner.Sweep(t.Context())

	if len(store.deletedIDs) != 0 {
		t.Errorf("expected 0 deletions on list error, got %d", len(store.deletedIDs))
	}
}

func TestRetentionSweep_DeleteError(t *testing.T) {
	store := &fakeRetentionStore{
		eligibleIDs: []string{"m-1"},
		deleteErr:   errors.New("db fail"),
		deleteRooms: map[string]string{"m-1": "r-1"},
		deleteKeys:  map[string][]string{"m-1": {}},
	}
	storage := &fakeDeletionStorage{}

	notified := false
	cleaner := worker.NewRetentionCleaner(store, storage, func(_, _ string) { notified = true }, zerolog.Nop())

	cleaner.Sweep(t.Context())

	if notified {
		t.Error("should not notify on DB delete error")
	}
}

func TestRetentionSweep_StorageDeleteError(t *testing.T) {
	store := &fakeRetentionStore{
		eligibleIDs: []string{"m-1"},
		deleteRooms: map[string]string{"m-1": "r-1"},
		deleteKeys:  map[string][]string{"m-1": {"uploads/a.jpg"}},
	}
	storage := &fakeDeletionStorage{deleteErr: errors.New("s3 fail")}

	notified := false
	cleaner := worker.NewRetentionCleaner(store, storage, func(_, _ string) { notified = true }, zerolog.Nop())

	cleaner.Sweep(t.Context())

	if !notified {
		t.Error("should still notify even when storage delete fails")
	}
	if len(store.deletedIDs) != 1 {
		t.Errorf("expected 1 DB deletion, got %d", len(store.deletedIDs))
	}
}

func TestRetentionSweep_NilNotify(t *testing.T) {
	store := &fakeRetentionStore{
		eligibleIDs: []string{"m-1"},
		deleteRooms: map[string]string{"m-1": "r-1"},
		deleteKeys:  map[string][]string{"m-1": {}},
	}
	storage := &fakeDeletionStorage{}
	cleaner := worker.NewRetentionCleaner(store, storage, nil, zerolog.Nop())

	cleaner.Sweep(t.Context())

	if len(store.deletedIDs) != 1 {
		t.Errorf("expected 1 deletion, got %d", len(store.deletedIDs))
	}
}

func TestRetentionSweep_MultipleStorageKeys(t *testing.T) {
	store := &fakeRetentionStore{
		eligibleIDs: []string{"m-1"},
		deleteRooms: map[string]string{"m-1": "r-1"},
		deleteKeys: map[string][]string{
			"m-1": {"uploads/img.jpg", "thumbnails/img.jpg"},
		},
	}
	storage := &fakeDeletionStorage{}
	cleaner := worker.NewRetentionCleaner(store, storage, nil, zerolog.Nop())

	cleaner.Sweep(t.Context())

	if len(storage.deletedKeys) != 2 {
		t.Errorf("deleted %d keys, want 2", len(storage.deletedKeys))
	}
}

func TestRetentionCleaner_Start_StopsOnContextCancel(t *testing.T) {
	store := &fakeRetentionStore{}
	storage := &fakeDeletionStorage{}
	cleaner := worker.NewRetentionCleaner(store, storage, nil, zerolog.Nop())

	cleaner.Start(t.Context())
}
