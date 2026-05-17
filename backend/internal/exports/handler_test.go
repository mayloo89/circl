package exports_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/exports"
	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/token"
)

const (
	testSecret = "supersecretfortesting-mustbe32chars!!"
	testUserID = "u-1"
)

// authedRequest stamps a Bearer JWT for testUserID onto r.
func authedRequest(r *http.Request) *http.Request {
	tok, _ := token.Generate(testUserID, token.RoleUser, testSecret, time.Hour)
	r.Header.Set("Authorization", "Bearer "+tok)
	return r
}

func serveAuthed(h http.Handler, r *http.Request, rec *httptest.ResponseRecorder) {
	middleware.RequireAuth(testSecret)(h).ServeHTTP(rec, r)
}

// memStorage is the small storage stub the handler tests need.
type memStorage struct {
	objects map[string][]byte
}

func (m *memStorage) GetObject(_ context.Context, key string) (io.ReadCloser, error) {
	b, ok := m.objects[key]
	if !ok {
		return nil, errors.New("no such key")
	}
	return io.NopCloser(strings.NewReader(string(b))), nil
}

func (m *memStorage) PutObject(_ context.Context, _, _ string, _ io.Reader, _ int64) error {
	return nil
}

func newAuthedHandlerFromStore(store *mockStore) http.Handler {
	svc := exports.NewService(exports.Config{
		Store:           store,
		Source:          &mockSource{},
		Storage:         &memStorage{objects: map[string][]byte{}},
		DownloadURLBase: "https://api.example/account/export",
		Log:             zerolog.Nop(),
	})
	return exports.NewAuthedHandler(svc)
}

// --- POST / (request export) ---

func TestRequest_HappyPath(t *testing.T) {
	store := &mockStore{create: &exports.Request{ID: "r-1", Status: exports.StatusPending, RequestedAt: time.Now()}}
	h := newAuthedHandlerFromStore(store)

	req := authedRequest(httptest.NewRequest(http.MethodPost, "/", nil))
	rec := httptest.NewRecorder()
	serveAuthed(h, req, rec)

	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d (body=%q)", rec.Code, http.StatusAccepted, rec.Body.String())
	}
}

func TestRequest_Unauthorized(t *testing.T) {
	h := newAuthedHandlerFromStore(&mockStore{})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	serveAuthed(h, req, rec)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequest_AlreadyPendingReturns409(t *testing.T) {
	h := newAuthedHandlerFromStore(&mockStore{createErr: exports.ErrAlreadyPending})

	req := authedRequest(httptest.NewRequest(http.MethodPost, "/", nil))
	rec := httptest.NewRecorder()
	serveAuthed(h, req, rec)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestRequest_RateLimitedReturns429(t *testing.T) {
	h := newAuthedHandlerFromStore(&mockStore{createErr: exports.ErrRateLimited})

	req := authedRequest(httptest.NewRequest(http.MethodPost, "/", nil))
	rec := httptest.NewRecorder()
	serveAuthed(h, req, rec)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

// --- GET / (status) ---

func TestStatus_NoRequestReturnsNone(t *testing.T) {
	h := newAuthedHandlerFromStore(&mockStore{latestErr: exports.ErrNotFound})

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	rec := httptest.NewRecorder()
	serveAuthed(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), `"none"`) {
		t.Errorf("body should contain status:none, got %q", rec.Body.String())
	}
}

func TestStatus_ReturnsLatest(t *testing.T) {
	h := newAuthedHandlerFromStore(&mockStore{latest: &exports.Request{ID: "r-1", Status: exports.StatusReady, RequestedAt: time.Now()}})

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	rec := httptest.NewRecorder()
	serveAuthed(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), `"ready"`) {
		t.Errorf("body should contain status:ready, got %q", rec.Body.String())
	}
}

// --- GET /{token} (download) ---

func TestDownloadHandler_InvalidToken(t *testing.T) {
	svc := exports.NewService(exports.Config{
		Store:   &mockStore{byHashErr: exports.ErrNotFound},
		Source:  &mockSource{},
		Storage: &memStorage{objects: map[string][]byte{}},
		Log:     zerolog.Nop(),
	})
	h := exports.NewDownloadHandler(svc)

	// Route through chi so {token} is captured.
	router := chi.NewRouter()
	router.Mount("/", h)
	req := httptest.NewRequest(http.MethodGet, "/garbage", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestDownloadHandler_NotReady(t *testing.T) {
	svc := exports.NewService(exports.Config{
		Store:   &mockStore{byHash: &exports.Request{ID: "r-1", Status: exports.StatusPending}},
		Source:  &mockSource{},
		Storage: &memStorage{objects: map[string][]byte{}},
		Log:     zerolog.Nop(),
	})
	h := exports.NewDownloadHandler(svc)

	router := chi.NewRouter()
	router.Mount("/", h)
	req := httptest.NewRequest(http.MethodGet, "/anytoken", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestDownload_StreamsZip(t *testing.T) {
	future := time.Now().Add(time.Hour)
	key := "exports/u-1/r-1.zip"
	stor := &memStorage{objects: map[string][]byte{key: []byte("ZIP-CONTENT")}}
	svc := exports.NewService(exports.Config{
		Store: &mockStore{byHash: &exports.Request{
			ID:         "r-1",
			Status:     exports.StatusReady,
			ExpiresAt:  &future,
			StorageKey: &key,
		}},
		Source:  &mockSource{},
		Storage: stor,
		Log:     zerolog.Nop(),
	})
	h := exports.NewDownloadHandler(svc)

	router := chi.NewRouter()
	router.Mount("/", h)
	req := httptest.NewRequest(http.MethodGet, "/anytoken", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/zip" {
		t.Errorf("Content-Type = %q, want application/zip", got)
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "attachment") {
		t.Errorf("Content-Disposition missing attachment: %q", rec.Header().Get("Content-Disposition"))
	}
	if rec.Body.String() != "ZIP-CONTENT" {
		t.Errorf("body = %q, want ZIP-CONTENT", rec.Body.String())
	}
}
