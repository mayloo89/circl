// Package moderation implements the per-upload image moderation pipeline.
//
// Design intent
//
//   The pipeline is split into a synchronous fast lane (hash list lookup +
//   cheap heuristics) that runs at ConfirmUpload time and an asynchronous
//   slow lane (the NSFW classifier) that runs inside the asynq image worker.
//   Sync rejection responds to the client immediately so the upload UX
//   surfaces the reason in the same request; async rejection arrives via the
//   notifications hub.
//
//   Every concrete detector implements the same `Moderator` interface, so
//   future integrations (StopNCII feed → HashList, NudeNet → NSFW, Cloudflare
//   CSAM → its own Moderator) plug in without touching the pipeline orchestration.
package moderation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

// Decision codes are stable strings recorded in `uploads.moderation_code` so
// the admin queue can group / count rejections by reason.
const (
	CodeHashMatch         = "hash_match"
	CodeSizeOutOfBounds   = "size_out_of_bounds"
	CodeAspectOutOfBounds = "aspect_ratio_out_of_bounds"
	CodeNSFWDetected      = "nsfw_detected"
)

// Context names describe the surface the upload is destined for. Different
// surfaces have different content policies — the NSFW classifier blocks
// explicit content on public surfaces, but only tags it on private albums
// (where the recipient has explicitly consented via a grant). Hash-list
// matches (CSAM / NCII) and heuristic checks always block, regardless of
// context — they're legal floors, not surface-policy.
const (
	ContextPublic  = "public"
	ContextPrivate = "private"
)

// Status values written to `uploads.moderation_status`.
const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusRejected = "rejected"
	StatusSkipped  = "skipped"
)

// Decision is what every Moderator returns.
type Decision struct {
	// Allowed is the only field consumers should branch on. Reason and Code
	// are populated when Allowed is false.
	Allowed bool
	// Code is a stable machine-readable string (see Code* constants).
	Code string
	// Reason is a short human-readable explanation rendered to the uploader.
	Reason string
	// Source records which moderator made the call (for the audit trail).
	Source string
	// Score is the classifier confidence in [0, 1]. Populated by NSFW
	// rejections; zero for moderators that don't have a probability (hash
	// list, heuristics).
	Score float64
	// Categories are per-region labels the classifier returned (e.g.
	// "FEMALE_BREAST_EXPOSED"). Populated by NSFW rejections; nil otherwise.
	Categories []string
}

// Allow is the canonical "no objection" decision.
func Allow() Decision { return Decision{Allowed: true} }

// Reject builds a rejection decision. Source should be the package or
// concrete moderator name so the audit row is self-explanatory.
func Reject(code, reason, source string) Decision {
	return Decision{Allowed: false, Code: code, Reason: reason, Source: source}
}

// Input is the bag of data each moderator consumes. Bytes is optional —
// detectors that only need metadata (HashList from a precomputed hash,
// Heuristic from dimensions) can be passed an empty slice. Hash is
// precomputed once at the orchestration layer to avoid every moderator
// hashing the bytes again.
//
// Context tags the destination surface (ContextPublic / ContextPrivate).
// Empty defaults to public, so pre-existing callers keep their semantics.
type Input struct {
	ContentType string
	SizeBytes   int64
	Width       int
	Height      int
	Hash        string
	Bytes       []byte
	Context     string
}

// HashFor returns the hex-encoded SHA-256 of b. Exposed so the orchestration
// layer can compute the hash once and pass it through Input.
func HashFor(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Moderator is the interface every detector implements. Errors signal an
// infrastructure problem (DB unreachable, classifier crashed) — the
// orchestration layer treats them as "fail open" by default (allow) and
// logs them; we never block a user upload on transient backend trouble.
type Moderator interface {
	Check(ctx context.Context, in Input) (Decision, error)
	// Name is recorded in logs and in the audit trail.
	Name() string
}

// Chain runs each moderator in order and returns on the first rejection.
// Approved decisions are silent; we only surface the rejection.
type Chain struct {
	mods []Moderator
}

// NewChain returns a Chain composed of the given moderators.
func NewChain(mods ...Moderator) *Chain {
	return &Chain{mods: mods}
}

// Check runs the chain. The first moderator that rejects wins. Errors from
// any moderator are returned wrapped with the moderator name so the caller
// can log them; the caller decides whether to fail open or hard.
func (c *Chain) Check(ctx context.Context, in Input) (Decision, error) {
	for _, m := range c.mods {
		d, err := m.Check(ctx, in)
		if err != nil {
			return Decision{}, &ChainError{Name: m.Name(), Err: err}
		}
		if !d.Allowed {
			return d, nil
		}
	}
	return Allow(), nil
}

// Name returns the composite name for logging.
func (c *Chain) Name() string {
	parts := make([]string, len(c.mods))
	for i, m := range c.mods {
		parts[i] = m.Name()
	}
	return "chain[" + strings.Join(parts, ",") + "]"
}

// ChainError annotates which moderator failed inside a Chain.
type ChainError struct {
	Name string
	Err  error
}

func (e *ChainError) Error() string { return e.Name + ": " + e.Err.Error() }
func (e *ChainError) Unwrap() error { return e.Err }

// ErrInputMissing is returned by detectors that need a field the caller
// didn't populate (e.g. a classifier called without Bytes).
var ErrInputMissing = errors.New("moderation: required input field missing")
