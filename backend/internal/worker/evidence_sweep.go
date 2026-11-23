package worker

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/evidence"
)

// evidenceSweepBatch caps one pass; a backlog drains over the following days.
const evidenceSweepBatch = 500

// EvidenceSweeper is the evidence service's sweep entry point.
type EvidenceSweeper interface {
	Sweep(ctx context.Context, batch int) (evidence.SweepResult, error)
}

// SweepReportEvidence discards the evidence of dismissed reports and of
// reviewed reports past their retention period. Intended for the daily
// maintenance ticker; failures are logged and retried on the next run.
func SweepReportEvidence(ctx context.Context, log zerolog.Logger, svc EvidenceSweeper) {
	log = log.With().Str("component", "evidence_sweep").Logger()
	res, err := svc.Sweep(ctx, evidenceSweepBatch)
	if err != nil {
		log.Error().Err(err).Int("failed", res.Failed).Msg("report evidence sweep had failures")
	}
	if res.Purged > 0 {
		log.Info().Int("count", res.Purged).Msg("purged report evidence")
	}
}
