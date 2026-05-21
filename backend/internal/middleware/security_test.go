package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mayloo89/circl/backend/internal/middleware"
)

func TestSecurityHeaders_Common(t *testing.T) {
	for _, env := range []string{"development", "production"} {
		t.Run(env, func(t *testing.T) {
			handler := middleware.SecurityHeaders(env)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

			want := map[string]string{
				"X-Content-Type-Options": "nosniff",
				"X-Frame-Options":        "DENY",
				"Referrer-Policy":        "strict-origin-when-cross-origin",
				"Cache-Control":          "no-store",
				"Permissions-Policy":     "camera=(), microphone=(), geolocation=()",
			}
			for k, v := range want {
				if got := rec.Header().Get(k); got != v {
					t.Errorf("%s: %s = %q, want %q", env, k, got, v)
				}
			}
		})
	}
}

func TestLimitRequestBody(t *testing.T) {
	// Handler that reports 400 when the body read fails (mirrors real JSON handlers).
	handler := middleware.LimitRequestBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			http.Error(w, "body too large", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("allows body within limit", func(t *testing.T) {
		body := strings.NewReader(strings.Repeat("x", 512))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", body)
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		body := strings.NewReader(strings.Repeat("x", 1<<20+1))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", body)
		handler.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			t.Error("expected non-200 for oversized body, got 200")
		}
	})
}

func TestSecurityHeaders_HSTSProductionOnly(t *testing.T) {
	t.Run("production sets HSTS", func(t *testing.T) {
		handler := middleware.SecurityHeaders("production")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if got := rec.Header().Get("Strict-Transport-Security"); got == "" {
			t.Error("production: expected HSTS header, got none")
		}
	})

	t.Run("development omits HSTS", func(t *testing.T) {
		handler := middleware.SecurityHeaders("development")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
			t.Errorf("development: unexpected HSTS header %q", got)
		}
	})
}
