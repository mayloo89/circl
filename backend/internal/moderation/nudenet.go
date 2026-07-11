package moderation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// DefaultHTTPClassifierTimeout caps how long a single classify call can take.
// NudeNet on CPU averages 200-600 ms per image; 5 s gives plenty of headroom
// for cold-start latency without letting the worker stall on a hung sidecar.
const DefaultHTTPClassifierTimeout = 5 * time.Second

// HTTPClassifierConfig configures the HTTPNSFWClassifier.
type HTTPClassifierConfig struct {
	// BaseURL is the sidecar's root (no trailing slash), e.g. "http://moderation:8000".
	BaseURL string
	// Timeout per request. Zero means use DefaultHTTPClassifierTimeout.
	Timeout time.Duration
	// HTTPClient lets callers inject a custom client for testing or to share
	// connection pools with the rest of the app. Zero value uses a fresh client
	// with the configured Timeout.
	HTTPClient *http.Client
}

// HTTPNSFWClassifier implements NSFWClassifier by POSTing the image bytes
// to a remote sidecar that runs NudeNet (or any other model exposing the
// same `{nsfw_score, categories, detections}` contract). Errors propagate
// up so the orchestrator can apply its fail-open policy.
type HTTPNSFWClassifier struct {
	url    string
	client *http.Client
}

// NewHTTPNSFWClassifier returns a classifier that talks to the sidecar at
// cfg.BaseURL. Hits `POST {BaseURL}/classify` with a multipart body.
func NewHTTPNSFWClassifier(cfg HTTPClassifierConfig) *HTTPNSFWClassifier {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultHTTPClassifierTimeout
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return &HTTPNSFWClassifier{
		url:    cfg.BaseURL + "/classify",
		client: client,
	}
}

// classifyResponse mirrors the sidecar's JSON shape.
type classifyResponse struct {
	NSFWScore  float64  `json:"nsfw_score"`
	Categories []string `json:"categories"`
}

// Classify POSTs the bytes as a multipart "file" field and parses the
// response. A non-200 from the sidecar surfaces as an error — the NSFW
// moderator then logs and fails open per the pipeline policy.
func (c *HTTPNSFWClassifier) Classify(ctx context.Context, raw []byte) (NSFWResult, error) {
	if len(raw) == 0 {
		return NSFWResult{}, fmt.Errorf("moderation/nudenet: empty bytes")
	}

	body := new(bytes.Buffer)
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", "upload")
	if err != nil {
		return NSFWResult{}, fmt.Errorf("moderation/nudenet: create form file: %w", err)
	}
	if _, err := part.Write(raw); err != nil {
		return NSFWResult{}, fmt.Errorf("moderation/nudenet: write form: %w", err)
	}
	if err := w.Close(); err != nil {
		return NSFWResult{}, fmt.Errorf("moderation/nudenet: close form: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, body)
	if err != nil {
		return NSFWResult{}, fmt.Errorf("moderation/nudenet: new request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return NSFWResult{}, fmt.Errorf("moderation/nudenet: do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		// Drain a small chunk of the body for diagnostics without letting a
		// hostile sidecar dump arbitrary bytes into our logs.
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return NSFWResult{}, fmt.Errorf("moderation/nudenet: status %d: %s", resp.StatusCode, snippet)
	}

	var parsed classifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return NSFWResult{}, fmt.Errorf("moderation/nudenet: decode response: %w", err)
	}

	return NSFWResult{
		Probability: parsed.NSFWScore,
		Categories:  parsed.Categories,
	}, nil
}
