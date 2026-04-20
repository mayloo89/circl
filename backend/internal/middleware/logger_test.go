package middleware_test

import (
	"bytes"
	"encoding/json"
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
		rid := w.Header().Get("X-Request-ID")
		if rid == "" {
			// Header not set yet; read from response after ServeHTTP returns.
		}
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
}

func TestRequestLogger_AttachesLoggerToContext(t *testing.T) {
	var buf bytes.Buffer
	log := zerolog.New(&buf)

	var capturedRequestID string
	handler := middleware.RequestLogger(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log something from within the handler using the context logger.
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

	// Handler writes body without calling WriteHeader explicitly.
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
