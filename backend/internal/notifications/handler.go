package notifications

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/mayloo89/circl/backend/internal/apierror"
	"github.com/mayloo89/circl/backend/internal/token"
)

// NewHandler returns an SSE handler for GET /notifications/stream.
//
// Clients authenticate via a ?token=<jwt> query parameter because the
// browser EventSource API does not support custom request headers.
func NewHandler(hub *Hub, jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Validate token from query param.
		tok := r.URL.Query().Get("token")
		if tok == "" {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		claims, err := token.Validate(tok, jwtSecret)
		if err != nil {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		userID := claims.Subject

		// Enrich the span created by the HTTP tracing middleware with SSE-specific
		// attributes so it's easy to filter SSE connections in Tempo.
		trace.SpanFromContext(r.Context()).SetAttributes(
			attribute.String("user.id", userID),
			attribute.String("sse.type", "notifications"),
		)

		flusher, ok := w.(http.Flusher)
		if !ok {
			apierror.Write(w, http.StatusNotImplemented, apierror.CodeInternalError, "streaming unsupported")
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		ch, unsub := hub.Subscribe(userID)
		defer unsub()

		// Initial event so the client knows the stream is live.
		writeEvent(w, flusher, Event{Type: "connected"})

		for {
			select {
			case e := <-ch:
				writeEvent(w, flusher, e)
			case <-time.After(25 * time.Second):
				// SSE comment heartbeat — keeps TCP alive through proxies.
				fmt.Fprintf(w, ": heartbeat\n\n")
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}

func writeEvent(w http.ResponseWriter, f http.Flusher, e Event) {
	data, _ := json.Marshal(e)
	fmt.Fprintf(w, "data: %s\n\n", data)
	f.Flush()
}
