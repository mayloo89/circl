package moderation

import "context"

// NSFW is the seam for the future NudeNet / Rekognition classifier. The
// orchestrator calls it inside the asynq worker (slow lane) — failure here
// is logged and treated as allow because we never want a transient model
// outage to block uploads.
type NSFW struct {
	classifier NSFWClassifier
	threshold  float64
}

// NSFWClassifier is the model-side seam. A real NudeNet adapter implements
// this; the bundled NoopClassifier returns "0% confidence" for every image
// so the pipeline runs end-to-end today without an actual model installed.
type NSFWClassifier interface {
	Classify(ctx context.Context, bytes []byte) (NSFWResult, error)
}

// NSFWResult is what a classifier returns. Probability is on [0, 1].
// Categories is optional metadata (e.g. ["nudity", "graphic_violence"]).
type NSFWResult struct {
	Probability float64
	Categories  []string
}

// DefaultNSFWThreshold is the probability above which an image is rejected.
// Tuned conservatively — we'd rather under-block until a real model is
// installed than have false positives flood the moderation queue.
const DefaultNSFWThreshold = 0.80

// NewNSFW returns an NSFW detector. If classifier is nil, NoopClassifier is
// used and every Check returns Allow().
func NewNSFW(classifier NSFWClassifier, threshold float64) *NSFW {
	if classifier == nil {
		classifier = NoopClassifier{}
	}
	if threshold <= 0 {
		threshold = DefaultNSFWThreshold
	}
	return &NSFW{classifier: classifier, threshold: threshold}
}

// Name returns the detector name.
func (n *NSFW) Name() string { return "nsfw" }

// Check runs the classifier on the bytes and rejects if probability exceeds
// the configured threshold. Errors from the classifier are returned so the
// orchestrator can decide; the typical policy is fail-open.
func (n *NSFW) Check(ctx context.Context, in Input) (Decision, error) {
	if len(in.Bytes) == 0 {
		return Decision{}, ErrInputMissing
	}
	res, err := n.classifier.Classify(ctx, in.Bytes)
	if err != nil {
		return Decision{}, err
	}
	if res.Probability >= n.threshold {
		return Reject(CodeNSFWDetected, "explicit-content classifier flagged this image", n.Name()), nil
	}
	return Allow(), nil
}

// NoopClassifier is the bundled stub that scores every image at 0.0. Use it
// in dev, in tests, and as the production default until the real NudeNet /
// Rekognition adapter lands behind the same NSFWClassifier interface.
type NoopClassifier struct{}

// Classify returns 0% probability for every input.
func (NoopClassifier) Classify(_ context.Context, _ []byte) (NSFWResult, error) {
	return NSFWResult{Probability: 0.0}, nil
}
