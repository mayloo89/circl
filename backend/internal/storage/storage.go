// Package storage defines the Storage interface for file persistence.
// Implementations (LocalStorage, S3Storage) are selected at startup via config.
package storage

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when a storage key does not exist.
var ErrNotFound = errors.New("storage: not found")

// UploadParams describes a file the client wants to upload.
type UploadParams struct {
	Key         string // server-generated storage key
	ContentType string
	MaxSize     int64
}

// UploadResult is returned by GenerateUploadURL with the URL the client should
// PUT/POST the file to.
type UploadResult struct {
	UploadURL string     // URL the client uploads to
	ExpiresAt *time.Time // optional expiry for pre-signed URLs
}

// Storage is the provider-agnostic interface for file operations.
// LocalStorage and S3Storage both implement this interface.
type Storage interface {
	// GenerateUploadURL returns a URL the client can PUT the file to.
	// For LocalStorage this is a backend endpoint; for S3 it is a pre-signed URL.
	GenerateUploadURL(ctx context.Context, params UploadParams) (*UploadResult, error)

	// PublicURL returns the serving URL for a stored file.
	PublicURL(key string) string

	// Delete removes a file from storage.
	Delete(ctx context.Context, key string) error
}
