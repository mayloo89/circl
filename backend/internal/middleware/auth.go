package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/mayloo89/circl/backend/internal/token"
)

type contextKey string

const (
	userIDKey contextKey = "userID"
	roleKey   contextKey = "role"
)

// UserStatusChecker allows the auth middleware to verify that the authenticated
// user is still active in the database (e.g. not suspended or banned).
type UserStatusChecker interface {
	IsActiveUser(ctx context.Context, userID string) (bool, error)
}

// RequireAuth validates the Bearer JWT in the Authorization header.
// On success it injects the user ID and role into the request context.
// On failure it responds with 401 and stops the chain.
// An optional UserStatusChecker can be passed; when provided, it is called after
// JWT validation and the request is rejected with 401 if the user is not active.
func RequireAuth(jwtSecret string, checker ...UserStatusChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeUnauthorized(w)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				writeUnauthorized(w)
				return
			}

			claims, err := token.Validate(parts[1], jwtSecret)
			if err != nil {
				writeUnauthorized(w)
				return
			}

			if len(checker) > 0 && checker[0] != nil {
				active, err := checker[0].IsActiveUser(r.Context(), claims.Subject)
				if err != nil || !active {
					writeUnauthorized(w)
					return
				}
			}

			ctx := context.WithValue(r.Context(), userIDKey, claims.Subject)
			ctx = context.WithValue(ctx, roleKey, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext retrieves the authenticated user ID from the context.
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey).(string)
	return id, ok
}

// RoleFromContext retrieves the role string from the context.
// Returns empty string when not set.
func RoleFromContext(ctx context.Context) string {
	r, _ := ctx.Value(roleKey).(string)
	return r
}

// IsAdminFromContext returns true when the role is admin or super_admin.
func IsAdminFromContext(ctx context.Context) bool {
	r := RoleFromContext(ctx)
	return r == token.RoleAdmin || r == token.RoleSuperAdmin
}

// IsSuperAdminFromContext returns true only when the role is super_admin.
func IsSuperAdminFromContext(ctx context.Context) bool {
	return RoleFromContext(ctx) == token.RoleSuperAdmin
}

// ContextWithUserID returns a new context with the given user ID.
// Intended for testing — production code uses RequireAuth middleware.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"unauthorized"}`)) //nolint:errcheck
}
