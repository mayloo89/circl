package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// --- mocks ---

type mockAccountPurger struct {
	ids          []string
	getIDsErr    error
	uploadKeys   []string
	thumbKeys    []string
	getKeysErr   error
	deleteDataErr error
	anonymizeErr error
}

func (m *mockAccountPurger) GetExpiredDeletedUserIDs(_ context.Context, _ time.Time) ([]string, error) {
	return m.ids, m.getIDsErr
}

func (m *mockAccountPurger) GetUserUploadKeys(_ context.Context, _ string) ([]string, []string, error) {
	return m.uploadKeys, m.thumbKeys, m.getKeysErr
}

func (m *mockAccountPurger) DeleteUserData(_ context.Context, _ string) error {
	return m.deleteDataErr
}

func (m *mockAccountPurger) AnonymizeUser(_ context.Context, _ string) error {
	return m.anonymizeErr
}

type mockFileStorage struct {
	deleted []string
	err     error
}

func (m *mockFileStorage) Delete(_ context.Context, key string) error {
	m.deleted = append(m.deleted, key)
	return m.err
}

// --- PurgeDeletedAccounts ---

func TestPurgeDeletedAccounts_NoneExpired(t *testing.T) {
	store := &mockAccountPurger{ids: nil}
	fs := &mockFileStorage{}
	PurgeDeletedAccounts(t.Context(), zerolog.Nop(), store, fs) // should not panic or log error
}

func TestPurgeDeletedAccounts_GetIDsError(t *testing.T) {
	store := &mockAccountPurger{getIDsErr: errors.New("db error")}
	fs := &mockFileStorage{}
	PurgeDeletedAccounts(t.Context(), zerolog.Nop(), store, fs) // should log error, not panic
}

func TestPurgeDeletedAccounts_PurgesUsers(t *testing.T) {
	store := &mockAccountPurger{
		ids:        []string{"uid-1", "uid-2"},
		uploadKeys: []string{"uploads/img.jpg"},
		thumbKeys:  []string{"uploads/img_thumb.jpg"},
	}
	fs := &mockFileStorage{}
	PurgeDeletedAccounts(t.Context(), zerolog.Nop(), store, fs)

	// Each user has 1 storage key + 1 thumbnail = 2 deletions per user × 2 users = 4.
	if len(fs.deleted) != 4 {
		t.Errorf("storage deletes = %d, want 4", len(fs.deleted))
	}
}

func TestPurgeDeletedAccounts_StorageErrorDoesNotAbort(t *testing.T) {
	store := &mockAccountPurger{
		ids:        []string{"uid-1"},
		uploadKeys: []string{"uploads/img.jpg"},
		thumbKeys:  []string{},
	}
	fs := &mockFileStorage{err: errors.New("s3 error")}
	// Should complete without panic even when storage.Delete fails.
	PurgeDeletedAccounts(t.Context(), zerolog.Nop(), store, fs)
}

func TestPurgeDeletedAccounts_DeleteDataError(t *testing.T) {
	store := &mockAccountPurger{
		ids:           []string{"uid-1"},
		deleteDataErr: errors.New("db error"),
	}
	fs := &mockFileStorage{}
	PurgeDeletedAccounts(t.Context(), zerolog.Nop(), store, fs) // logs error, does not panic
}

func TestPurgeDeletedAccounts_AnonymizeError(t *testing.T) {
	store := &mockAccountPurger{
		ids:          []string{"uid-1"},
		anonymizeErr: errors.New("db error"),
	}
	fs := &mockFileStorage{}
	PurgeDeletedAccounts(t.Context(), zerolog.Nop(), store, fs) // logs error, does not panic
}

func TestPurgeDeletedAccounts_EmptyStorageKeysSkipped(t *testing.T) {
	store := &mockAccountPurger{
		ids:        []string{"uid-1"},
		uploadKeys: []string{""},
		thumbKeys:  []string{""},
	}
	fs := &mockFileStorage{}
	PurgeDeletedAccounts(t.Context(), zerolog.Nop(), store, fs)

	if len(fs.deleted) != 0 {
		t.Errorf("expected no storage deletions for empty keys, got %v", fs.deleted)
	}
}
