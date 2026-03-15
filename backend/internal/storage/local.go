package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const uploadTokenTTL = 15 * time.Minute

type pendingUpload struct {
	params    UploadParams
	expiresAt time.Time
}

// LocalStorage stores files on the local filesystem. It is intended for
// development only — production deployments should use S3Storage.
type LocalStorage struct {
	basePath string // filesystem root, e.g. "./data/uploads"
	baseURL  string // public URL prefix, e.g. "http://localhost:8080/uploads/files"

	mu      sync.Mutex
	pending map[string]pendingUpload // token → pending upload
}

// NewLocalStorage creates a LocalStorage that writes files under basePath and
// serves them under baseURL.
func NewLocalStorage(basePath, baseURL string) *LocalStorage {
	ls := &LocalStorage{
		basePath: basePath,
		baseURL:  baseURL,
		pending:  make(map[string]pendingUpload),
	}
	go ls.cleanupLoop()
	return ls
}

// GenerateUploadURL creates a short-lived upload token and returns a URL the
// client should PUT the file body to.
func (ls *LocalStorage) GenerateUploadURL(_ context.Context, params UploadParams) (*UploadResult, error) {
	token, err := randomToken()
	if err != nil {
		return nil, fmt.Errorf("local storage: generate token: %w", err)
	}

	exp := time.Now().Add(uploadTokenTTL)

	ls.mu.Lock()
	ls.pending[token] = pendingUpload{params: params, expiresAt: exp}
	ls.mu.Unlock()

	return &UploadResult{
		UploadURL: fmt.Sprintf("%s/put/%s", ls.baseURL, token),
		ExpiresAt: &exp,
	}, nil
}

// PublicURL returns the serving URL for a stored file.
func (ls *LocalStorage) PublicURL(key string) string {
	return fmt.Sprintf("%s/%s", ls.baseURL, key)
}

// Delete removes a file from disk.
func (ls *LocalStorage) Delete(_ context.Context, key string) error {
	path := filepath.Join(ls.basePath, filepath.Clean(key))
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("local storage: delete: %w", err)
	}
	return nil
}

// ConsumePendingUpload validates and removes a pending upload token, returning
// the associated params. Called by the upload HTTP handler.
func (ls *LocalStorage) ConsumePendingUpload(token string) (UploadParams, error) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	p, ok := ls.pending[token]
	if !ok {
		return UploadParams{}, fmt.Errorf("local storage: unknown or expired upload token")
	}
	delete(ls.pending, token)

	if time.Now().After(p.expiresAt) {
		return UploadParams{}, fmt.Errorf("local storage: upload token expired")
	}
	return p.params, nil
}

// FilePath returns the absolute filesystem path for a storage key.
func (ls *LocalStorage) FilePath(key string) string {
	return filepath.Join(ls.basePath, filepath.Clean(key))
}

// cleanupLoop periodically removes expired pending upload tokens.
func (ls *LocalStorage) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		ls.mu.Lock()
		now := time.Now()
		for token, p := range ls.pending {
			if now.After(p.expiresAt) {
				delete(ls.pending, token)
			}
		}
		ls.mu.Unlock()
	}
}

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
