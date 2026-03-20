package storage

import (
	"io"
	"os"
	"path/filepath"
	"strings"
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

func TestLocalStorage_GetObject_Success(t *testing.T) {
	dir := t.TempDir()
	ls := NewLocalStorage(dir, "http://localhost:8080/uploads/files")

	key := "chat-attachment/user1/file.jpg"
	path := filepath.Join(dir, key)
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte("image bytes"), 0o644)

	rc, err := ls.GetObject(t.Context(), key)
	if err != nil {
		t.Fatalf("GetObject() error: %v", err)
	}
	defer rc.Close()
	data, _ := io.ReadAll(rc)
	if string(data) != "image bytes" {
		t.Errorf("data = %q, want image bytes", data)
	}
}

func TestLocalStorage_GetObject_NotFound(t *testing.T) {
	dir := t.TempDir()
	ls := NewLocalStorage(dir, "http://localhost:8080/uploads/files")

	_, err := ls.GetObject(t.Context(), "nonexistent/file.jpg")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLocalStorage_PutObject(t *testing.T) {
	dir := t.TempDir()
	ls := NewLocalStorage(dir, "http://localhost:8080/uploads/files")

	key := "thumbnails/chat-attachment/user1/file.jpg"
	body := strings.NewReader("thumbnail data")
	if err := ls.PutObject(t.Context(), key, "image/jpeg", body, int64(body.Len())); err != nil {
		t.Fatalf("PutObject() error: %v", err)
	}

	path := filepath.Join(dir, key)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(data) != "thumbnail data" {
		t.Errorf("data = %q, want thumbnail data", data)
	}
}

func TestLocalStorage_PutObject_Overwrites(t *testing.T) {
	dir := t.TempDir()
	ls := NewLocalStorage(dir, "http://localhost:8080/uploads/files")

	key := "chat-attachment/user1/file.jpg"
	path := filepath.Join(dir, key)
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte("old content"), 0o644)

	ls.PutObject(t.Context(), key, "image/jpeg", strings.NewReader("new content"), 11) //nolint:errcheck

	data, _ := os.ReadFile(path)
	if string(data) != "new content" {
		t.Errorf("data = %q, want new content", data)
	}
}

// Compile-time check: LocalStorage implements Storage.
var _ Storage = (*LocalStorage)(nil)
