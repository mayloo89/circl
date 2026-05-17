package moderation

import (
	"context"
	"fmt"
)

// Default Heuristic bounds. Conservative defaults that won't reject legitimate
// product uploads (avatars, gallery photos, chat attachments) but catch
// obvious spam patterns like 1×1-px pixels or 10000×100 banner ads.
const (
	defaultMinSizeBytes   = 1024     // 1 KB — anything tinier is almost certainly junk
	defaultMaxSizeBytes   = 0        // 0 = no upper bound at this layer (storage caps elsewhere)
	defaultMinDimension   = 16       // both width and height must be at least this many px
	defaultMaxAspectRatio = 6.0      // reject ratios more extreme than 6:1 in either direction
)

// Heuristic catches uploads with sizes or proportions that are almost
// always spam or a probe — too-small files, extreme aspect ratios, etc.
// Bounds are configurable so each surface can tune (e.g. avatars get a
// tighter ratio than chat attachments).
type Heuristic struct {
	MinSizeBytes   int64
	MaxSizeBytes   int64 // 0 disables the upper-bound check
	MinDimension   int
	MaxAspectRatio float64
}

// NewHeuristic returns a Heuristic with sensible defaults that pass every
// realistic product upload while still catching obvious spam.
func NewHeuristic() *Heuristic {
	return &Heuristic{
		MinSizeBytes:   defaultMinSizeBytes,
		MaxSizeBytes:   defaultMaxSizeBytes,
		MinDimension:   defaultMinDimension,
		MaxAspectRatio: defaultMaxAspectRatio,
	}
}

// Name returns the detector name.
func (h *Heuristic) Name() string { return "heuristic" }

// Check evaluates size and shape bounds; the first violation wins. Width and
// Height are required — pass them as 0 only for non-image uploads that
// should be skipped by the orchestrator, not by this detector.
func (h *Heuristic) Check(_ context.Context, in Input) (Decision, error) {
	if in.SizeBytes > 0 && in.SizeBytes < h.MinSizeBytes {
		return Reject(
			CodeSizeOutOfBounds,
			fmt.Sprintf("file is too small (%d bytes); minimum is %d", in.SizeBytes, h.MinSizeBytes),
			h.Name(),
		), nil
	}
	if h.MaxSizeBytes > 0 && in.SizeBytes > h.MaxSizeBytes {
		return Reject(
			CodeSizeOutOfBounds,
			fmt.Sprintf("file is too large (%d bytes); maximum is %d", in.SizeBytes, h.MaxSizeBytes),
			h.Name(),
		), nil
	}
	if in.Width > 0 && in.Height > 0 {
		if in.Width < h.MinDimension || in.Height < h.MinDimension {
			return Reject(
				CodeSizeOutOfBounds,
				fmt.Sprintf("image dimensions are too small (%d×%d); minimum is %d×%d", in.Width, in.Height, h.MinDimension, h.MinDimension),
				h.Name(),
			), nil
		}
		ratio := aspectRatio(in.Width, in.Height)
		if ratio > h.MaxAspectRatio {
			return Reject(
				CodeAspectOutOfBounds,
				fmt.Sprintf("image aspect ratio is too extreme (%.2f); maximum is %.2f", ratio, h.MaxAspectRatio),
				h.Name(),
			), nil
		}
	}
	return Allow(), nil
}

func aspectRatio(w, h int) float64 {
	if w == 0 || h == 0 {
		return 0
	}
	if w >= h {
		return float64(w) / float64(h)
	}
	return float64(h) / float64(w)
}
