// Package tracing initialises the global OpenTelemetry TracerProvider and
// propagator. Call Init once at startup; call the returned shutdown function on
// application exit to flush buffered spans.
package tracing

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Init configures the global TracerProvider and W3C TraceContext propagator.
//
// When OTEL_EXPORTER_OTLP_ENDPOINT is unset the SDK uses a no-op exporter so
// the application starts cleanly without a running Tempo/Collector instance.
// Set OTEL_SAMPLE_RATE (0.0–1.0) for head-based sampling; defaults to 1.0.
//
// The returned shutdown function must be called on application exit to flush
// and close the exporter. It is safe to call even if Init returns an error.
func Init(ctx context.Context, log zerolog.Logger, serviceName, serviceVersion, env string) (func(context.Context) error, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
			attribute.String("deployment.environment.name", env),
		),
		resource.WithProcess(),
		resource.WithHost(),
	)
	if err != nil {
		return noop, fmt.Errorf("tracing resource: %w", err)
	}

	exporter, err := newExporter(ctx)
	if err != nil {
		return noop, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sampleRateSampler())),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		log.Warn().Err(err).Msg("otel export error")
	}))

	return tp.Shutdown, nil
}

// newExporter returns an OTLP HTTP exporter when OTEL_EXPORTER_OTLP_ENDPOINT is
// set, otherwise a no-op exporter that discards all spans.
func newExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return &nopExporter{}, nil
	}
	return otlptracehttp.New(ctx)
}

// sampleRateSampler returns a Sampler based on the OTEL_SAMPLE_RATE env var.
// Defaults to AlwaysSample when unset or invalid.
func sampleRateSampler() sdktrace.Sampler {
	v := os.Getenv("OTEL_SAMPLE_RATE")
	if v == "" {
		return sdktrace.AlwaysSample()
	}
	r, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return sdktrace.AlwaysSample()
	}
	if r >= 1.0 {
		return sdktrace.AlwaysSample()
	}
	if r <= 0 {
		return sdktrace.NeverSample()
	}
	return sdktrace.TraceIDRatioBased(r)
}

func noop(_ context.Context) error { return nil }

// nopExporter discards all spans. Used when no OTLP endpoint is configured.
type nopExporter struct{}

func (*nopExporter) ExportSpans(_ context.Context, _ []sdktrace.ReadOnlySpan) error { return nil }
func (*nopExporter) Shutdown(_ context.Context) error                               { return nil }
