package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

// DBPinger is the minimal interface required by the health handler.
// *pgxpool.Pool satisfies this interface.
type DBPinger interface {
	Ping(ctx context.Context) error
}

// New returns a configured chi router with all application routes registered.
// authHandler is the auth sub-router (auth.NewHandler).
// accountHandler is the user account sub-router (auth.NewAccountHandler); must run behind requireAuth.
// profileHandler is the profiles sub-router (profiles.NewHandler).
// contactsHandler is the contacts sub-router (contacts.NewHandler).
// notificationsHandler is the SSE handler (notifications.NewHandler).
// chatHandler is the REST chat sub-router (chat.NewHandler); must run behind requireAuth.
// chatWSHandler is the WebSocket endpoint (chat.NewWSHandler); handles its own auth via ?token=.
// presenceHandler is the presence sub-router (presence.NewHandler); must run behind requireAuth.
// uploadHandler is the uploads sub-router (uploads.NewHandler); must run behind requireAuth.
// reportsHandler is the reports sub-router (reports.NewManager); must run behind requireAuth.
// localStorageHandler serves uploaded files in dev mode; nil in production.
// requireAuth is the JWT middleware that protects authenticated routes.
func New(db DBPinger, env string, corsOrigins []string, authHandler http.Handler, accountHandler http.Handler, profileHandler http.Handler, contactsHandler http.Handler, notificationsHandler http.Handler, chatHandler http.Handler, chatWSHandler http.Handler, presenceHandler http.Handler, uploadHandler http.Handler, reportsHandler http.Handler, pushHandler http.Handler, localStorageHandler http.Handler, requireAuth func(http.Handler) http.Handler) http.Handler {
	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   corsOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", healthHandler(db, env))
	r.Mount("/auth", authHandler)

	// SSE stream — auth is handled inside the handler via ?token= query param
	// because the browser EventSource API does not support custom headers.
	r.Handle("/notifications/stream", notificationsHandler)

	// WebSocket endpoint — auth is handled inside the handler via ?token=
	// because the browser WebSocket API does not support custom headers.
	r.Handle("/chat/rooms/{id}/ws", chatWSHandler)

	// Protected routes — requireAuth validates the Bearer JWT before forwarding.
	r.Group(func(g chi.Router) {
		g.Use(requireAuth)
		g.Mount("/users/me", accountHandler)
		g.Mount("/profiles", profileHandler)
		g.Mount("/", contactsHandler)
		g.Mount("/chat", chatHandler)
		g.Mount("/presence", presenceHandler)
		g.Mount("/uploads", uploadHandler)
		g.Mount("/reports", reportsHandler)
		g.Mount("/push", pushHandler)
	})

	// Local file serving — only mounted when localStorageHandler is not nil
	// (i.e. when STORAGE_PROVIDER=local for development).
	if localStorageHandler != nil {
		r.Mount("/uploads/files", localStorageHandler)
	}

	return r
}

// healthHandler reports server and database status.
func healthHandler(db DBPinger, env string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "ok"
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			log.Printf("DB health check failed: %v", err)
			dbStatus = "error"
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","env":"%s","db":"%s"}`, env, dbStatus)
	}
}

// NormalizeCORSOrigins splits a comma-separated origins string into a slice,
// trimming whitespace from each entry.
func NormalizeCORSOrigins(raw string) []string {
	var out []string
	for p := range strings.SplitSeq(raw, ",") {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
