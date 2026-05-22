package middleware

import "net/http"

// maxJSONBodyBytes is the upper bound for JSON API request bodies (1 MiB).
// File uploads go through a separate handler that applies per-category limits.
const maxJSONBodyBytes = 1 << 20

// SecurityHeaders adds security-related HTTP response headers to every response.
// In production (env == "production") it also sets HSTS with a one-year max-age.
func SecurityHeaders(env string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Cache-Control", "no-store")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			if env == "production" {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// LimitRequestBody caps JSON API request bodies at maxJSONBodyBytes to prevent
// memory exhaustion from oversized payloads. Apply globally on API routes only;
// file-upload paths must not use this middleware — they apply their own limits
// via http.MaxBytesReader with per-category sizes.
func LimitRequestBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
		next.ServeHTTP(w, r)
	})
}
