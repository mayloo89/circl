package notifications_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mayloo89/circl/backend/internal/notifications"
	"github.com/mayloo89/circl/backend/internal/wsticket"
)

// mockRedeemer is a test double for notifications.TicketRedeemer.
type mockRedeemer struct {
	tickets map[string]string // ticket → userID
}

func (m *mockRedeemer) Redeem(_ context.Context, ticket string) (wsticket.TicketData, error) {
	if uid, ok := m.tickets[ticket]; ok {
		delete(m.tickets, ticket)
		return wsticket.TicketData{UserID: uid}, nil
	}
	return wsticket.TicketData{}, errors.New("invalid ticket")
}

// noFlusherWriter wraps an http.ResponseWriter without exposing http.Flusher,
// so the handler's Flusher check can be exercised.
type noFlusherWriter struct {
	rec *httptest.ResponseRecorder
}

func (n *noFlusherWriter) Header() http.Header         { return n.rec.Header() }
func (n *noFlusherWriter) Write(b []byte) (int, error) { return n.rec.Write(b) }
func (n *noFlusherWriter) WriteHeader(code int)        { n.rec.WriteHeader(code) }

func TestSSEHandler_Headers(t *testing.T) {
	hub := notifications.NewHub()
	redeemer := &mockRedeemer{tickets: map[string]string{"tk-user1": "user-1"}}
	handler := notifications.NewHandler(hub, redeemer)

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/notifications/stream?ticket=tk-user1", nil).WithContext(ctx)
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
	redeemer := &mockRedeemer{tickets: map[string]string{"tk-user1": "user-1"}}
	handler := notifications.NewHandler(hub, redeemer)

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/notifications/stream?ticket=tk-user1", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `"type":"connected"`) {
		t.Errorf("expected connected event, got: %s", rec.Body.String())
	}
}

func TestSSEHandler_ReceivesNotification(t *testing.T) {
	hub := notifications.NewHub()
	redeemer := &mockRedeemer{tickets: map[string]string{"tk-user1": "user-1"}}
	handler := notifications.NewHandler(hub, redeemer)

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/notifications/stream?ticket=tk-user1", nil).WithContext(ctx)
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

func TestSSEHandler_Unauthorized_MissingTicket(t *testing.T) {
	hub := notifications.NewHub()
	redeemer := &mockRedeemer{tickets: map[string]string{}}
	handler := notifications.NewHandler(hub, redeemer)

	req := httptest.NewRequest(http.MethodGet, "/notifications/stream", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestSSEHandler_Unauthorized_InvalidTicket(t *testing.T) {
	hub := notifications.NewHub()
	redeemer := &mockRedeemer{tickets: map[string]string{}}
	handler := notifications.NewHandler(hub, redeemer)

	req := httptest.NewRequest(http.MethodGet, "/notifications/stream?ticket=bad-ticket-here", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestSSEHandler_ClientDisconnect(t *testing.T) {
	hub := notifications.NewHub()
	redeemer := &mockRedeemer{tickets: map[string]string{"tk-user1": "user-1"}}
	handler := notifications.NewHandler(hub, redeemer)

	ctx, cancel := context.WithCancel(t.Context())
	req := httptest.NewRequest(http.MethodGet, "/notifications/stream?ticket=tk-user1", nil).WithContext(ctx)
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
	redeemer := &mockRedeemer{tickets: map[string]string{"tk-user1": "user-1"}}
	handler := notifications.NewHandler(hub, redeemer)

	req := httptest.NewRequest(http.MethodGet, "/notifications/stream?ticket=tk-user1", nil)
	rec := httptest.NewRecorder()
	nf := &noFlusherWriter{rec: rec}
	handler.ServeHTTP(nf, req)

	if rec.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotImplemented)
	}
}
