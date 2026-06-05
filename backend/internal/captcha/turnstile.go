// Package captcha verifies human-challenge tokens. The only implementation is
// Cloudflare Turnstile, but callers depend on the Verify func signature, not
// the concrete type, so the provider can be swapped without touching them.
package captcha

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// Turnstile verifies Cloudflare Turnstile tokens via the siteverify endpoint.
type Turnstile struct {
	secret string
	url    string
	client *http.Client
}

// NewTurnstile returns a verifier bound to the given secret key.
func NewTurnstile(secret string) *Turnstile {
	return &Turnstile{
		secret: secret,
		url:    turnstileVerifyURL,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// Verify reports whether token is a valid Turnstile response. remoteIP is the
// client IP (optional, improves Cloudflare's scoring); pass "" to omit it.
func (t *Turnstile) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	if token == "" {
		return false, nil
	}
	form := url.Values{"secret": {t.secret}, "response": {token}}
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url, strings.NewReader(form.Encode()))
	if err != nil {
		return false, fmt.Errorf("captcha: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("captcha: verify request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var out struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, fmt.Errorf("captcha: decode response: %w", err)
	}
	return out.Success, nil
}
