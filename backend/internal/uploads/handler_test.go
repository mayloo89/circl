package uploads

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/storage"
)

// --- Test doubles ---

type mockStore struct {
	uploads map[string]*Upload
	idSeq   int
}

func newMockStore() *mockStore {
	return &mockStore{uploads: make(map[string]*Upload)}
}

func (m *mockStore) Create(_ context.Context, u *Upload) error {
	m.idSeq++
	u.ID = "upload-" + string(rune('0'+m.idSeq))
	u.CreatedAt = time.Now()
	m.uploads[u.ID] = u
	return nil
}

func (m *mockStore) GetByID(_ context.Context, id string) (*Upload, error) {
	u, ok := m.uploads[id]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

func (m *mockStore) Commit(_ context.Context, id string) error {
	u, ok := m.uploads[id]
	if !ok {
		return ErrNotPending
	}
	if u.Status != "pending" {
		return ErrNotPending
	}
	u.Status = "committed"
	now := time.Now()
	u.CommittedAt = &now
	return nil
}

func (m *mockStore) SetThumbnailKey(_ context.Context, id, key string) error {
	u, ok := m.uploads[id]
	if !ok {
		return ErrNotFound
	}
	u.ThumbnailKey = &key
	return nil
}

func (m *mockStore) MarkApproved(_ context.Context, _ string) error          { return nil }
func (m *mockStore) MarkRejected(_ context.Context, _ RejectionRecord) error { return nil }
func (m *mockStore) ListExpiredRetained(_ context.Context, _ time.Time, _ int) ([]RetainedRejection, error) {
	return nil, nil
}
func (m *mockStore) ClearRetention(_ context.Context, _ string) error { return nil }

type mockStorage struct{}

func (m *mockStorage) GenerateUploadURL(_ context.Context, params storage.UploadParams) (*storage.UploadResult, error) {
	return &storage.UploadResult{UploadURL: "http://test/upload/" + params.Key}, nil
}

func (m *mockStorage) PublicURL(key string) string {
	return "http://test/files/" + key
}

func (m *mockStorage) Delete(_ context.Context, _ string) error { return nil }

func (m *mockStorage) GetObject(_ context.Context, _ string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *mockStorage) PutObject(_ context.Context, _, _ string, _ io.Reader, _ int64) error {
	return nil
}

// failingMockStorage returns an error from GenerateUploadURL.
type failingMockStorage struct{ mockStorage }

func (f *failingMockStorage) GenerateUploadURL(_ context.Context, _ storage.UploadParams) (*storage.UploadResult, error) {
	return nil, errors.New("storage unavailable")
}

// failingCommitStore is a mockStore where Commit always returns a DB error.
type failingCommitStore struct {
	mockStore
}

func (f *failingCommitStore) Commit(_ context.Context, _ string) error {
	return errors.New("db error")
}

// --- Helpers ---

func withAuth(r *http.Request, userID string) *http.Request {
	ctx := middleware.ContextWithUserID(r.Context(), userID)
	return r.WithContext(ctx)
}

func chiContext(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// --- Tests ---

func TestRequestUpload_Success(t *testing.T) {
	svc := NewService(newMockStore(), &mockStorage{}, zerolog.Nop())
	handler := NewHandler(svc)

	body := `{"category":"avatar","filename":"photo.jpg","content_type":"image/jpeg","size_bytes":1024}`
	req := httptest.NewRequest(http.MethodPost, "/request", bytes.NewBufferString(body))
	req = withAuth(req, "user-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp RequestUploadOutput
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.UploadID == "" {
		t.Error("expected non-empty upload_id")
	}
	if resp.UploadURL == "" {
		t.Error("expected non-empty upload_url")
	}
	if resp.StorageKey == "" {
		t.Error("expected non-empty storage_key")
	}
}

func TestRequestUpload_NoAuth(t *testing.T) {
	svc := NewService(newMockStore(), &mockStorage{}, zerolog.Nop())
	handler := NewHandler(svc)

	body := `{"category":"avatar","filename":"photo.jpg","content_type":"image/jpeg","size_bytes":1024}`
	req := httptest.NewRequest(http.MethodPost, "/request", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequestUpload_InvalidCategory(t *testing.T) {
	svc := NewService(newMockStore(), &mockStorage{}, zerolog.Nop())
	handler := NewHandler(svc)

	body := `{"category":"bogus","filename":"photo.jpg","content_type":"image/jpeg","size_bytes":1024}`
	req := httptest.NewRequest(http.MethodPost, "/request", bytes.NewBufferString(body))
	req = withAuth(req, "user-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRequestUpload_InvalidContentType(t *testing.T) {
	svc := NewService(newMockStore(), &mockStorage{}, zerolog.Nop())
	handler := NewHandler(svc)

	body := `{"category":"avatar","filename":"script.sh","content_type":"application/x-sh","size_bytes":1024}`
	req := httptest.NewRequest(http.MethodPost, "/request", bytes.NewBufferString(body))
	req = withAuth(req, "user-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRequestUpload_FileTooLarge(t *testing.T) {
	svc := NewService(newMockStore(), &mockStorage{}, zerolog.Nop())
	handler := NewHandler(svc)

	body := `{"category":"avatar","filename":"photo.jpg","content_type":"image/jpeg","size_bytes":10000000}`
	req := httptest.NewRequest(http.MethodPost, "/request", bytes.NewBufferString(body))
	req = withAuth(req, "user-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestConfirmUpload_Success(t *testing.T) {
	store := newMockStore()
	svc := NewService(store, &mockStorage{}, zerolog.Nop())

	// First create a pending upload via the service.
	out, err := svc.RequestUpload(t.Context(), RequestUploadInput{
		UserID:      "user-1",
		Category:    "avatar",
		Filename:    "photo.jpg",
		ContentType: "image/jpeg",
		SizeBytes:   1024,
	})
	if err != nil {
		t.Fatalf("RequestUpload() error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/"+out.UploadID+"/confirm", nil)
	req = withAuth(req, "user-1")
	req = chiContext(req, "id", out.UploadID)
	rec := httptest.NewRecorder()

	// Use chi router to handle URL params properly.
	r := chi.NewRouter()
	r.Post("/{id}/confirm", confirmHandler(svc))
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp ConfirmUploadOutput
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Status != "committed" {
		t.Errorf("status = %q, want %q", resp.Status, "committed")
	}
	if resp.URL == "" {
		t.Error("expected non-empty url")
	}
}

func TestConfirmUpload_NotFound(t *testing.T) {
	svc := NewService(newMockStore(), &mockStorage{}, zerolog.Nop())

	r := chi.NewRouter()
	r.Post("/{id}/confirm", confirmHandler(svc))

	req := httptest.NewRequest(http.MethodPost, "/nonexistent/confirm", nil)
	req = withAuth(req, "user-1")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestConfirmUpload_Forbidden(t *testing.T) {
	store := newMockStore()
	svc := NewService(store, &mockStorage{}, zerolog.Nop())

	out, _ := svc.RequestUpload(t.Context(), RequestUploadInput{
		UserID:      "user-1",
		Category:    "avatar",
		Filename:    "photo.jpg",
		ContentType: "image/jpeg",
		SizeBytes:   1024,
	})

	r := chi.NewRouter()
	r.Post("/{id}/confirm", confirmHandler(svc))

	req := httptest.NewRequest(http.MethodPost, "/"+out.UploadID+"/confirm", nil)
	req = withAuth(req, "user-2") // Different user
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestConfirmUpload_AlreadyCommitted(t *testing.T) {
	store := newMockStore()
	svc := NewService(store, &mockStorage{}, zerolog.Nop())

	out, _ := svc.RequestUpload(t.Context(), RequestUploadInput{
		UserID: "user-1", Category: "avatar", Filename: "photo.jpg",
		ContentType: "image/jpeg", SizeBytes: 1024,
	})
	svc.ConfirmUpload(t.Context(), out.UploadID, "user-1") //nolint:errcheck

	r := chi.NewRouter()
	r.Post("/{id}/confirm", confirmHandler(svc))

	req := httptest.NewRequest(http.MethodPost, "/"+out.UploadID+"/confirm", nil)
	req = withAuth(req, "user-1")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestConfirmUpload_NoAuth(t *testing.T) {
	svc := NewService(newMockStore(), &mockStorage{}, zerolog.Nop())
	r := chi.NewRouter()
	r.Post("/{id}/confirm", confirmHandler(svc))

	req := httptest.NewRequest(http.MethodPost, "/some-id/confirm", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequestUpload_InvalidJSON(t *testing.T) {
	svc := NewService(newMockStore(), &mockStorage{}, zerolog.Nop())
	handler := NewHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/request", bytes.NewBufferString("not json"))
	req = withAuth(req, "user-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestConfirmUpload_EnqueuerCalled(t *testing.T) {
	store := newMockStore()
	svc := NewService(store, &mockStorage{}, zerolog.Nop())

	var enqueuedUploadID string
	svc.SetEnqueuer(func(_ context.Context, uploadID, _, _ string) error {
		enqueuedUploadID = uploadID
		return nil
	})

	out, _ := svc.RequestUpload(t.Context(), RequestUploadInput{
		UserID:      "user-1",
		Category:    "chat-attachment",
		Filename:    "photo.jpg",
		ContentType: "image/jpeg",
		SizeBytes:   1024,
	})

	_, err := svc.ConfirmUpload(t.Context(), out.UploadID, "user-1")
	if err != nil {
		t.Fatalf("ConfirmUpload() error: %v", err)
	}
	if enqueuedUploadID != out.UploadID {
		t.Errorf("enqueued upload ID = %q, want %q", enqueuedUploadID, out.UploadID)
	}
}

func TestConfirmUpload_EnqueuerError_DoesNotFail(t *testing.T) {
	store := newMockStore()
	svc := NewService(store, &mockStorage{}, zerolog.Nop())
	svc.SetEnqueuer(func(_ context.Context, _, _, _ string) error {
		return errors.New("redis down")
	})

	out, _ := svc.RequestUpload(t.Context(), RequestUploadInput{
		UserID:      "user-1",
		Category:    "chat-attachment",
		Filename:    "photo.jpg",
		ContentType: "image/jpeg",
		SizeBytes:   1024,
	})

	_, err := svc.ConfirmUpload(t.Context(), out.UploadID, "user-1")
	if err != nil {
		t.Errorf("ConfirmUpload() should succeed even if enqueuer fails: %v", err)
	}
}

func TestRequestUpload_StorageError(t *testing.T) {
	svc := NewService(newMockStore(), &failingMockStorage{}, zerolog.Nop())
	handler := NewHandler(svc)

	body := `{"category":"avatar","filename":"photo.jpg","content_type":"image/jpeg","size_bytes":1024}`
	req := httptest.NewRequest(http.MethodPost, "/request", bytes.NewBufferString(body))
	req = withAuth(req, "user-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestConfirmUpload_CommitError(t *testing.T) {
	store := &failingCommitStore{mockStore: *newMockStore()}
	svc := NewService(&store.mockStore, &mockStorage{}, zerolog.Nop())

	out, _ := svc.RequestUpload(t.Context(), RequestUploadInput{
		UserID:      "user-1",
		Category:    "avatar",
		Filename:    "photo.jpg",
		ContentType: "image/jpeg",
		SizeBytes:   1024,
	})

	// Use failingCommitStore directly as the store.
	svcFail := NewService(store, &mockStorage{}, zerolog.Nop())

	r := chi.NewRouter()
	r.Post("/{id}/confirm", confirmHandler(svcFail))

	req := httptest.NewRequest(http.MethodPost, "/"+out.UploadID+"/confirm", nil)
	req = withAuth(req, "user-1")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestConfirmUpload_MissingID(t *testing.T) {
	// Call confirmHandler directly without chi URL params so id is empty.
	svc := NewService(newMockStore(), &mockStorage{}, zerolog.Nop())
	req := httptest.NewRequest(http.MethodPost, "/confirm", nil)
	req = withAuth(req, "user-1")
	rec := httptest.NewRecorder()
	confirmHandler(svc)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRequestUpload_InvalidFilename(t *testing.T) {
	svc := NewService(newMockStore(), &mockStorage{}, zerolog.Nop())
	handler := NewHandler(svc)

	body := `{"category":"avatar","filename":"","content_type":"image/jpeg","size_bytes":1024}`
	req := httptest.NewRequest(http.MethodPost, "/request", bytes.NewBufferString(body))
	req = withAuth(req, "user-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestConfirmUpload_NonImageDoesNotEnqueue(t *testing.T) {
	store := newMockStore()
	svc := NewService(store, &mockStorage{}, zerolog.Nop())

	enqueued := false
	svc.SetEnqueuer(func(_ context.Context, _, _, _ string) error {
		enqueued = true
		return nil
	})

	out, _ := svc.RequestUpload(t.Context(), RequestUploadInput{
		UserID:      "user-1",
		Category:    "chat-attachment",
		Filename:    "clip.mp4",
		ContentType: "video/mp4",
		SizeBytes:   1024,
	})

	svc.ConfirmUpload(t.Context(), out.UploadID, "user-1") //nolint:errcheck
	if enqueued {
		t.Error("expected enqueuer NOT to be called for non-image uploads")
	}
}

