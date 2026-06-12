package middleware

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/apierror"
)

// Limiter is the counting interface required by the RateLimit middleware.
// *ratelimit.RedisLimiter satisfies it.
type Limiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

// RateLimit enforces a global per-IP request budget across every route it
// wraps, as a backstop behind the stricter per-endpoint limiters (login,
// register, contact requests, reports). Limiter errors fail open: an
// unreachable Redis must not take the API down with it.
func RateLimit(limiter Limiter, limit int, window time.Duration) func(http.Handler) http.Handler {
	retryAfter := strconv.Itoa(int(window.Seconds()))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			allowed, err := limiter.Allow(r.Context(), "global:ip:"+ClientIP(r), limit, window)
			if err != nil {
				zerolog.Ctx(r.Context()).Warn().Err(err).Msg("global rate limiter error")
			} else if !allowed {
				w.Header().Set("Retry-After", retryAfter)
				apierror.Write(w, http.StatusTooManyRequests, apierror.CodeRateLimited, "too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
