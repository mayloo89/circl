package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mayloo89/circl/backend/internal/middleware"
)

func TestRealIP_NoTrustedProxies(t *testing.T) {
	handler := middleware.RealIP(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Got-IP", middleware.ClientIP(r))
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.5")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// X-Forwarded-For must be ignored when no trusted proxies are configured.
	if got := rec.Header().Get("X-Got-IP"); got != "10.0.0.1" {
		t.Errorf("ClientIP = %q, want 10.0.0.1", got)
	}
}

func TestRealIP_TrustedProxy_ReadsXFF(t *testing.T) {
	cidrs := middleware.ParseCIDRs("10.0.0.0/8")
	handler := middleware.RealIP(cidrs)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Got-IP", middleware.ClientIP(r))
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234" // within trusted CIDR
	req.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Got-IP"); got != "203.0.113.5" {
		t.Errorf("ClientIP = %q, want 203.0.113.5", got)
	}
}

func TestRealIP_TrustedProxy_ReadsXRealIP(t *testing.T) {
	cidrs := middleware.ParseCIDRs("10.0.0.0/8")
	handler := middleware.RealIP(cidrs)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Got-IP", middleware.ClientIP(r))
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.2:80"
	req.Header.Set("X-Real-IP", "198.51.100.7")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Got-IP"); got != "198.51.100.7" {
		t.Errorf("ClientIP = %q, want 198.51.100.7", got)
	}
}

func TestRealIP_UntrustedProxy_IgnoresXFF(t *testing.T) {
	cidrs := middleware.ParseCIDRs("10.0.0.0/8")
	handler := middleware.RealIP(cidrs)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Got-IP", middleware.ClientIP(r))
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:9999" // outside trusted CIDR
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Got-IP"); got != "192.168.1.1" {
		t.Errorf("ClientIP = %q, want 192.168.1.1", got)
	}
}

func TestClientIP_NoMiddleware(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "172.16.0.5:5555"
	if got := middleware.ClientIP(req); got != "172.16.0.5" {
		t.Errorf("ClientIP = %q, want 172.16.0.5", got)
	}
}

func TestParseCIDRs_SkipsInvalid(t *testing.T) {
	cidrs := middleware.ParseCIDRs("10.0.0.0/8, not-a-cidr, 192.168.0.0/16")
	if len(cidrs) != 2 {
		t.Errorf("len(cidrs) = %d, want 2", len(cidrs))
	}
}

func TestParseCIDRs_Empty(t *testing.T) {
	if cidrs := middleware.ParseCIDRs(""); len(cidrs) != 0 {
		t.Errorf("expected empty slice, got len=%d", len(cidrs))
	}
}
