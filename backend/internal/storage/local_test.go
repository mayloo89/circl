package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStorage_GenerateUploadURL(t *testing.T) {
	dir := t.TempDir()
	ls := NewLocalStorage(dir, "http://localhost:8080/uploads/files")

	ctx := t.Context()
	result, err := ls.GenerateUploadURL(ctx, UploadParams{
		Key:         "avatar/user1/abc-photo.jpg",
		ContentType: "image/jpeg",
		MaxSize:     5 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("GenerateUploadURL() error: %v", err)
	}
	if result.UploadURL == "" {
		t.Error("expected non-empty UploadURL")
	}
	if result.ExpiresAt == nil {
		t.Error("expected non-nil ExpiresAt")
	}
}

func TestLocalStorage_PublicURL(t *testing.T) {
	ls := NewLocalStorage("/tmp/uploads", "http://localhost:8080/uploads/files")
	url := ls.PublicURL("avatar/user1/photo.jpg")
	want := "http://localhost:8080/uploads/files/avatar/user1/photo.jpg"
	if url != want {
		t.Errorf("PublicURL() = %q, want %q", url, want)
	}
}

func TestLocalStorage_Delete(t *testing.T) {
	dir := t.TempDir()
	ls := NewLocalStorage(dir, "http://localhost:8080/uploads/files")

	// Create a test file.
	key := "avatar/user1/photo.jpg"
	path := filepath.Join(dir, key)
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte("test"), 0o644)

	ctx := t.Context()
	if err := ls.Delete(ctx, key); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected file to be deleted")
	}
}

func TestLocalStorage_Delete_NotExist(t *testing.T) {
	dir := t.TempDir()
	ls := NewLocalStorage(dir, "http://localhost:8080/uploads/files")

	ctx := t.Context()
	if err := ls.Delete(ctx, "nonexistent/file.jpg"); err != nil {
		t.Errorf("Delete() should not error for non-existent file: %v", err)
	}
}

func TestLocalStorage_ConsumePendingUpload(t *testing.T) {
	dir := t.TempDir()
	ls := NewLocalStorage(dir, "http://localhost:8080/uploads/files")

	ctx := t.Context()
	result, err := ls.GenerateUploadURL(ctx, UploadParams{
		Key:         "avatar/user1/abc-photo.jpg",
		ContentType: "image/jpeg",
		MaxSize:     5 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("GenerateUploadURL() error: %v", err)
	}

	// Extract token from URL.
	// URL looks like: http://localhost:8080/uploads/files/put/<token>
	token := filepath.Base(result.UploadURL)

	params, err := ls.ConsumePendingUpload(token)
	if err != nil {
		t.Fatalf("ConsumePendingUpload() error: %v", err)
	}
	if params.Key != "avatar/user1/abc-photo.jpg" {
		t.Errorf("Key = %q, want %q", params.Key, "avatar/user1/abc-photo.jpg")
	}

	// Second consume should fail.
	_, err = ls.ConsumePendingUpload(token)
	if err == nil {
		t.Error("expected error on second consume")
	}
}

func TestLocalStorage_ConsumePendingUpload_UnknownToken(t *testing.T) {
	dir := t.TempDir()
	ls := NewLocalStorage(dir, "http://localhost:8080/uploads/files")

	_, err := ls.ConsumePendingUpload("nonexistent-token")
	if err == nil {
		t.Error("expected error for unknown token")
	}
}

// Compile-time check: LocalStorage implements Storage.
var _ Storage = (*LocalStorage)(nil)
