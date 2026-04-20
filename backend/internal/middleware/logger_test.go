package middleware_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/middleware"
)

func TestRequestLogger_SetsRequestID(t *testing.T) {
	var buf bytes.Buffer
	log := zerolog.New(&buf)

	handler := middleware.RequestLogger(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	rid := rec.Header().Get("X-Request-ID")
	if rid == "" {
		t.Error("expected X-Request-ID header to be set")
	}
}

func TestRequestLogger_LogsAccessEntry(t *testing.T) {
	var buf bytes.Buffer
	log := zerolog.New(&buf)

	handler := middleware.RequestLogger(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/things", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log output is not valid JSON: %v — output: %s", err, buf.String())
	}

	if entry["method"] != "POST" {
		t.Errorf("method = %v, want POST", entry["method"])
	}
	if entry["path"] != "/things" {
		t.Errorf("path = %v, want /things", entry["path"])
	}
	if int(entry["status"].(float64)) != http.StatusCreated {
		t.Errorf("status = %v, want %d", entry["status"], http.StatusCreated)
	}
	if entry["request_id"] == "" {
		t.Error("expected non-empty request_id in log entry")
	}
	if _, ok := entry["latency_ms"]; !ok {
		t.Error("expected latency_ms in log entry")
	}
	if _, ok := entry["bytes"]; !ok {
		t.Error("expected bytes in log entry")
	}
}

func TestRequestLogger_AttachesLoggerToContext(t *testing.T) {
	var buf bytes.Buffer
	log := zerolog.New(&buf)

	var capturedRequestID string
	handler := middleware.RequestLogger(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		zerolog.Ctx(r.Context()).Info().Msg("inner log")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Two log lines should be produced: inner log + access log.
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("expected 2 log lines, got %d: %s", len(lines), buf.String())
	}

	// Inner log must contain request_id.
	var inner map[string]any
	if err := json.Unmarshal(lines[0], &inner); err != nil {
		t.Fatalf("inner log is not valid JSON: %v", err)
	}
	capturedRequestID, _ = inner["request_id"].(string)
	if capturedRequestID == "" {
		t.Error("inner log must contain request_id")
	}

	// Access log must contain the same request_id.
	var access map[string]any
	if err := json.Unmarshal(lines[1], &access); err != nil {
		t.Fatalf("access log is not valid JSON: %v", err)
	}
	if access["request_id"] != capturedRequestID {
		t.Errorf("access log request_id = %v, want %v", access["request_id"], capturedRequestID)
	}
}

func TestRequestLogger_EnrichRequestLog_AppearsInAccessLog(t *testing.T) {
	var buf bytes.Buffer
	log := zerolog.New(&buf)

	handler := middleware.RequestLogger(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		middleware.EnrichRequestLog(r.Context(), "user_id", "user-42")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log output is not valid JSON: %v", err)
	}
	if entry["user_id"] != "user-42" {
		t.Errorf("user_id = %v, want user-42", entry["user_id"])
	}
}

func TestRequestLogger_DefaultStatus200(t *testing.T) {
	var buf bytes.Buffer
	log := zerolog.New(&buf)

	handler := middleware.RequestLogger(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok")) //nolint:errcheck
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log output is not valid JSON: %v", err)
	}
	if int(entry["status"].(float64)) != http.StatusOK {
		t.Errorf("status = %v, want 200", entry["status"])
	}
}

// hijackableRecorder is an httptest.Recorder that also implements http.Hijacker,
// needed to verify the middleware forwards the interface for WebSocket upgrades.
type hijackableRecorder struct {
	*httptest.ResponseRecorder
	hijacked bool
}

func (h *hijackableRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.hijacked = true
	return nil, nil, nil
}

func TestRequestLogger_PreservesHijacker(t *testing.T) {
	var buf bytes.Buffer
	log := zerolog.New(&buf)

	hr := &hijackableRecorder{ResponseRecorder: httptest.NewRecorder()}

	var gotHijacker http.Hijacker
	handler := middleware.RequestLogger(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, ok := w.(http.Hijacker)
		if !ok {
			t.Error("ResponseWriter does not implement http.Hijacker")
			return
		}
		gotHijacker = h
		w.WriteHeader(http.StatusSwitchingProtocols)
	}))

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	handler.ServeHTTP(hr, req)

	if gotHijacker == nil {
		t.Fatal("handler did not receive http.Hijacker")
	}
	gotHijacker.Hijack() //nolint:errcheck
	if !hr.hijacked {
		t.Error("Hijack() was not forwarded to the underlying ResponseWriter")
	}
}

// flushableRecorder is an httptest.Recorder that also implements http.Flusher,
// needed to verify the middleware forwards the interface for SSE streams.
type flushableRecorder struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (f *flushableRecorder) Flush() {
	f.flushed = true
	f.ResponseRecorder.Flush()
}

func TestRequestLogger_PreservesFlusher(t *testing.T) {
	var buf bytes.Buffer
	log := zerolog.New(&buf)

	fr := &flushableRecorder{ResponseRecorder: httptest.NewRecorder()}

	handler := middleware.RequestLogger(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, ok := w.(http.Flusher)
		if !ok {
			t.Error("ResponseWriter does not implement http.Flusher")
			return
		}
		f.Flush()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/notifications/stream", nil)
	handler.ServeHTTP(fr, req)

	if !fr.flushed {
		t.Error("Flush() was not forwarded to the underlying ResponseWriter")
	}
}
