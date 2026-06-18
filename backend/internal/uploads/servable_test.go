package uploads

import (
	"testing"

	"github.com/rs/zerolog"
)

func newServableSvc(t *testing.T) (*Service, *mockStore) {
	t.Helper()
	store := newMockStore()
	return NewService(store, nil, zerolog.Nop()), store
}

func TestIsKeyServable(t *testing.T) {
	tests := []struct {
		name    string
		upload  *Upload
		ownerID string
		want    bool
	}{
		{
			name:    "approved committed image is servable",
			upload:  &Upload{UserID: "u1", StorageKey: "k1", ContentType: "image/jpeg", Status: "committed", ModerationStatus: "approved"},
			ownerID: "u1",
			want:    true,
		},
		{
			name:    "pending image is not servable",
			upload:  &Upload{UserID: "u1", StorageKey: "k1", ContentType: "image/jpeg", Status: "committed", ModerationStatus: "pending"},
			ownerID: "u1",
			want:    false,
		},
		{
			name:    "quarantined image is not servable",
			upload:  &Upload{UserID: "u1", StorageKey: "k1", ContentType: "image/jpeg", Status: "failed", ModerationStatus: "quarantined"},
			ownerID: "u1",
			want:    false,
		},
		{
			name:    "uppercase image content-type still gated (no bypass)",
			upload:  &Upload{UserID: "u1", StorageKey: "k1", ContentType: "Image/JPEG", Status: "committed", ModerationStatus: "pending"},
			ownerID: "u1",
			want:    false,
		},
		{
			name:    "committed non-image is servable",
			upload:  &Upload{UserID: "u1", StorageKey: "k1", ContentType: "application/pdf", Status: "committed", ModerationStatus: "pending"},
			ownerID: "u1",
			want:    true,
		},
		{
			name:    "uncommitted non-image is not servable",
			upload:  &Upload{UserID: "u1", StorageKey: "k1", ContentType: "application/pdf", Status: "pending", ModerationStatus: "pending"},
			ownerID: "u1",
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, store := newServableSvc(t)
			store.uploads["id1"] = tt.upload
			got, err := svc.IsKeyServable(t.Context(), tt.upload.StorageKey, tt.ownerID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("IsKeyServable = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsKeyServable_ClientInputErrorsBlockWithoutFault(t *testing.T) {
	svc, store := newServableSvc(t)
	store.uploads["id1"] = &Upload{UserID: "owner", StorageKey: "k1", ContentType: "image/jpeg", Status: "committed", ModerationStatus: "approved"}

	t.Run("unknown key blocks without error", func(t *testing.T) {
		ok, err := svc.IsKeyServable(t.Context(), "does-not-exist", "owner")
		if err != nil || ok {
			t.Errorf("got (%v, %v), want (false, nil)", ok, err)
		}
	})

	t.Run("ownership mismatch blocks without error", func(t *testing.T) {
		ok, err := svc.IsKeyServable(t.Context(), "k1", "someone-else")
		if err != nil || ok {
			t.Errorf("got (%v, %v), want (false, nil)", ok, err)
		}
	})
}

func TestIsUploadServable_ClientInputErrorsBlockWithoutFault(t *testing.T) {
	svc, store := newServableSvc(t)
	store.uploads["up1"] = &Upload{UserID: "owner", StorageKey: "k1", ContentType: "image/jpeg", Status: "committed", ModerationStatus: "approved"}

	t.Run("missing upload blocks without error", func(t *testing.T) {
		ok, err := svc.IsUploadServable(t.Context(), "missing", "owner")
		if err != nil || ok {
			t.Errorf("got (%v, %v), want (false, nil)", ok, err)
		}
	})

	t.Run("ownership mismatch blocks without error", func(t *testing.T) {
		ok, err := svc.IsUploadServable(t.Context(), "up1", "someone-else")
		if err != nil || ok {
			t.Errorf("got (%v, %v), want (false, nil)", ok, err)
		}
	})
}
