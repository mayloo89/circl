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

func TestServeHandler_NeverServesRestrictedKeysOrDirectories(t *testing.T) {
	dir := t.TempDir()
	ls := NewLocalStorage(dir, "http://x/uploads/files")
	for _, key := range []string{"quarantine/chat/a.jpg", "evidence/r1/f1.jpg", "exports/u1/r1.zip", "chat/ok.jpg"} {
		p := filepath.Join(dir, filepath.FromSlash(key))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("bytes"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	h := NewLocalHandler(ls)
	get := func(target string) int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		return rec.Code
	}

	for _, target := range []string{
		"/quarantine/chat/a.jpg",
		"/evidence/r1/f1.jpg",
		"/exports/u1/r1.zip",
		"//quarantine/chat/a.jpg",
		"/./evidence/r1/f1.jpg",
		"/Evidence/r1/f1.jpg",
		"/chat/",
		"/quarantine/",
	} {
		if code := get(target); code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", target, code)
		}
	}
	if code := get("/chat/ok.jpg"); code != http.StatusOK {
		t.Errorf("GET /chat/ok.jpg = %d, want 200", code)
	}
}

func TestIsRestrictedKey(t *testing.T) {
	cases := map[string]bool{
		"quarantine/a.jpg": true,
		"evidence/r/f.jpg": true,
		"exports/u/r.zip":  true,
		"/evidence/x":      true,
		"EXPORTS/u/r.zip":  true,
		"evidence":         true,
		"chat/evidence/x":  false,
		"evidence-like/x":  false,
		"avatars/u/a.jpg":  false,
		"thumbnails/c.jpg": false,
	}
	for key, want := range cases {
		if got := IsRestrictedKey(key); got != want {
			t.Errorf("IsRestrictedKey(%q) = %v, want %v", key, got, want)
		}
	}
}
