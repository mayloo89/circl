package worker

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/evidence"
)

type fakeSweeper struct {
	res   evidence.SweepResult
	err   error
	batch int
}

func (f *fakeSweeper) Sweep(_ context.Context, batch int) (evidence.SweepResult, error) {
	f.batch = batch
	return f.res, f.err
}

func TestSweepReportEvidence_LogsOutcome(t *testing.T) {
	var buf bytes.Buffer
	s := &fakeSweeper{res: evidence.SweepResult{Purged: 3}}
	SweepReportEvidence(t.Context(), zerolog.New(&buf), s)
	if s.batch != evidenceSweepBatch || !strings.Contains(buf.String(), `"count":3`) {
		t.Errorf("batch = %d, log = %s", s.batch, buf.String())
	}

	buf.Reset()
	SweepReportEvidence(t.Context(), zerolog.New(&buf), &fakeSweeper{res: evidence.SweepResult{Failed: 2}, err: errors.New("offline")})
	if !strings.Contains(buf.String(), `"failed":2`) || !strings.Contains(buf.String(), "offline") {
		t.Errorf("failure not logged: %s", buf.String())
	}

	buf.Reset()
	SweepReportEvidence(t.Context(), zerolog.New(&buf), &fakeSweeper{})
	if buf.Len() != 0 {
		t.Errorf("idle sweep logged: %s", buf.String())
	}
}
