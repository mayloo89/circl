package worker

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockAccountPurger struct {
	n   int64
	err error
}

func (m *mockAccountPurger) PurgeExpiredDeletedUsers(_ context.Context, _ time.Time) (int64, error) {
	return m.n, m.err
}

func TestPurgeDeletedAccounts_NoneExpired(t *testing.T) {
	store := &mockAccountPurger{n: 0}
	PurgeDeletedAccounts(t.Context(), store) // should not panic or log error
}

func TestPurgeDeletedAccounts_SomePurged(t *testing.T) {
	store := &mockAccountPurger{n: 3}
	PurgeDeletedAccounts(t.Context(), store) // should log count, not error
}

func TestPurgeDeletedAccounts_StoreError(t *testing.T) {
	store := &mockAccountPurger{err: errors.New("db error")}
	PurgeDeletedAccounts(t.Context(), store) // should log error, not panic
}
