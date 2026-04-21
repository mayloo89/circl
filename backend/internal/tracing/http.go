package tracing

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware returns a chi-compatible OpenTelemetry server-span middleware.
//
// Each request begins as "HTTP <METHOD>" and is renamed after the handler
// returns to "HTTP <METHOD> <route-pattern>" (e.g. "HTTP GET /profiles/{id}"),
// ensuring low-cardinality span names. W3C TraceContext headers are extracted
// from incoming requests and injected into outgoing responses for distributed
// trace propagation.
func HTTPMiddleware(service string) func(http.Handler) http.Handler {
	tracer := otel.Tracer(service)
	prop := otel.GetTextMapPropagator()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := prop.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
			ctx, span := tracer.Start(ctx, "HTTP "+r.Method,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPRequestMethodKey.String(r.Method),
					attribute.String("url.path", r.URL.Path),
					attribute.String("server.address", r.Host),
				),
			)
			defer span.End()

			prop.Inject(ctx, propagation.HeaderCarrier(w.Header()))
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)

			// After the handler returns chi has resolved the route pattern.
			route := r.URL.Path
			if rctx := chi.RouteContext(r.Context()); rctx != nil && rctx.RoutePattern() != "" {
				route = rctx.RoutePattern()
			}
			span.SetName("HTTP " + r.Method + " " + route)
			span.SetAttributes(semconv.HTTPRouteKey.String(route))
		})
	}
}
