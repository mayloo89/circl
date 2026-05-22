package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/mayloo89/circl/backend/internal/apierror"
)

// TicketRedeemer consumes a single-use ticket and returns the associated userID.
// Tickets are deleted on first use and expire after a short TTL.
type TicketRedeemer interface {
	Redeem(ctx context.Context, ticket string) (string, error)
}

// ErrInvalidTicket is returned by a TicketRedeemer when the ticket does not
// exist or has already been consumed.
var ErrInvalidTicket = errors.New("invalid or expired ticket")

// NewHandler returns an SSE handler for GET /notifications/stream.
//
// Clients authenticate via a ?ticket= query parameter. The ticket must be
// obtained first from POST /ws-ticket (behind RequireAuth). Single-use tickets
// avoid embedding long-lived JWTs in URLs where they would appear in server
// access logs and browser history.
func NewHandler(hub *Hub, redeemer TicketRedeemer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ticket := r.URL.Query().Get("ticket")
		if ticket == "" {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		userID, err := redeemer.Redeem(r.Context(), ticket)
		if err != nil {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}

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

		writeEvent(w, flusher, Event{Type: "connected"})

		for {
			select {
			case e := <-ch:
				writeEvent(w, flusher, e)
			case <-time.After(25 * time.Second):
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
