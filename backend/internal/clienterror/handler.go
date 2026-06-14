// Package clienterror ingests browser-side error reports and forwards them to
// the structured log (event=client_error) so frontend crashes surface in Loki
// and Grafana alongside backend errors — no third-party error-tracking SDK.
package clienterror

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/apierror"
)

type report struct {
	Message string `json:"message"`
	Stack   string `json:"stack,omitempty"`
	URL     string `json:"url,omitempty"`
	// Kind is the origin: "error" (window.onerror), "unhandledrejection", or
	// "boundary" (a React error boundary).
	Kind string `json:"kind,omitempty"`
}

// field length caps keep a single log line bounded regardless of the (already
// body-size-limited) request.
const (
	maxKind    = 32
	maxURL     = 512
	maxMessage = 1024
	maxStack   = 4096
)

// NewHandler returns the POST /client-errors handler. It is unauthenticated by
// design (errors happen before/around auth) and relies on the global per-IP
// rate limit and request-body cap that wrap the API group.
func NewHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rep report
		if err := json.NewDecoder(r.Body).Decode(&rep); err != nil || rep.Message == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
			return
		}

		zerolog.Ctx(r.Context()).Warn().
			Str("event", "client_error").
			Str("client_kind", truncate(rep.Kind, maxKind)).
			Str("client_url", truncate(stripQuery(rep.URL), maxURL)).
			Str("client_message", truncate(rep.Message, maxMessage)).
			Str("client_stack", truncate(rep.Stack, maxStack)).
			Msg("client error reported")

		w.WriteHeader(http.StatusNoContent)
	})
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}

// stripQuery drops the query string and fragment so tokens/PII a client may
// have included in the URL never reach the logs, even though the trusted
// frontend already sends only origin+path.
func stripQuery(rawURL string) string {
	if before, _, found := strings.Cut(rawURL, "?"); found {
		rawURL = before
	}
	if before, _, found := strings.Cut(rawURL, "#"); found {
		rawURL = before
	}
	return rawURL
}
