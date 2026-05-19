package albums_test

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

	"github.com/mayloo89/circl/backend/internal/albums"
	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/storage"
	"github.com/mayloo89/circl/backend/internal/token"
	"github.com/rs/zerolog"
)

const (
	htSecret = "supersecretfortesting-mustbe32chars!!"
	htUserID = "user-handler-test"
)

type mockLimiter struct {
	allowed bool
	err     error
}

func (m *mockLimiter) Allow(_ context.Context, _ string, _ int, _ time.Duration) (bool, error) {
	return m.allowed, m.err
}

func htAuthedRequest(r *http.Request) *http.Request {
	tok, _ := token.Generate(htUserID, token.RoleUser, htSecret, time.Hour)
	r.Header.Set("Authorization", "Bearer "+tok)
	return r
}

func htServe(h http.Handler, r *http.Request, rec *httptest.ResponseRecorder) {
	middleware.RequireAuth(htSecret)(h).ServeHTTP(rec, r)
}

// noopStorage satisfies storage.Storage enough for handler tests that don't
// reach the storage layer.
type noopStorage struct{ storage.Storage }

func (noopStorage) GetObject(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, errors.New("noop")
}

func TestInviteGrant_RateLimited(t *testing.T) {
	store := newFakeStore()
	svc := albums.NewService(store, nil, fakeMedia{owner: htUserID, category: "album-private"}, zerolog.Nop())
	a, _ := svc.CreateAlbum(t.Context(), htUserID, "test", "")

	limiter := &mockLimiter{allowed: false}
	h := albums.NewHandler(svc, noopStorage{}, albums.WithLimiter(limiter))

	// Mount under /{id}/grants/invite to satisfy chi URL params.
	r := chi.NewRouter()
	r.Mount("/albums", h)

	body := `{"grantee_id":"other-user"}`
	req := htAuthedRequest(httptest.NewRequest(http.MethodPost, "/albums/"+a.ID+"/grants/invite", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	htServe(r, req, rec)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d (429)", rec.Code, http.StatusTooManyRequests)
	}
}

func TestInviteGrant_LimiterError_Continues(t *testing.T) {
	store := newFakeStore()
	svc := albums.NewService(store, nil, fakeMedia{owner: htUserID, category: "album-private"}, zerolog.Nop())
	a, _ := svc.CreateAlbum(t.Context(), htUserID, "test", "")

	// When limiter returns an error, the invite should still go through.
	limiter := &mockLimiter{allowed: false, err: errors.New("redis down")}
	h := albums.NewHandler(svc, noopStorage{}, albums.WithLimiter(limiter))

	r := chi.NewRouter()
	r.Mount("/albums", h)

	body := `{"grantee_id":"other-user"}`
	req := htAuthedRequest(httptest.NewRequest(http.MethodPost, "/albums/"+a.ID+"/grants/invite", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	htServe(r, req, rec)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d (201)", rec.Code, http.StatusCreated)
	}
}
