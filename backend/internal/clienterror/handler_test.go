package clienterror_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/clienterror"
)

func TestHandler_AcceptsReportReturns204(t *testing.T) {
	h := clienterror.NewHandler()
	body := `{"message":"TypeError: x is undefined","stack":"at foo","url":"https://app/x","kind":"error"}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/client-errors", strings.NewReader(body)))

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

func TestHandler_RejectsEmptyMessage(t *testing.T) {
	h := clienterror.NewHandler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/client-errors", strings.NewReader(`{"stack":"x"}`)))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for missing message", rec.Code)
	}
}

func TestHandler_RejectsInvalidJSON(t *testing.T) {
	h := clienterror.NewHandler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/client-errors", strings.NewReader("not json")))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for invalid JSON", rec.Code)
	}
}

func TestHandler_StripsQueryAndFragmentFromLoggedURL(t *testing.T) {
	var buf bytes.Buffer
	log := zerolog.New(&buf)
	h := clienterror.NewHandler()

	body := `{"message":"x","url":"https://app/reset?token=secret123#frag"}`
	req := httptest.NewRequest(http.MethodPost, "/client-errors", strings.NewReader(body))
	req = req.WithContext(log.WithContext(req.Context()))
	h.ServeHTTP(httptest.NewRecorder(), req)

	logged := buf.String()
	if strings.Contains(logged, "secret123") || strings.Contains(logged, "frag") {
		t.Errorf("query/fragment leaked into log line: %s", logged)
	}
	if !strings.Contains(logged, "https://app/reset") {
		t.Errorf("expected sanitized path in log, got: %s", logged)
	}
}
