package tracing

import (
	"context"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type pgxSpanKey struct{}

// PgxTracer implements pgx.QueryTracer using OpenTelemetry spans.
// Assign it to pgxpool.Config.ConnConfig.Tracer to get automatic per-query
// child spans with the SQL statement and affected rows as attributes.
type PgxTracer struct {
	tracer trace.Tracer
}

// NewPgxTracer returns a PgxTracer backed by the global TracerProvider.
func NewPgxTracer() *PgxTracer {
	return &PgxTracer{tracer: otel.Tracer("circl/pgx")}
}

func (t *PgxTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	ctx, span := t.tracer.Start(ctx, "pgx.query",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.statement", data.SQL),
		),
	)
	return context.WithValue(ctx, pgxSpanKey{}, span)
}

func (t *PgxTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	span, ok := ctx.Value(pgxSpanKey{}).(trace.Span)
	if !ok {
		return
	}
	if data.Err != nil {
		span.RecordError(data.Err)
	}
	span.SetAttributes(attribute.Int64("db.rows_affected", data.CommandTag.RowsAffected()))
	span.End()
}
