package moderation_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mayloo89/circl/backend/internal/moderation"
)

func TestHTTPNSFWClassifier_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/classify" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Errorf("expected multipart/form-data, got %q", r.Header.Get("Content-Type"))
		}
		// Echo back a representative response.
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"nsfw_score": 0.92,
			"categories": []string{"FEMALE_BREAST_EXPOSED"},
		})
	}))
	defer srv.Close()

	c := moderation.NewHTTPNSFWClassifier(moderation.HTTPClassifierConfig{BaseURL: srv.URL})
	res, err := c.Classify(t.Context(), []byte("fake-png-bytes"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Probability != 0.92 {
		t.Errorf("probability = %v, want 0.92", res.Probability)
	}
	if len(res.Categories) != 1 || res.Categories[0] != "FEMALE_BREAST_EXPOSED" {
		t.Errorf("categories = %v, want [FEMALE_BREAST_EXPOSED]", res.Categories)
	}
}

func TestHTTPNSFWClassifier_EmptyBytesErrors(t *testing.T) {
	c := moderation.NewHTTPNSFWClassifier(moderation.HTTPClassifierConfig{BaseURL: "http://unused"})
	_, err := c.Classify(t.Context(), nil)
	if err == nil {
		t.Fatal("expected error for empty bytes")
	}
}

func TestHTTPNSFWClassifier_Non200Errors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, "upstream down")
	}))
	defer srv.Close()

	c := moderation.NewHTTPNSFWClassifier(moderation.HTTPClassifierConfig{BaseURL: srv.URL})
	_, err := c.Classify(t.Context(), []byte("x"))
	if err == nil {
		t.Fatal("expected error on non-200")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Errorf("error should mention status code, got %v", err)
	}
}

func TestHTTPNSFWClassifier_MalformedJSONErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "not json")
	}))
	defer srv.Close()

	c := moderation.NewHTTPNSFWClassifier(moderation.HTTPClassifierConfig{BaseURL: srv.URL})
	_, err := c.Classify(t.Context(), []byte("x"))
	if err == nil {
		t.Fatal("expected error on malformed JSON")
	}
}
