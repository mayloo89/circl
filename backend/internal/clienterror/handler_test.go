package clienterror_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
