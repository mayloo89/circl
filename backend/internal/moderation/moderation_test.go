package moderation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mayloo89/circl/backend/internal/moderation"
)

// --- Chain ---

type fakeMod struct {
	name string
	d    moderation.Decision
	err  error
}

func (f *fakeMod) Name() string { return f.name }
func (f *fakeMod) Check(_ context.Context, _ moderation.Input) (moderation.Decision, error) {
	return f.d, f.err
}

func TestChain_AllowsWhenEveryoneAllows(t *testing.T) {
	c := moderation.NewChain(
		&fakeMod{name: "a", d: moderation.Allow()},
		&fakeMod{name: "b", d: moderation.Allow()},
	)
	d, err := c.Check(t.Context(), moderation.Input{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !d.Allowed {
		t.Error("expected Allow")
	}
}

func TestChain_FirstRejectWins(t *testing.T) {
	c := moderation.NewChain(
		&fakeMod{name: "a", d: moderation.Allow()},
		&fakeMod{name: "b", d: moderation.Reject(moderation.CodeHashMatch, "x", "b")},
		// This third moderator must not run — the chain should short-circuit.
		&fakeMod{name: "c", d: moderation.Reject(moderation.CodeNSFWDetected, "should-not-fire", "c")},
	)
	d, err := c.Check(t.Context(), moderation.Input{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Allowed {
		t.Error("expected Reject")
	}
	if d.Code != moderation.CodeHashMatch {
		t.Errorf("code = %q, want hash_match", d.Code)
	}
	if d.Source != "b" {
		t.Errorf("source = %q, want b", d.Source)
	}
}

func TestChain_ErrorWraps(t *testing.T) {
	dbErr := errors.New("db down")
	c := moderation.NewChain(
		&fakeMod{name: "a", d: moderation.Allow()},
		&fakeMod{name: "b", err: dbErr},
	)
	_, err := c.Check(t.Context(), moderation.Input{})
	if err == nil {
		t.Fatal("expected error")
	}
	var chainErr *moderation.ChainError
	if !errors.As(err, &chainErr) {
		t.Fatalf("expected ChainError, got %T", err)
	}
	if chainErr.Name != "b" {
		t.Errorf("name = %q, want b", chainErr.Name)
	}
	if !errors.Is(err, dbErr) {
		t.Error("expected error chain to unwrap to dbErr")
	}
}

// --- HashFor ---

func TestHashFor_StableAndCollisionResistant(t *testing.T) {
	first := moderation.HashFor([]byte("hello"))
	second := moderation.HashFor([]byte("hello"))
	if first != second {
		t.Error("hash not stable")
	}
	if moderation.HashFor([]byte("a")) == moderation.HashFor([]byte("b")) {
		t.Error("hashes collided")
	}
}

// --- Heuristic ---

func TestHeuristic_TooSmall(t *testing.T) {
	h := moderation.NewHeuristic()
	d, _ := h.Check(t.Context(), moderation.Input{SizeBytes: 100, Width: 64, Height: 64})
	if d.Allowed {
		t.Error("expected reject for tiny file")
	}
	if d.Code != moderation.CodeSizeOutOfBounds {
		t.Errorf("code = %q, want size_out_of_bounds", d.Code)
	}
}

func TestHeuristic_ExtremeAspect(t *testing.T) {
	h := moderation.NewHeuristic()
	d, _ := h.Check(t.Context(), moderation.Input{SizeBytes: 10_000, Width: 5000, Height: 100})
	if d.Allowed {
		t.Error("expected reject for extreme aspect ratio")
	}
	if d.Code != moderation.CodeAspectOutOfBounds {
		t.Errorf("code = %q, want aspect_ratio_out_of_bounds", d.Code)
	}
}

func TestHeuristic_NormalImagePasses(t *testing.T) {
	h := moderation.NewHeuristic()
	d, err := h.Check(t.Context(), moderation.Input{SizeBytes: 500_000, Width: 1024, Height: 768})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !d.Allowed {
		t.Errorf("expected allow, got reject: %s", d.Reason)
	}
}

func TestHeuristic_MissingDimensionsAllows(t *testing.T) {
	// When width/height are zero (non-image, or caller didn't decode),
	// dimensional checks are skipped — size still applies.
	h := moderation.NewHeuristic()
	d, _ := h.Check(t.Context(), moderation.Input{SizeBytes: 50_000})
	if !d.Allowed {
		t.Error("expected allow when dimensions are zero and size is fine")
	}
}

// --- HashList ---

type fakeHashStore struct {
	hits map[string]*moderation.HashEntry
}

func (s *fakeHashStore) Lookup(_ context.Context, h string) (*moderation.HashEntry, error) {
	if entry, ok := s.hits[h]; ok {
		return entry, nil
	}
	return nil, moderation.ErrHashNotFound
}
func (s *fakeHashStore) Add(_ context.Context, _ moderation.HashEntry, _ string) error {
	return nil
}

func TestHashList_HitRejects(t *testing.T) {
	store := &fakeHashStore{hits: map[string]*moderation.HashEntry{
		"abc123": {Hash: "abc123", Source: "local", Reason: "test sample"},
	}}
	h := moderation.NewHashList(store)
	d, err := h.Check(t.Context(), moderation.Input{Hash: "abc123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Allowed {
		t.Error("expected reject")
	}
	if d.Code != moderation.CodeHashMatch {
		t.Errorf("code = %q, want hash_match", d.Code)
	}
	if d.Source != "hashlist:local" {
		t.Errorf("source = %q, want hashlist:local", d.Source)
	}
}

func TestHashList_MissAllows(t *testing.T) {
	h := moderation.NewHashList(&fakeHashStore{hits: map[string]*moderation.HashEntry{}})
	d, err := h.Check(t.Context(), moderation.Input{Hash: "nope"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !d.Allowed {
		t.Error("expected allow for unknown hash")
	}
}

func TestHashList_EmptyHashErrors(t *testing.T) {
	h := moderation.NewHashList(&fakeHashStore{})
	_, err := h.Check(t.Context(), moderation.Input{Hash: ""})
	if !errors.Is(err, moderation.ErrInputMissing) {
		t.Errorf("err = %v, want ErrInputMissing", err)
	}
}

// --- NSFW ---

type fakeClassifier struct {
	prob float64
	err  error
}

func (f fakeClassifier) Classify(_ context.Context, _ []byte) (moderation.NSFWResult, error) {
	return moderation.NSFWResult{Probability: f.prob}, f.err
}

func TestNSFW_AboveThresholdRejects(t *testing.T) {
	n := moderation.NewNSFW(fakeClassifier{prob: 0.95}, 0.8)
	d, _ := n.Check(t.Context(), moderation.Input{Bytes: []byte("x")})
	if d.Allowed {
		t.Error("expected reject above threshold")
	}
	if d.Code != moderation.CodeNSFWDetected {
		t.Errorf("code = %q, want nsfw_detected", d.Code)
	}
}

func TestNSFW_BelowThresholdAllows(t *testing.T) {
	n := moderation.NewNSFW(fakeClassifier{prob: 0.30}, 0.8)
	d, _ := n.Check(t.Context(), moderation.Input{Bytes: []byte("x")})
	if !d.Allowed {
		t.Error("expected allow below threshold")
	}
}

func TestNSFW_NoopClassifier(t *testing.T) {
	n := moderation.NewNSFW(nil, 0) // nil → NoopClassifier, 0 → default threshold
	d, err := n.Check(t.Context(), moderation.Input{Bytes: []byte("x")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !d.Allowed {
		t.Error("Noop classifier should always allow")
	}
}

func TestNSFW_EmptyBytesErrors(t *testing.T) {
	n := moderation.NewNSFW(fakeClassifier{}, 0.8)
	_, err := n.Check(t.Context(), moderation.Input{})
	if !errors.Is(err, moderation.ErrInputMissing) {
		t.Errorf("err = %v, want ErrInputMissing", err)
	}
}

func TestNSFW_PrivateContextSkipsRejection(t *testing.T) {
	// Private-album uploads run with ContextPrivate. Even if the classifier
	// would have flagged the image as explicit, NSFW.Check must return
	// Allow() — the consent gate sits one layer up at the album-grant level.
	n := moderation.NewNSFW(fakeClassifier{prob: 0.99}, 0.8)
	d, err := n.Check(t.Context(), moderation.Input{
		Bytes:   []byte("x"),
		Context: moderation.ContextPrivate,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !d.Allowed {
		t.Errorf("private context with high score should allow, got reject: %s", d.Reason)
	}
}

func TestNSFW_PrivateContextSkipsEmptyBytesCheck(t *testing.T) {
	// Private context short-circuits before the input validation, so callers
	// that haven't decoded the image yet can still funnel through without
	// getting ErrInputMissing.
	n := moderation.NewNSFW(fakeClassifier{}, 0.8)
	d, err := n.Check(t.Context(), moderation.Input{Context: moderation.ContextPrivate})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !d.Allowed {
		t.Error("expected allow")
	}
}
