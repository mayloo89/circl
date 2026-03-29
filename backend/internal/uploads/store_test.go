package uploads

import (
	"errors"
	"testing"

	"github.com/mayloo89/circl/backend/internal/testutil"
)

func TestPgStore_Integration(t *testing.T) {
	pool := testutil.OpenDB(t)
	store := NewStore(pool)
	ctx := t.Context()

	userID := testutil.CreateUser(t, pool, "uploads_store@test.com")

	t.Run("Create populates ID and CreatedAt", func(t *testing.T) {
		u := &Upload{
			UserID:      userID,
			StorageKey:  "test/avatar-create.jpg",
			Filename:    "avatar.jpg",
			ContentType: "image/jpeg",
			SizeBytes:   1024,
			Category:    "avatar",
			Status:      "pending",
		}
		if err := store.Create(ctx, u); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if u.ID == "" {
			t.Error("expected non-empty ID after Create")
		}
		if u.CreatedAt.IsZero() {
			t.Error("expected non-zero CreatedAt after Create")
		}
		t.Cleanup(func() {
			pool.Exec(t.Context(), `DELETE FROM uploads WHERE id = $1`, u.ID) //nolint:errcheck
		})
	})

	t.Run("GetByID returns the created upload", func(t *testing.T) {
		u := &Upload{
			UserID:      userID,
			StorageKey:  "test/avatar-get.jpg",
			Filename:    "avatar.jpg",
			ContentType: "image/jpeg",
			SizeBytes:   512,
			Category:    "avatar",
			Status:      "pending",
		}
		if err := store.Create(ctx, u); err != nil {
			t.Fatalf("Create: %v", err)
		}
		t.Cleanup(func() {
			pool.Exec(t.Context(), `DELETE FROM uploads WHERE id = $1`, u.ID) //nolint:errcheck
		})

		got, err := store.GetByID(ctx, u.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got.StorageKey != u.StorageKey {
			t.Errorf("StorageKey = %q, want %q", got.StorageKey, u.StorageKey)
		}
		if got.Status != "pending" {
			t.Errorf("Status = %q, want pending", got.Status)
		}
	})

	t.Run("GetByID returns ErrNotFound for unknown ID", func(t *testing.T) {
		_, err := store.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("Commit transitions status to committed", func(t *testing.T) {
		u := &Upload{
			UserID:      userID,
			StorageKey:  "test/avatar-commit.jpg",
			Filename:    "avatar.jpg",
			ContentType: "image/jpeg",
			SizeBytes:   2048,
			Category:    "avatar",
			Status:      "pending",
		}
		if err := store.Create(ctx, u); err != nil {
			t.Fatalf("Create: %v", err)
		}
		t.Cleanup(func() {
			pool.Exec(t.Context(), `DELETE FROM uploads WHERE id = $1`, u.ID) //nolint:errcheck
		})

		if err := store.Commit(ctx, u.ID); err != nil {
			t.Fatalf("Commit: %v", err)
		}

		got, err := store.GetByID(ctx, u.ID)
		if err != nil {
			t.Fatalf("GetByID after Commit: %v", err)
		}
		if got.Status != "committed" {
			t.Errorf("Status = %q, want committed", got.Status)
		}
		if got.CommittedAt == nil {
			t.Error("expected non-nil CommittedAt after Commit")
		}
	})

	t.Run("Commit already-committed upload returns ErrNotPending", func(t *testing.T) {
		u := &Upload{
			UserID:      userID,
			StorageKey:  "test/avatar-committed.jpg",
			Filename:    "avatar.jpg",
			ContentType: "image/jpeg",
			SizeBytes:   512,
			Category:    "avatar",
			Status:      "pending",
		}
		if err := store.Create(ctx, u); err != nil {
			t.Fatalf("Create: %v", err)
		}
		t.Cleanup(func() {
			pool.Exec(t.Context(), `DELETE FROM uploads WHERE id = $1`, u.ID) //nolint:errcheck
		})

		if err := store.Commit(ctx, u.ID); err != nil {
			t.Fatalf("first Commit: %v", err)
		}
		if err := store.Commit(ctx, u.ID); !errors.Is(err, ErrNotPending) {
			t.Errorf("second Commit: err = %v, want ErrNotPending", err)
		}
	})

	t.Run("SetThumbnailKey stores the key", func(t *testing.T) {
		u := &Upload{
			UserID:      userID,
			StorageKey:  "test/avatar-thumb.jpg",
			Filename:    "avatar.jpg",
			ContentType: "image/jpeg",
			SizeBytes:   512,
			Category:    "avatar",
			Status:      "pending",
		}
		if err := store.Create(ctx, u); err != nil {
			t.Fatalf("Create: %v", err)
		}
		t.Cleanup(func() {
			pool.Exec(t.Context(), `DELETE FROM uploads WHERE id = $1`, u.ID) //nolint:errcheck
		})

		if err := store.SetThumbnailKey(ctx, u.ID, "test/avatar-thumb_480.jpg"); err != nil {
			t.Fatalf("SetThumbnailKey: %v", err)
		}

		got, err := store.GetByID(ctx, u.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got.ThumbnailKey == nil || *got.ThumbnailKey != "test/avatar-thumb_480.jpg" {
			t.Errorf("ThumbnailKey = %v, want test/avatar-thumb_480.jpg", got.ThumbnailKey)
		}
	})

	t.Run("SetThumbnailKey returns ErrNotFound for unknown ID", func(t *testing.T) {
		err := store.SetThumbnailKey(ctx, "00000000-0000-0000-0000-000000000000", "key")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}
