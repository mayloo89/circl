package notifications_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mayloo89/circl/backend/internal/notifications"
	"github.com/mayloo89/circl/backend/internal/token"
)

const testSecret = "supersecretfortesting-mustbe32chars!!"

// noFlusherWriter wraps an http.ResponseWriter without exposing http.Flusher,
// so the handler's Flusher check can be exercised.
type noFlusherWriter struct {
	rec *httptest.ResponseRecorder
}

func (n *noFlusherWriter) Header() http.Header          { return n.rec.Header() }
func (n *noFlusherWriter) Write(b []byte) (int, error)  { return n.rec.Write(b) }
func (n *noFlusherWriter) WriteHeader(code int)         { n.rec.WriteHeader(code) }

func TestSSEHandler_Headers(t *testing.T) {
	hub := notifications.NewHub()
	handler := notifications.NewHandler(hub, testSecret)
	tok, _ := token.Generate("user-1", testSecret, time.Hour)

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/notifications/stream?token="+tok, nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}
}

func TestSSEHandler_ConnectedEvent(t *testing.T) {
	hub := notifications.NewHub()
	handler := notifications.NewHandler(hub, testSecret)
	tok, _ := token.Generate("user-1", testSecret, time.Hour)

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/notifications/stream?token="+tok, nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `"type":"connected"`) {
		t.Errorf("expected connected event, got: %s", rec.Body.String())
	}
}

func TestSSEHandler_ReceivesNotification(t *testing.T) {
	hub := notifications.NewHub()
	handler := notifications.NewHandler(hub, testSecret)
	tok, _ := token.Generate("user-1", testSecret, time.Hour)

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/notifications/stream?token="+tok, nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	go func() {
		// Give the handler time to subscribe before notifying.
		time.Sleep(20 * time.Millisecond)
		hub.Notify("user-1", notifications.Event{Type: "contact_request"})
	}()

	handler.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `"type":"contact_request"`) {
		t.Errorf("expected contact_request event, got: %s", rec.Body.String())
	}
}

func TestSSEHandler_Unauthorized_MissingToken(t *testing.T) {
	hub := notifications.NewHub()
	handler := notifications.NewHandler(hub, testSecret)

	req := httptest.NewRequest(http.MethodGet, "/notifications/stream", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestSSEHandler_Unauthorized_InvalidToken(t *testing.T) {
	hub := notifications.NewHub()
	handler := notifications.NewHandler(hub, testSecret)

	req := httptest.NewRequest(http.MethodGet, "/notifications/stream?token=bad.token.here", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestSSEHandler_ClientDisconnect(t *testing.T) {
	hub := notifications.NewHub()
	handler := notifications.NewHandler(hub, testSecret)
	tok, _ := token.Generate("user-1", testSecret, time.Hour)

	ctx, cancel := context.WithCancel(t.Context())
	req := httptest.NewRequest(http.MethodGet, "/notifications/stream?token="+tok, nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(rec, req)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// handler returned cleanly after disconnect
	case <-time.After(time.Second):
		t.Fatal("handler did not return after client disconnect")
	}
}

func TestSSEHandler_NoFlusher(t *testing.T) {
	hub := notifications.NewHub()
	handler := notifications.NewHandler(hub, testSecret)
	tok, _ := token.Generate("user-1", testSecret, time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/notifications/stream?token="+tok, nil)
	rec := httptest.NewRecorder()
	nf := &noFlusherWriter{rec: rec}
	handler.ServeHTTP(nf, req)

	if rec.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotImplemented)
	}
}
