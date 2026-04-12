package middleware

import "net/http"

// RequireAdmin rejects requests from users who are not admin or super_admin with 403.
// Must be chained after RequireAuth.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsAdminFromContext(r.Context()) {
			writeForbidden(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireSuperAdmin rejects requests from users who are not super_admin with 403.
// Must be chained after RequireAuth.
func RequireSuperAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsSuperAdminFromContext(r.Context()) {
			writeForbidden(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(`{"error":"forbidden"}`)) //nolint:errcheck
}
