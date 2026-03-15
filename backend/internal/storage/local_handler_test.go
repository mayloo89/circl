package storage

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalHandler_UploadAndServe(t *testing.T) {
	dir := t.TempDir()
	ls := NewLocalStorage(dir, "http://test/uploads/files")
	handler := NewLocalHandler(ls)

	ctx := t.Context()

	// Generate an upload URL.
	result, err := ls.GenerateUploadURL(ctx, UploadParams{
		Key:         "avatar/user1/photo.jpg",
		ContentType: "image/jpeg",
		MaxSize:     5 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("GenerateUploadURL() error: %v", err)
	}

	token := filepath.Base(result.UploadURL)

	// Upload the file.
	body := bytes.NewReader([]byte("fake jpeg content"))
	req := httptest.NewRequest(http.MethodPut, "/put/"+token, body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("upload status = %d, want %d; body: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	// Verify the file exists on disk.
	path := filepath.Join(dir, "avatar/user1/photo.jpg")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not found: %v", err)
	}
	if string(data) != "fake jpeg content" {
		t.Errorf("file content = %q, want %q", string(data), "fake jpeg content")
	}

	// Serve the file.
	req = httptest.NewRequest(http.MethodGet, "/avatar/user1/photo.jpg", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("serve status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "fake jpeg content" {
		t.Errorf("served content = %q, want %q", rec.Body.String(), "fake jpeg content")
	}
}

func TestLocalHandler_UploadInvalidToken(t *testing.T) {
	dir := t.TempDir()
	ls := NewLocalStorage(dir, "http://test/uploads/files")
	handler := NewLocalHandler(ls)

	body := bytes.NewReader([]byte("content"))
	req := httptest.NewRequest(http.MethodPut, "/put/invalid-token", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLocalHandler_ServeNotFound(t *testing.T) {
	dir := t.TempDir()
	ls := NewLocalStorage(dir, "http://test/uploads/files")
	handler := NewLocalHandler(ls)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent/file.jpg", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestLocalHandler_ServePathTraversal(t *testing.T) {
	dir := t.TempDir()
	ls := NewLocalStorage(dir, "http://test/uploads/files")
	handler := NewLocalHandler(ls)

	req := httptest.NewRequest(http.MethodGet, "/../../etc/passwd", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d for path traversal", rec.Code, http.StatusNotFound)
	}
}

