package worker

import (
	"context"
	"encoding/json"
	"maps"
	"slices"

	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// traceCarrier adapts a map[string]string to otel's TextMapCarrier interface.
type traceCarrier map[string]string

func (c traceCarrier) Get(key string) string { return c[key] }
func (c traceCarrier) Set(key, val string)   { c[key] = val }
func (c traceCarrier) Keys() []string { return slices.Collect(maps.Keys(c)) }

// taskEnvelope wraps any asynq task payload with W3C trace context headers so
// that task execution can be correlated with the HTTP request that enqueued it.
type taskEnvelope struct {
	Trace   traceCarrier    `json:"_trace,omitzero"`
	Payload json.RawMessage `json:"payload"`
}

// InjectTraceContext wraps raw task payload bytes with the current span context
// extracted from ctx. Pass the returned bytes as the asynq task payload.
func InjectTraceContext(ctx context.Context, payload []byte) ([]byte, error) {
	carrier := make(traceCarrier)
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(carrier))
	return json.Marshal(taskEnvelope{Trace: carrier, Payload: json.RawMessage(payload)})
}

// extractTraceContext unwraps an asynq task payload. Returns the original payload
// and a context with the span context restored from the envelope. When the data
// is not wrapped (tasks enqueued before tracing was deployed) it falls back
// gracefully by returning ctx and data unchanged.
func extractTraceContext(ctx context.Context, data []byte) (context.Context, []byte) {
	var env taskEnvelope
	if err := json.Unmarshal(data, &env); err != nil || env.Payload == nil {
		return ctx, data
	}
	if env.Trace != nil {
		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(env.Trace))
	}
	return ctx, []byte(env.Payload)
}

// otelMiddleware is an asynq.MiddlewareFunc that restores distributed trace
// context from the task payload envelope and creates a consumer span covering
// the full task execution. Errors are recorded on the span.
func otelMiddleware(next asynq.Handler) asynq.Handler {
	tracer := otel.Tracer("circl/worker")
	return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		ctx, payload := extractTraceContext(ctx, t.Payload())
		ctx, span := tracer.Start(ctx, "worker."+t.Type(),
			trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(
				attribute.String("messaging.system", "redis"),
				attribute.String("messaging.operation", "process"),
				attribute.String("task.type", t.Type()),
			),
		)
		defer span.End()

		if err := next.ProcessTask(ctx, asynq.NewTask(t.Type(), payload)); err != nil {
			span.RecordError(err)
			return err
		}
		return nil
	})
}
