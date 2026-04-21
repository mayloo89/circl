package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/xid"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"
)

type requestFieldsKey struct{}

// requestFields accumulates key-value pairs that downstream middleware and
// handlers can contribute to the access-log entry for this request.
// Storing a pointer in context lets all layers mutate the same instance.
type requestFields struct {
	mu     sync.Mutex
	fields map[string]string
}

func (f *requestFields) set(key, value string) {
	f.mu.Lock()
	f.fields[key] = value
	f.mu.Unlock()
}

func (f *requestFields) apply(e *zerolog.Event) *zerolog.Event {
	f.mu.Lock()
	defer f.mu.Unlock()
	for k, v := range f.fields {
		e = e.Str(k, v)
	}
	return e
}

// EnrichRequestLog adds key=value to the access-log entry written by
// RequestLogger for the current request. Safe to call from any middleware or
// handler in the chain. No-op when called outside a RequestLogger context.
func EnrichRequestLog(ctx context.Context, key, value string) {
	if f, ok := ctx.Value(requestFieldsKey{}).(*requestFields); ok {
		f.set(key, value)
	}
}

// RequestLogger returns a middleware that:
//   - assigns a unique request_id to every request (xid)
//   - attaches a request-scoped logger to the context (read via zerolog.Ctx)
//   - propagates X-Request-ID as a response header
//   - writes one structured access-log entry per request: method, path, status,
//     bytes, latency_ms, request_id, plus any extra fields added via EnrichRequestLog
//
// Uses chi's WrapResponseWriter to forward http.Hijacker and http.Flusher so
// that WebSocket upgrades and SSE streams work correctly behind this middleware.
func RequestLogger(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rid := xid.New().String()

			fields := &requestFields{fields: make(map[string]string)}
			ctx := context.WithValue(r.Context(), requestFieldsKey{}, fields)

			reqLog := log.With().Str("request_id", rid).Logger()
			// If the tracing middleware (which runs before this one) has placed
			// a span in the context, inject trace_id and span_id into every log
			// line for this request — enabling log-trace correlation in Grafana.
			if span := trace.SpanFromContext(r.Context()); span.SpanContext().IsValid() {
				sc := span.SpanContext()
				reqLog = reqLog.With().
					Str("trace_id", sc.TraceID().String()).
					Str("span_id", sc.SpanID().String()).
					Logger()
			}
			ctx = reqLog.WithContext(ctx)
			r = r.WithContext(ctx)

			w.Header().Set("X-Request-ID", rid)

			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			e := reqLog.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", ww.Status()).
				Int("bytes", ww.BytesWritten()).
				Int64("latency_ms", time.Since(start).Milliseconds())
			e = fields.apply(e)
			e.Msg("request")
		})
	}
}
