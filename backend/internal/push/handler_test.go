package push_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/push"
)

// stubStore is a no-op Store for handler tests.
type stubStore struct {
	saveErr   error
	deleteErr error
}

func (s *stubStore) Save(_ context.Context, _ push.Subscription) error          { return s.saveErr }
func (s *stubStore) Delete(_ context.Context, _, _ string) error                { return s.deleteErr }
func (s *stubStore) ListByUser(_ context.Context, _ string) ([]push.Subscription, error) {
	return nil, nil
}
func (s *stubStore) DeleteByEndpoint(_ context.Context, _ string) error { return nil }

func authedRequest(method, path, body string) *http.Request {
	var bodyReader *strings.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	} else {
		bodyReader = strings.NewReader("")
	}
	r := httptest.NewRequest(method, path, bodyReader)
	ctx := middleware.ContextWithUserID(r.Context(), "user-abc")
	return r.WithContext(ctx)
}

func newHandler(store push.Store) http.Handler {
	svc := push.NewService(store, "pubkey", "privkey", "mailto:test@example.com")
	return push.NewHandler(svc)
}

// --- GET /vapid-public-key ---

func TestVapidPublicKeyHandler_Enabled(t *testing.T) {
	h := newHandler(&stubStore{})
	req := httptest.NewRequest(http.MethodGet, "/vapid-public-key", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "pubkey") {
		t.Errorf("body = %q, want to contain pubkey", rec.Body.String())
	}
}

func TestVapidPublicKeyHandler_Disabled(t *testing.T) {
	svc := push.NewService(&stubStore{}, "", "", "")
	h := push.NewHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/vapid-public-key", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

// --- POST /subscribe ---

func TestSubscribeHandler_Success(t *testing.T) {
	h := newHandler(&stubStore{})
	req := authedRequest(http.MethodPost, "/subscribe",
		`{"endpoint":"https://ep","p256dh":"pk","auth":"au"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestSubscribeHandler_MissingFields(t *testing.T) {
	h := newHandler(&stubStore{})
	for _, body := range []string{
		`{"endpoint":"","p256dh":"pk","auth":"au"}`,
		`{"endpoint":"https://ep","p256dh":"","auth":"au"}`,
		`{"endpoint":"https://ep","p256dh":"pk","auth":""}`,
	} {
		req := authedRequest(http.MethodPost, "/subscribe", body)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %s: status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestSubscribeHandler_MalformedJSON(t *testing.T) {
	h := newHandler(&stubStore{})
	req := authedRequest(http.MethodPost, "/subscribe", "{not json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestSubscribeHandler_Unauthorized(t *testing.T) {
	h := newHandler(&stubStore{})
	req := httptest.NewRequest(http.MethodPost, "/subscribe",
		strings.NewReader(`{"endpoint":"https://ep","p256dh":"pk","auth":"au"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestSubscribeHandler_StoreError(t *testing.T) {
	h := newHandler(&stubStore{saveErr: context.DeadlineExceeded})
	req := authedRequest(http.MethodPost, "/subscribe",
		`{"endpoint":"https://ep","p256dh":"pk","auth":"au"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- DELETE /subscribe ---

func TestUnsubscribeHandler_Success(t *testing.T) {
	h := newHandler(&stubStore{})
	req := authedRequest(http.MethodDelete, "/subscribe", `{"endpoint":"https://ep"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestUnsubscribeHandler_MissingEndpoint(t *testing.T) {
	h := newHandler(&stubStore{})
	req := authedRequest(http.MethodDelete, "/subscribe", `{"endpoint":""}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUnsubscribeHandler_Unauthorized(t *testing.T) {
	h := newHandler(&stubStore{})
	req := httptest.NewRequest(http.MethodDelete, "/subscribe",
		strings.NewReader(`{"endpoint":"https://ep"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestUnsubscribeHandler_StoreError(t *testing.T) {
	h := newHandler(&stubStore{deleteErr: context.DeadlineExceeded})
	req := authedRequest(http.MethodDelete, "/subscribe", `{"endpoint":"https://ep"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

