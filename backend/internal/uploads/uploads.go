// Package uploads manages the lifecycle of file uploads:
// request → upload → confirm.
package uploads

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/mayloo89/circl/backend/internal/storage"
)

var (
	ErrNotFound   = errors.New("uploads: not found")
	ErrNotPending = errors.New("uploads: upload is not in pending state")
	ErrForbidden  = errors.New("uploads: forbidden")
)

// Upload represents a tracked file upload.
type Upload struct {
	ID           string     `json:"id"`
	UserID       string     `json:"user_id"`
	StorageKey   string     `json:"storage_key"`
	Filename     string     `json:"filename"`
	ContentType  string     `json:"content_type"`
	SizeBytes    int64      `json:"size_bytes"`
	Category     string     `json:"category"`
	Status       string     `json:"status"` // "pending" or "committed"
	ThumbnailKey *string    `json:"thumbnail_key,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	CommittedAt  *time.Time `json:"committed_at,omitempty"`
}

// Store is the persistence contract for uploads.
type Store interface {
	Create(ctx context.Context, u *Upload) error
	GetByID(ctx context.Context, id string) (*Upload, error)
	Commit(ctx context.Context, id string) error
	SetThumbnailKey(ctx context.Context, id, thumbnailKey string) error
}

// Service is the application-layer that coordinates the storage provider
// and the uploads store.
type Service struct {
	store   Store
	storage storage.Storage
	enqueue func(ctx context.Context, uploadID, storageKey, contentType string) error
}

// NewService returns a ready-to-use upload service.
func NewService(store Store, st storage.Storage) *Service {
	return &Service{store: store, storage: st}
}

// SetEnqueuer registers a function that enqueues a background processing task
// after an image upload is confirmed. The function is called asynchronously
// and errors are logged but do not fail the confirm request.
func (s *Service) SetEnqueuer(fn func(ctx context.Context, uploadID, storageKey, contentType string) error) {
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
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
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
		if err := s.enqueue(ctx, u.ID, u.StorageKey, u.ContentType); err != nil {
			log.Printf("uploads: enqueue image processing for %s: %v", u.ID, err)
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
