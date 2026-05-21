package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
)

type clientIPKey struct{}

// ParseCIDRs parses a comma-separated list of CIDR strings.
// Invalid entries are silently skipped.
func ParseCIDRs(list string) []*net.IPNet {
	var out []*net.IPNet
	for cidr := range strings.SplitSeq(list, ",") {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		if _, network, err := net.ParseCIDR(cidr); err == nil {
			out = append(out, network)
		}
	}
	return out
}

// RealIP resolves the true client IP from X-Forwarded-For or X-Real-IP only
// when the direct connection arrives from a trusted proxy CIDR. The result is
// stored in the request context for retrieval via ClientIP.
//
// When cidrs is empty every request's IP is taken from r.RemoteAddr regardless
// of any X-Forwarded-For header — the safe default when not behind a proxy.
func RealIP(cidrs []*net.IPNet) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := resolveClientIP(r, cidrs)
			r = r.WithContext(context.WithValue(r.Context(), clientIPKey{}, ip))
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIP returns the real client IP. The RealIP middleware must run first; if
// it has not, the fallback is r.RemoteAddr with the port stripped.
func ClientIP(r *http.Request) string {
	if ip, ok := r.Context().Value(clientIPKey{}).(string); ok && ip != "" {
		return ip
	}
	return hostFromRemoteAddr(r.RemoteAddr)
}

func resolveClientIP(r *http.Request, cidrs []*net.IPNet) string {
	host := hostFromRemoteAddr(r.RemoteAddr)
	if len(cidrs) == 0 {
		return host
	}
	remoteIP := net.ParseIP(host)
	if remoteIP == nil || !inCIDRs(remoteIP, cidrs) {
		return host
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		before, _, ok := strings.Cut(xff, ",")
		if ok {
			return strings.TrimSpace(before)
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	return host
}

func inCIDRs(ip net.IP, cidrs []*net.IPNet) bool {
	for _, c := range cidrs {
		if c.Contains(ip) {
			return true
		}
	}
	return false
}

func hostFromRemoteAddr(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}
