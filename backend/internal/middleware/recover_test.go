package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/middleware"
)

func TestRecoverer_CatchesPanicReturns500(t *testing.T) {
	var recordedSource string
	h := middleware.Recoverer(func(source string) { recordedSource = source })(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic("boom")
		}),
	)
	// Attach a logger so the panic log path doesn't hit the disabled default.
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req = req.WithContext(zerolog.New(nil).WithContext(req.Context()))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if recordedSource != "http" {
		t.Errorf("onPanic source = %q, want %q", recordedSource, "http")
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != "internal_error" {
		t.Errorf("error code = %q, want internal_error", body.Code)
	}
}

func TestRecoverer_PassesThroughWhenNoPanic(t *testing.T) {
	called := false
	h := middleware.Recoverer(nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	if !called {
		t.Error("next handler was not called")
	}
	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want 418 (handler's own status preserved)", rec.Code)
	}
}

func TestRecoverer_RepanicsOnErrAbortHandler(t *testing.T) {
	h := middleware.Recoverer(nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	defer func() {
		if r := recover(); r != http.ErrAbortHandler {
			t.Errorf("recovered = %v, want http.ErrAbortHandler to propagate", r)
		}
	}()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
	t.Fatal("expected ErrAbortHandler to propagate")
}
