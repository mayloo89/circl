package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/middleware"
)

// DBPinger is the minimal interface required by the health handler.
// *pgxpool.Pool satisfies this interface.
type DBPinger interface {
	Ping(ctx context.Context) error
}

// Config holds all dependencies needed to construct the HTTP server.
type Config struct {
	// Core
	DB          DBPinger
	RedisPing   func(context.Context) error // nil = skip Redis health check
	Log         zerolog.Logger
	Env         string
	Version     string // reported in /health; defaults to "dev"
	CORSOrigins []string

	// Observability — all are optional (nil disables)
	// RealIPMiddleware is inserted first so every subsequent middleware and
	// handler sees the true client IP via middleware.ClientIP(r).
	RealIPMiddleware func(http.Handler) http.Handler
	// TracingMiddleware is inserted before RequestLogger so trace_id/span_id
	// are available to the logger for log-trace correlation.
	TracingMiddleware func(http.Handler) http.Handler
	MetricsHandler    http.Handler // mounted at GET /metrics
	MetricsMiddleware func(http.Handler) http.Handler

	// Security
	RequireAuth func(http.Handler) http.Handler

	// Sub-routers / handlers
	Auth          http.Handler
	WSTicket      http.Handler
	GuestWSTicket http.Handler
	Guest         http.Handler
	Account       http.Handler
	Profile       http.Handler
	Available     http.Handler
	Contacts      http.Handler
	Notifications http.Handler
	Chat          http.Handler
	ChatWS        http.Handler
	Presence      http.Handler
	Upload        http.Handler
	Albums        http.Handler
	Reports       http.Handler
	Push          http.Handler
	Admin         http.Handler
	Appeals       http.Handler // public /appeal/{token}; nil disables
	// Data exports (Habeas Data / GDPR Art. 20). Export mounts at
	// /users/me/exports behind RequireAuth; ExportDownload mounts at
	// /account/export and is un-authenticated by design — the path token is
	// the credential, and the user might have lost their session by the time
	// the ready email lands.
	Export         http.Handler
	ExportDownload http.Handler
	LocalStorage   http.Handler // nil in production
	Test           http.Handler // nil unless TEST_ENDPOINTS_ENABLED
}

// New returns a configured chi router with all application routes registered.
func New(cfg Config) http.Handler {
	if cfg.Version == "" {
		cfg.Version = "dev"
	}

	r := chi.NewRouter()

	if cfg.RealIPMiddleware != nil {
		r.Use(cfg.RealIPMiddleware)
	}
	if cfg.TracingMiddleware != nil {
		r.Use(cfg.TracingMiddleware)
	}
	r.Use(middleware.RequestLogger(cfg.Log))
	r.Use(middleware.SecurityHeaders(cfg.Env))
	if cfg.MetricsMiddleware != nil {
		r.Use(cfg.MetricsMiddleware)
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", healthHandler(cfg.DB, cfg.RedisPing, cfg.Env, cfg.Version))
	if cfg.MetricsHandler != nil {
		r.Get("/metrics", cfg.MetricsHandler.ServeHTTP)
	}

	// JSON API routes — body size is capped to prevent memory exhaustion.
	// The /uploads/files local-storage path is excluded because it handles
	// binary file uploads that apply their own per-category size limits.
	r.Group(func(api chi.Router) {
		api.Use(middleware.LimitRequestBody)

		api.Mount("/auth", cfg.Auth)
		api.Handle("/profiles/available", cfg.Available)

		// /appeal/{token} is intentionally un-authenticated — the locked-out user
		// it serves cannot log in. Auth is provided by the single-use token.
		if cfg.Appeals != nil {
			api.Mount("/appeal", cfg.Appeals)
		}

		// /account/export/{token} is intentionally un-authenticated — the user
		// downloads with the single-use token from their ready email.
		if cfg.ExportDownload != nil {
			api.Mount("/account/export", cfg.ExportDownload)
		}

		// Guest-facing routes — unauthenticated. The guest session endpoint
		// creates an ephemeral Redis-backed identity; the WS ticket endpoint
		// validates the session internally.
		if cfg.Guest != nil {
			api.Mount("/guest", cfg.Guest)
		}
		if cfg.GuestWSTicket != nil {
			api.Handle("/guest/ws-ticket", cfg.GuestWSTicket)
		}

		// SSE stream — auth is handled inside the handler via ?token= query param
		// because the browser EventSource API does not support custom headers.
		api.Handle("/notifications/stream", cfg.Notifications)

		// WebSocket endpoint — auth via single-use ?ticket= (from POST /ws-ticket).
		api.Handle("/chat/rooms/{id}/ws", cfg.ChatWS)

		// Protected routes — RequireAuth validates the Bearer JWT before forwarding.
		api.Group(func(g chi.Router) {
			g.Use(cfg.RequireAuth)
			g.Handle("/ws-ticket", cfg.WSTicket)
			g.Mount("/users/me", cfg.Account)
			g.Mount("/profiles", cfg.Profile)
			g.Mount("/", cfg.Contacts)
			g.Mount("/chat", cfg.Chat)
			g.Mount("/presence", cfg.Presence)
			g.Mount("/uploads", cfg.Upload)
			if cfg.Albums != nil {
				g.Mount("/albums", cfg.Albums)
			}
			g.Mount("/reports", cfg.Reports)
			g.Mount("/push", cfg.Push)
			g.Mount("/admin", cfg.Admin)
			if cfg.Export != nil {
				g.Mount("/users/me/exports", cfg.Export)
			}
		})
	})

	// Local file serving — only mounted when LocalStorage is not nil
	// (i.e. when STORAGE_PROVIDER=local for development). Not wrapped in
	// LimitRequestBody because it handles binary uploads with per-category limits.
	if cfg.LocalStorage != nil {
		r.Mount("/uploads/files", cfg.LocalStorage)
	}

	// Test endpoints — only mounted when TEST_ENDPOINTS_ENABLED=true.
	// Never set this in production.
	if cfg.Test != nil {
		r.Mount("/test", cfg.Test)
	}

	return r
}

// healthHandler reports server, database, and Redis status.
func healthHandler(db DBPinger, redisPing func(context.Context) error, env, version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		dbStatus := "ok"
		if err := db.Ping(ctx); err != nil {
			zerolog.Ctx(r.Context()).Error().Err(err).Msg("db health check failed")
			dbStatus = "error"
		}

		redisStatus := "ok"
		if redisPing != nil {
			if err := redisPing(ctx); err != nil {
				zerolog.Ctx(r.Context()).Error().Err(err).Msg("redis health check failed")
				redisStatus = "error"
			}
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","env":"%s","version":"%s","db":"%s","redis":"%s"}`,
			env, version, dbStatus, redisStatus)
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
