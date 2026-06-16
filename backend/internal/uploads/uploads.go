// Package uploads manages the lifecycle of file uploads:
// request → upload → confirm.
package uploads

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/storage"
)

var (
	ErrNotFound   = errors.New("uploads: not found")
	ErrNotPending = errors.New("uploads: upload is not in pending state")
	ErrForbidden  = errors.New("uploads: forbidden")
)

// Upload represents a tracked file upload.
type Upload struct {
	ID               string     `json:"id"`
	UserID           string     `json:"user_id"`
	StorageKey       string     `json:"storage_key"`
	Filename         string     `json:"filename"`
	ContentType      string     `json:"content_type"`
	SizeBytes        int64      `json:"size_bytes"`
	Category         string     `json:"category"`
	Status           string     `json:"status"` // "pending" | "committed" | "failed"
	ThumbnailKey     *string    `json:"thumbnail_key,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	CommittedAt      *time.Time `json:"committed_at,omitzero"`
	ModerationStatus string     `json:"moderation_status"`           // "pending" | "approved" | "rejected" | "skipped"
	ModerationCode   string     `json:"moderation_code,omitempty"`   // populated only on rejection
	ModerationReason string     `json:"moderation_reason,omitempty"` // populated only on rejection
	ModeratedAt      *time.Time `json:"moderated_at,omitzero"`
}

// RejectionRecord carries everything the store needs to persist a moderation
// rejection. Score and Categories are populated only by the NSFW classifier;
// the hash-list and heuristic detectors leave them at their zero values.
// FileRetained tells the store whether the storage object was kept (true) or
// purged (false) — when false, uploads.status flips to 'failed' since the
// public URL would point at a missing object.
type RejectionRecord struct {
	UploadID     string
	Code         string
	Reason       string
	Source       string
	Score        float64
	Categories   []string
	FileRetained bool
}

// Store is the persistence contract for uploads.
type Store interface {
	Create(ctx context.Context, u *Upload) error
	GetByID(ctx context.Context, id string) (*Upload, error)
	Commit(ctx context.Context, id string) error
	SetThumbnailKey(ctx context.Context, id, thumbnailKey string) error
	// MarkApproved records that moderation cleared the upload.
	MarkApproved(ctx context.Context, id string) error
	// MarkRejected records a moderation rejection. Source is the originating
	// moderator (e.g. "hashlist:local", "heuristic", "nsfw"). When
	// rec.FileRetained is false the upload's lifecycle status flips to
	// 'failed' (the storage object was purged); when true it stays
	// 'committed' so the admin review tools can still load the file.
	MarkRejected(ctx context.Context, rec RejectionRecord) error
	// MarkQuarantined records a CSAM-class hit. The caller has already moved
	// the original storage object to the restricted quarantineKey; this
	// preserves the audit row, sets moderation_status to 'quarantined', and
	// flips the lifecycle status to 'failed' so the original public URL 404s.
	MarkQuarantined(ctx context.Context, rec RejectionRecord, quarantineKey string) error
	// ListExpiredRetained returns rejected uploads whose retention window
	// has elapsed and whose storage object is still kept for admin review.
	// limit <= 0 falls back to a sensible default in the implementation.
	ListExpiredRetained(ctx context.Context, before time.Time, limit int) ([]RetainedRejection, error)
	// ClearRetention flips moderation_file_retained to FALSE and the
	// lifecycle status to 'failed' after the cleanup worker has purged the
	// storage object. Idempotent — re-running on an already-cleared row is
	// a no-op.
	ClearRetention(ctx context.Context, uploadID string) error
}

// Service is the application-layer that coordinates the storage provider
// and the uploads store.
type Service struct {
	store   Store
	storage storage.Storage
	enqueue func(ctx context.Context, uploadID, storageKey, contentType, category string) error
	log     zerolog.Logger
}

// NewService returns a ready-to-use upload service.
func NewService(store Store, st storage.Storage, log zerolog.Logger) *Service {
	return &Service{
		store:   store,
		storage: st,
		log:     log.With().Str("component", "uploads").Logger(),
	}
}

// SetEnqueuer registers a function that enqueues a background processing task
// after an image upload is confirmed. The function is called asynchronously
// and errors are logged but do not fail the confirm request.
func (s *Service) SetEnqueuer(fn func(ctx context.Context, uploadID, storageKey, contentType, category string) error) {
	s.enqueue = fn
}

// RequestUploadInput is the input for RequestUpload.
type RequestUploadInput struct {
	UserID      string
	Category    string
	Filename    string
	ContentType string
	SizeBytes   int64
}

// RequestUploadOutput is the result of a successful upload request.
type RequestUploadOutput struct {
	UploadID   string     `json:"upload_id"`
	UploadURL  string     `json:"upload_url"`
	StorageKey string     `json:"storage_key"`
	ExpiresAt  *time.Time `json:"expires_at,omitzero"`
}

// RequestUpload validates the upload, creates a pending record, and returns
// a URL the client can PUT the file to.
func (s *Service) RequestUpload(ctx context.Context, in RequestUploadInput) (*RequestUploadOutput, error) {
	cat, err := storage.ParseCategory(in.Category)
	if err != nil {
		return nil, err
	}
	if err := storage.ValidateUpload(cat, in.ContentType, in.SizeBytes); err != nil {
		return nil, err
	}
	filename, err := storage.SanitizeFilename(in.Filename)
	if err != nil {
		return nil, err
	}

	key := buildStorageKey(cat, in.UserID, filename)

	u := &Upload{
		UserID:      in.UserID,
		StorageKey:  key,
		Filename:    filename,
		ContentType: in.ContentType,
		SizeBytes:   in.SizeBytes,
		Category:    in.Category,
		Status:      "pending",
	}
	if err := s.store.Create(ctx, u); err != nil {
		return nil, err
	}

	result, err := s.storage.GenerateUploadURL(ctx, storage.UploadParams{
		Key:         key,
		ContentType: in.ContentType,
		MaxSize:     storage.MaxSize(cat),
	})
	if err != nil {
		return nil, err
	}

	return &RequestUploadOutput{
		UploadID:   u.ID,
		UploadURL:  result.UploadURL,
		StorageKey: key,
		ExpiresAt:  result.ExpiresAt,
	}, nil
}

// ConfirmUploadOutput is the result of a successful upload confirmation.
type ConfirmUploadOutput struct {
	UploadID   string `json:"upload_id"`
	StorageKey string `json:"storage_key"`
	URL        string `json:"url"`
	Status     string `json:"status"`
}

// GetUploadForUser returns an upload by id, scoped to the caller. Used by
// the frontend to poll the moderation outcome after confirming.
func (s *Service) GetUploadForUser(ctx context.Context, uploadID, userID string) (*Upload, error) {
	u, err := s.store.GetByID(ctx, uploadID)
	if err != nil {
		return nil, err
	}
	if u.UserID != userID {
		return nil, ErrForbidden
	}
	return u, nil
}

// IsUploadServable reports whether an upload owned by ownerID may be exposed
// to other users (attached to a message, album, or profile). Image uploads
// must have cleared the moderation pipeline; non-image types are not scanned
// by the image pipeline and are cleared on confirm. This is the attach-time
// gate that keeps an un-moderated, rejected, or quarantined image from ever
// reaching another user.
func (s *Service) IsUploadServable(ctx context.Context, uploadID, ownerID string) (bool, error) {
	u, err := s.GetUploadForUser(ctx, uploadID, ownerID)
	if err != nil {
		return false, err
	}
	if !strings.HasPrefix(u.ContentType, "image/") {
		return true, nil
	}
	return u.ModerationStatus == "approved", nil
}

// ConfirmUpload marks a pending upload as committed. The caller must own
// the upload. For image uploads, a background processing task is enqueued
// to generate a thumbnail and strip EXIF metadata.
func (s *Service) ConfirmUpload(ctx context.Context, uploadID, userID string) (*ConfirmUploadOutput, error) {
	u, err := s.store.GetByID(ctx, uploadID)
	if err != nil {
		return nil, err
	}
	if u.UserID != userID {
		return nil, ErrForbidden
	}
	if u.Status != "pending" {
		return nil, ErrNotPending
	}

	if err := s.store.Commit(ctx, uploadID); err != nil {
		return nil, err
	}

	if s.enqueue != nil && strings.HasPrefix(u.ContentType, "image/") {
		if err := s.enqueue(ctx, u.ID, u.StorageKey, u.ContentType, u.Category); err != nil {
			s.log.Warn().Err(err).Str("upload_id", u.ID).Msg("enqueue image processing failed")
		}
	}

	return &ConfirmUploadOutput{
		UploadID:   u.ID,
		StorageKey: u.StorageKey,
		URL:        s.storage.PublicURL(u.StorageKey),
		Status:     "committed",
	}, nil
}

func buildStorageKey(cat storage.Category, userID, filename string) string {
	return string(cat) + "/" + userID + "/" + randomID() + "-" + filename
}
