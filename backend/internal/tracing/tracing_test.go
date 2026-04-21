package tracing_test

import (
	"testing"

	"go.opentelemetry.io/otel"

	"github.com/mayloo89/circl/backend/internal/tracing"
)

func TestInit_NoEndpoint(t *testing.T) {
	// OTEL_EXPORTER_OTLP_ENDPOINT is unset → uses no-op exporter; must not error.
	ctx := t.Context()
	shutdown, err := tracing.Init(ctx, "test-svc", "0.0.0", "test")
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	defer shutdown(ctx) //nolint:errcheck

	_, span := otel.Tracer("test").Start(ctx, "test-span")
	if !span.SpanContext().IsValid() {
		t.Error("expected a valid span context after Init")
	}
	span.End()
}

func TestInit_SetsGlobalTracerProvider(t *testing.T) {
	ctx := t.Context()
	shutdown, err := tracing.Init(ctx, "test-svc", "0.0.0", "test")
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	defer shutdown(ctx) //nolint:errcheck

	if otel.GetTracerProvider() == nil {
		t.Fatal("expected non-nil global TracerProvider after Init")
	}
}

func TestInit_ShutdownIdempotent(t *testing.T) {
	ctx := t.Context()
	shutdown, err := tracing.Init(ctx, "test-svc", "0.0.0", "test")
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	// Calling shutdown twice must not panic.
	if err := shutdown(ctx); err != nil {
		t.Errorf("first shutdown: %v", err)
	}
	if err := shutdown(ctx); err != nil {
		t.Errorf("second shutdown: %v", err)
	}
}
