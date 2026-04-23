package middleware_test

import (
	"net/http"
	"net/http/httptest"
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
			}
			for k, v := range want {
				if got := rec.Header().Get(k); got != v {
					t.Errorf("%s: %s = %q, want %q", env, k, got, v)
				}
			}
		})
	}
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
