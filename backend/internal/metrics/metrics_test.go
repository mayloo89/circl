package metrics_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mayloo89/circl/backend/internal/metrics"
)

func TestHandler_ServesMetrics(t *testing.T) {
	m := metrics.New(nil)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	m.Handler("").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	// Go runtime metrics are always present.
	if !strings.Contains(string(body), "go_goroutines") {
		t.Error("expected go_goroutines in metrics output")
	}
}

func TestHandler_TokenRequired(t *testing.T) {
	m := metrics.New(nil)
	h := m.Handler("secret")

	t.Run("forbidden without token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", rec.Code)
		}
	})

	t.Run("forbidden with wrong token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", rec.Code)
		}
	})

	t.Run("ok with correct token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.Header.Set("Authorization", "Bearer secret")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
	})
}

func TestMiddleware_RecordsHTTPMetrics(t *testing.T) {
	m := metrics.New(nil)

	handler := m.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/things", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the counter was incremented by scraping /metrics.
	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRec := httptest.NewRecorder()
	m.Handler("").ServeHTTP(metricsRec, metricsReq)

	body, _ := io.ReadAll(metricsRec.Body)
	if !strings.Contains(string(body), "circl_http_requests_total") {
		t.Error("expected circl_http_requests_total in metrics output after a request")
	}
}

func TestWSHub_ActiveConns(t *testing.T) {
	m := metrics.New(nil)

	hub := &stubWSHub{n: 3}
	m.RegisterWSHub(hub)

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	m.Handler("").ServeHTTP(rec, metricsReq)

	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "circl_websocket_active_connections 3") {
		t.Errorf("expected circl_websocket_active_connections 3 in output, got:\n%s", body)
	}
}

type stubWSHub struct{ n int64 }

func (s *stubWSHub) ActiveConns() int64 { return s.n }
