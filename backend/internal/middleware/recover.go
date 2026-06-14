package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/apierror"
)

// Recoverer catches panics in the HTTP handler chain, logs them as a
// structured error (event=panic) with the request-scoped logger so the
// request_id/trace_id are attached, and returns 500 instead of dropping the
// connection. onPanic, if set, is notified for alerting (panic counter).
//
// Place this just after RequestLogger so the panic log carries request context.
func Recoverer(onPanic func(source string)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				// http.ErrAbortHandler is a sentinel the server uses to abort a
				// response; let the stdlib handle it rather than swallowing it.
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				zerolog.Ctx(r.Context()).Error().
					Str("event", "panic").
					Str("source", "http").
					Interface("panic", rec).
					Bytes("stack", debug.Stack()).
					Msg("recovered panic in handler")
				if onPanic != nil {
					onPanic("http")
				}
				apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			}()
			next.ServeHTTP(w, r)
		})
	}
}
