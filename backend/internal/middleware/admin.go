package middleware

import "net/http"

// RequireAdmin rejects requests from non-admin users with 403.
// Must be chained after RequireAuth.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsAdminFromContext(r.Context()) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":"forbidden"}`)) //nolint:errcheck
			return
		}
		next.ServeHTTP(w, r)
	})
}
