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
	StatusPending    = "pending"
	StatusApproved   = "approved"
	StatusRejected   = "rejected"
	StatusSkipped    = "skipped"
	StatusQuarantined = "quarantined"
)

// Severity tells the orchestration layer how to treat an *error* from a
// moderator (a rejection is always honored — this only governs failures).
//
//   - SeveritySoft: fail open. A transient failure logs and allows, because
//     the detector is a quality/policy filter (NSFW, heuristics) and we never
//     want a flaky classifier to block legitimate uploads.
//   - SeverityHard: fail closed. A failure must NOT allow the image through —
//     the upload stays unapproved (never served) and is retried — because the
//     detector is a legal floor (CSAM, NCII, the operator block list).
type Severity int

const (
	SeveritySoft Severity = iota
	SeverityHard
)

// Disposition is what the worker does with the stored object when a moderator
// rejects. It rides on the Decision so the policy lives with the detector that
// made the call, not in a switch downstream.
//
//   - DispositionRetain: keep the object in place for admin review (NSFW /
//     heuristic — legal-to-store content that may be a false positive).
//   - DispositionPurge: delete the object immediately (operator block-list and
//     NCII matches — remove and do not retain).
//   - DispositionQuarantine: move the object to a restricted, never-served
//     location and preserve it (CSAM — destroying it can itself be unlawful;
//     the legal duty is preserve + report).
type Disposition int

const (
	DispositionRetain Disposition = iota
	DispositionPurge
	DispositionQuarantine
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
	// Disposition is what the worker does with the stored object on a
	// rejection. Zero value (DispositionRetain) keeps the file for admin
	// review; detectors that must purge or quarantine set it explicitly.
	Disposition Disposition
}

// Allow is the canonical "no objection" decision.
func Allow() Decision { return Decision{Allowed: true} }

// Reject builds a rejection decision that retains the file for admin review.
// Source should be the package or concrete moderator name so the audit row is
// self-explanatory. Detectors that need a different file disposition use
// RejectWith.
func Reject(code, reason, source string) Decision {
	return Decision{Allowed: false, Code: code, Reason: reason, Source: source, Disposition: DispositionRetain}
}

// RejectWith is Reject with an explicit file disposition (purge / quarantine).
func RejectWith(code, reason, source string, d Disposition) Decision {
	return Decision{Allowed: false, Code: code, Reason: reason, Source: source, Disposition: d}
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
// infrastructure problem (DB unreachable, classifier crashed). How the
// orchestration layer treats an error is governed by Severity: SeveritySoft
// detectors fail open (log + allow), SeverityHard detectors fail closed (the
// upload stays unapproved and is retried — never allowed through on error).
type Moderator interface {
	Check(ctx context.Context, in Input) (Decision, error)
	// Name is recorded in logs and in the audit trail.
	Name() string
	// Severity governs error handling — see Severity.
	Severity() Severity
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
			return Decision{}, &ChainError{Name: m.Name(), Err: err, Hard: m.Severity() == SeverityHard}
		}
		if !d.Allowed {
			return d, nil
		}
	}
	return Allow(), nil
}

// Severity reports the strictest severity among the chain's moderators, so a
// nested Chain still fails closed when any member is a legal floor.
func (c *Chain) Severity() Severity {
	for _, m := range c.mods {
		if m.Severity() == SeverityHard {
			return SeverityHard
		}
	}
	return SeveritySoft
}

// Name returns the composite name for logging.
func (c *Chain) Name() string {
	parts := make([]string, len(c.mods))
	for i, m := range c.mods {
		parts[i] = m.Name()
	}
	return "chain[" + strings.Join(parts, ",") + "]"
}

// ChainError annotates which moderator failed inside a Chain. Hard is true
// when the failing moderator is SeverityHard, so the orchestration layer knows
// it must fail closed (hold + retry) rather than fail open.
type ChainError struct {
	Name string
	Err  error
	Hard bool
}

func (e *ChainError) Error() string { return e.Name + ": " + e.Err.Error() }
func (e *ChainError) Unwrap() error { return e.Err }

// ErrInputMissing is returned by detectors that need a field the caller
// didn't populate (e.g. a classifier called without Bytes).
var ErrInputMissing = errors.New("moderation: required input field missing")
