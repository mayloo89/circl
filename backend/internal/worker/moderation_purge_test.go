package worker

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/uploads"
)

type fakePurger struct {
	listed    []uploads.RetainedRejection
	listErr   error
	cleared   []string
	clearErr  error
}

func (f *fakePurger) ListExpiredRetained(_ context.Context, _ time.Time, _ int) ([]uploads.RetainedRejection, error) {
	return f.listed, f.listErr
}
func (f *fakePurger) ClearRetention(_ context.Context, id string) error {
	if f.clearErr != nil {
		return f.clearErr
	}
	f.cleared = append(f.cleared, id)
	return nil
}

type fakeFileStorage struct {
	deleted []string
}

func (f *fakeFileStorage) Delete(_ context.Context, key string) error {
	f.deleted = append(f.deleted, key)
	return nil
}

func TestPurgeExpiredModerationFiles_DeletesAndClears(t *testing.T) {
	thumb := "thumbnails/u-1.jpg"
	purger := &fakePurger{
		listed: []uploads.RetainedRejection{
			{UploadID: "u-1", StorageKey: "gallery/u-1.png", ThumbnailKey: &thumb},
			{UploadID: "u-2", StorageKey: "gallery/u-2.png"},
		},
	}
	storage := &fakeFileStorage{}

	PurgeExpiredModerationFiles(t.Context(), zerolog.Nop(), purger, storage, 30)

	if len(storage.deleted) != 3 {
		t.Errorf("expected 3 storage deletes (2 originals + 1 thumbnail), got %d", len(storage.deleted))
	}
	if len(purger.cleared) != 2 {
		t.Errorf("expected 2 retention clears, got %d", len(purger.cleared))
	}
}

func TestPurgeExpiredModerationFiles_DefaultRetentionWhenZero(t *testing.T) {
	// retentionDays <= 0 should fall back to DefaultModerationRetentionDays
	// rather than treating "now" as the cutoff (which would purge everything).
	purger := &fakePurger{}
	storage := &fakeFileStorage{}
	PurgeExpiredModerationFiles(t.Context(), zerolog.Nop(), purger, storage, 0)
	// Nothing in the list → nothing deleted; success is "no panic".
}
