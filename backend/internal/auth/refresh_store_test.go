package auth

import (
	"context"
	"testing"
	"time"
)

// stubRefreshStore is an in-memory RefreshTokenStore used in unit tests.
type stubRefreshStore struct {
	tokens   map[string]*refreshTokenRecord
	revoked  map[string]int64 // userID → UnixNano
}

func newStubRefreshStore() *stubRefreshStore {
	return &stubRefreshStore{
		tokens:  make(map[string]*refreshTokenRecord),
		revoked: make(map[string]int64),
	}
}

func (s *stubRefreshStore) Create(_ context.Context, hash, userID, role string, issuedAt time.Time) error {
	s.tokens[hash] = &refreshTokenRecord{UserID: userID, Role: role, IssuedAt: issuedAt}
	return nil
}

func (s *stubRefreshStore) Get(_ context.Context, hash string) (*refreshTokenRecord, error) {
	rt, ok := s.tokens[hash]
	if !ok {
		return nil, ErrInvalidToken
	}
	if ts, revoked := s.revoked[rt.UserID]; revoked && rt.IssuedAt.UnixNano() <= ts {
		delete(s.tokens, hash)
		return nil, ErrInvalidToken
	}
	return rt, nil
}

func (s *stubRefreshStore) Delete(_ context.Context, hash string) error {
	delete(s.tokens, hash)
	return nil
}

func (s *stubRefreshStore) RevokeAllForUser(_ context.Context, userID string) error {
	s.revoked[userID] = time.Now().UnixNano()
	return nil
}

func TestStubRefreshStore_CreateGet(t *testing.T) {
	ctx := t.Context()
	store := newStubRefreshStore()

	if err := store.Create(ctx, "hash1", "user1", "user", time.Now()); err != nil {
		t.Fatal(err)
	}
	rt, err := store.Get(ctx, "hash1")
	if err != nil {
		t.Fatal(err)
	}
	if rt.UserID != "user1" || rt.Role != "user" {
		t.Errorf("unexpected record: %+v", rt)
	}
}

func TestStubRefreshStore_NotFound(t *testing.T) {
	_, err := newStubRefreshStore().Get(t.Context(), "nonexistent")
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestStubRefreshStore_Delete(t *testing.T) {
	ctx := t.Context()
	store := newStubRefreshStore()
	_ = store.Create(ctx, "hash1", "user1", "user", time.Now())
	_ = store.Delete(ctx, "hash1")
	if _, err := store.Get(ctx, "hash1"); err != ErrInvalidToken {
		t.Error("expected token to be gone after Delete")
	}
}

func TestStubRefreshStore_RevokeAllForUser(t *testing.T) {
	ctx := t.Context()
	store := newStubRefreshStore()

	issuedAt := time.Now().Add(-time.Minute)
	_ = store.Create(ctx, "old", "user1", "user", issuedAt)

	_ = store.RevokeAllForUser(ctx, "user1")

	// Token issued before revocation must be rejected.
	if _, err := store.Get(ctx, "old"); err != ErrInvalidToken {
		t.Error("expected old token to be revoked")
	}

	// Token issued after revocation must be accepted.
	_ = store.Create(ctx, "new", "user1", "user", time.Now())
	if _, err := store.Get(ctx, "new"); err != nil {
		t.Errorf("new token should be valid: %v", err)
	}
}
