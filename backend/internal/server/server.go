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
	Version     string   // reported in /health; defaults to "dev"
	CORSOrigins []string

	// Observability — all are optional (nil disables)
	// TracingMiddleware is inserted before RequestLogger so trace_id/span_id
	// are available to the logger for log-trace correlation.
	TracingMiddleware func(http.Handler) http.Handler
	MetricsHandler    http.Handler               // mounted at GET /metrics
	MetricsMiddleware func(http.Handler) http.Handler

	// Security
	RequireAuth func(http.Handler) http.Handler

	// Sub-routers / handlers
	Auth          http.Handler
	WSTicket      http.Handler
	Account       http.Handler
	Profile       http.Handler
	Available     http.Handler
	Contacts      http.Handler
	Notifications http.Handler
	Chat          http.Handler
	ChatWS        http.Handler
	Presence      http.Handler
	Upload        http.Handler
	Reports       http.Handler
	Push          http.Handler
	Admin         http.Handler
	LocalStorage  http.Handler // nil in production
	Test          http.Handler // nil unless TEST_ENDPOINTS_ENABLED
}

// New returns a configured chi router with all application routes registered.
func New(cfg Config) http.Handler {
	if cfg.Version == "" {
		cfg.Version = "dev"
	}

	r := chi.NewRouter()

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

	r.Mount("/auth", cfg.Auth)
	r.Handle("/profiles/available", cfg.Available)

	// SSE stream — auth is handled inside the handler via ?token= query param
	// because the browser EventSource API does not support custom headers.
	r.Handle("/notifications/stream", cfg.Notifications)

	// WebSocket endpoint — auth via single-use ?ticket= (from POST /ws-ticket).
	r.Handle("/chat/rooms/{id}/ws", cfg.ChatWS)

	// Protected routes — RequireAuth validates the Bearer JWT before forwarding.
	r.Group(func(g chi.Router) {
		g.Use(cfg.RequireAuth)
		g.Handle("/ws-ticket", cfg.WSTicket)
		g.Mount("/users/me", cfg.Account)
		g.Mount("/profiles", cfg.Profile)
		g.Mount("/", cfg.Contacts)
		g.Mount("/chat", cfg.Chat)
		g.Mount("/presence", cfg.Presence)
		g.Mount("/uploads", cfg.Upload)
		g.Mount("/reports", cfg.Reports)
		g.Mount("/push", cfg.Push)
		g.Mount("/admin", cfg.Admin)
	})

	// Local file serving — only mounted when LocalStorage is not nil
	// (i.e. when STORAGE_PROVIDER=local for development).
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
