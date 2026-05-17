package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/hibiken/asynq"

	"github.com/go-chi/chi/v5"

	"github.com/mayloo89/circl/backend/internal/admin"
	"github.com/mayloo89/circl/backend/internal/appeals"
	"github.com/mayloo89/circl/backend/internal/auth"
	"github.com/mayloo89/circl/backend/internal/chat"
	"github.com/mayloo89/circl/backend/internal/config"
	"github.com/mayloo89/circl/backend/internal/contacts"
	"github.com/mayloo89/circl/backend/internal/db"
	"github.com/mayloo89/circl/backend/internal/email"
	"github.com/mayloo89/circl/backend/internal/exports"
	"github.com/mayloo89/circl/backend/internal/logger"
	"github.com/mayloo89/circl/backend/internal/metrics"
	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/notifications"
	"github.com/mayloo89/circl/backend/internal/presence"
	"github.com/mayloo89/circl/backend/internal/profiles"
	"github.com/mayloo89/circl/backend/internal/push"
	"github.com/mayloo89/circl/backend/internal/ratelimit"
	"github.com/mayloo89/circl/backend/internal/reports"
	"github.com/mayloo89/circl/backend/internal/server"
	"github.com/mayloo89/circl/backend/internal/storage"
	"github.com/mayloo89/circl/backend/internal/token"
	"github.com/mayloo89/circl/backend/internal/tracing"
	"github.com/mayloo89/circl/backend/internal/uploads"
	"github.com/mayloo89/circl/backend/internal/worker"
	"github.com/mayloo89/circl/backend/internal/wsticket"
)

const tokenExpiry = 15 * time.Minute

func main() {
	// Load .env first so every subsequent os.Getenv call (LOKI_URL, LOG_LEVEL, etc.) sees it.
	dotenvErr := godotenv.Load()

	env := config.EnvOrDefault("ENV", "development")
	log, flushLogs := logger.New(env, config.EnvOrDefault("LOG_LEVEL", "info"))
	if dotenvErr != nil {
		log.Debug().Msg("no .env file found, using system environment")
	}

	// appCtx is cancelled when the process receives SIGINT or SIGTERM.
	// All long-running goroutines (hub, workers, cleaners) use this context
	// so they stop cleanly when the application shuts down.
	appCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	port := config.EnvOrDefault("PORT", "8080")
	corsOrigins := server.NormalizeCORSOrigins(config.EnvOrDefault("CORS_ALLOWED_ORIGINS", "http://localhost:3000"))

	databaseURL, err := config.RequireEnv("DATABASE_URL")
	if err != nil {
		log.Fatal().Err(err).Msg("missing required env var")
	}

	jwtSecret, err := config.RequireEnv("JWT_SECRET")
	if err != nil {
		log.Fatal().Err(err).Msg("missing required env var")
	}

	redisURL, err := config.RequireEnv("REDIS_URL")
	if err != nil {
		log.Fatal().Err(err).Msg("missing required env var")
	}

	// Initialise OpenTelemetry tracing. When OTEL_EXPORTER_OTLP_ENDPOINT is
	// unset a no-op exporter is used so the app starts without a collector.
	tracerShutdown, err := tracing.Init(appCtx, log, "circl-api", config.EnvOrDefault("BUILD_VERSION", "dev"), env)
	if err != nil {
		log.Fatal().Err(err).Msg("tracing init failed")
	}

	if err := db.Migrate("migrations", databaseURL); err != nil {
		log.Fatal().Err(err).Msg("migrations failed")
	}
	log.Info().Msg("migrations applied")

	initCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.Open(initCtx, databaseURL, func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = tracing.NewPgxTracer()
	})
	if err != nil {
		log.Fatal().Err(err).Msg("database connection failed")
	}
	defer pool.Close()
	log.Info().Msg("database connected")

	frontendURL := config.EnvOrDefault("FRONTEND_URL", "http://localhost:3000")

	var mailer email.Sender
	switch config.EnvOrDefault("EMAIL_PROVIDER", "console") {
	case "smtp":
		smtpPort := config.EnvIntOrDefault("SMTP_PORT", 1025)
		mailer = email.NewSMTPSender(email.SMTPConfig{
			Host:     config.EnvOrDefault("SMTP_HOST", "localhost"),
			Port:     smtpPort,
			Username: config.EnvOrDefault("SMTP_USER", ""),
			Password: config.EnvOrDefault("SMTP_PASS", ""),
			From:     config.EnvOrDefault("SMTP_FROM", "noreply@circl.app"),
		})
		log.Info().Str("provider", "smtp").Str("host", config.EnvOrDefault("SMTP_HOST", "localhost")).Int("port", smtpPort).Msg("email provider")
	default:
		mailer = email.NewConsoleSender()
		log.Info().Str("provider", "console").Msg("email provider")
	}

	authStore := auth.NewStore(pool)
	ageAuditStore := auth.NewAgeAuditStore(pool)
	authSvc := auth.NewService(authStore, mailer, frontendURL)

	profileStore := profiles.NewStore(pool)
	profileSvc := profiles.NewService(profileStore)
	profileHandler := profiles.NewHandler(profileSvc)

	hub := notifications.NewHub()
	notificationsHandler := notifications.NewHandler(hub, jwtSecret)

	contactStore := contacts.NewStore(pool)
	contactSvc := contacts.NewService(contactStore)

	reportStore := reports.NewStore(pool)
	reportSvc := reports.NewService(reportStore)

	redisOpt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatal().Err(err).Msg("invalid Redis URL")
	}
	rdb := redis.NewClient(redisOpt)
	defer rdb.Close()

	if err := rdb.Ping(appCtx).Err(); err != nil {
		log.Fatal().Err(err).Msg("redis connection failed")
	}
	log.Info().Msg("redis connected")

	limiter := ratelimit.NewRedisLimiter(rdb)

	adminStore := admin.NewStore(pool)
	adminSvc := admin.NewService(adminStore)

	loginIPLimit := config.EnvIntOrDefault("LOGIN_IP_LIMIT", 20)
	registerIPLimit := config.EnvIntOrDefault("REGISTER_IP_LIMIT", 10)
	refreshStore := auth.NewRedisRefreshStore(rdb)

	accountHandler := auth.NewAccountHandler(authSvc,
		auth.WithAccountRefreshStore(refreshStore),
	)

	authHandler := auth.NewHandler(authSvc, jwtSecret, tokenExpiry,
		auth.WithLocker(limiter),
		auth.WithLimiter(limiter),
		auth.WithLoginIPLimit(loginIPLimit, 15*time.Minute),
		auth.WithRegisterIPLimit(registerIPLimit, time.Hour),
		auth.WithEmailFlow(authSvc, frontendURL),
		auth.WithProfileStore(profileStore),
		auth.WithRefreshTokenStore(refreshStore),
		auth.WithAgeAuditStore(ageAuditStore),
	)

	// Appeals: separate sub-service for the suspension-appeal flow. The
	// reactivator is the admin service so an approved appeal flips the user
	// back to 'active' without admin needing to act twice.
	appealStore := appeals.NewStore(pool)
	appealSvc := appeals.NewService(appealStore, adminSvc)
	appealsPublicHandler := appeals.NewPublicHandler(appealSvc)
	appealsAdminHandler := appeals.NewAdminHandler(appealSvc)

	// Whenever admin suspends or bans a user, mint an appeal token, store it,
	// and email the user the appeal link. Errors are logged but never block
	// the moderation action — admin already saw it succeed.
	adminSvc.SetSuspensionNotifier(suspensionNotifier{
		svc:         appealSvc,
		mailer:      mailer,
		frontendURL: frontendURL,
		log:         log,
	})

	reportMgr := reports.NewManager(reportSvc,
		reports.WithModerator(adminSvc),
		reports.WithLimiter(limiter),
		reports.WithLogger(log),
	)
	reportsHandler := reports.NewHandler(reportMgr)

	pushStore := push.NewStore(pool)
	pushSvc := push.NewService(pushStore,
		config.EnvOrDefault("VAPID_PUBLIC_KEY", ""),
		config.EnvOrDefault("VAPID_PRIVATE_KEY", ""),
		config.EnvOrDefault("VAPID_SUBJECT", "mailto:admin@circl.app"),
		log,
	)
	pushHandler := push.NewHandler(pushSvc)
	if pushSvc.Enabled() {
		log.Info().Msg("web push notifications enabled")
	}

	// notifyUser sends an SSE event and a web push notification (if enabled
	// and the recipient hasn't opted out of the relevant notification
	// category). The category gate `wants` runs against the recipient's
	// stored notification preferences; pass nil to send unconditionally.
	// SSE is never gated — it powers in-app badges and live UI updates.
	notifyUser := func(userID string, e notifications.Event, n push.Notification, wants func(profiles.NotificationFlags) bool) {
		hub.Notify(userID, e)
		if !pushSvc.Enabled() {
			return
		}
		if wants != nil {
			flags, err := profileSvc.GetNotificationFlags(appCtx, userID)
			if err != nil || !wants(flags) {
				return
			}
		}
		go pushSvc.Send(appCtx, userID, n)
	}

	// contactPushNotifier implements notifications.Notifier and enriches contact
	// events with web push notifications for offline users.
	contactPushNotifier := &contactNotifier{notifyFn: notifyUser}
	contactsHandler := contacts.NewHandler(contactSvc,
		contacts.WithNotifier(contactPushNotifier),
		contacts.WithLimiter(limiter),
	)

	chatHub := chat.NewHub(rdb)
	go chatHub.Run(appCtx)

	presenceStore := presence.NewStore(rdb, pool)
	presenceHandler := presence.NewHandler(presenceStore, hub, presencePrivacy{svc: profileSvc})

	// Storage provider: LocalStorage for dev, S3Storage for production.
	storageProvider := config.EnvOrDefault("STORAGE_PROVIDER", "local")
	var fileStorage storage.Storage
	var localStorageHandler http.Handler

	switch storageProvider {
	case "local":
		baseURL := config.EnvOrDefault("LOCAL_STORAGE_BASE_URL", fmt.Sprintf("http://localhost:%s/uploads/files", port))
		ls := storage.NewLocalStorage("./data/uploads", baseURL)
		fileStorage = ls
		localStorageHandler = storage.NewLocalHandler(ls)
		log.Info().Str("provider", "local").Str("base_url", baseURL).Msg("storage provider")
	case "s3":
		endpoint := config.EnvOrDefault("S3_ENDPOINT", "localhost:9000")
		useSSL := !strings.HasPrefix(endpoint, "http://")
		endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://")
		bucket := config.EnvOrDefault("S3_BUCKET", "circl-media")
		accessKey := config.EnvOrDefault("S3_ACCESS_KEY", "")
		secretKey := config.EnvOrDefault("S3_SECRET_KEY", "")
		publicURL := config.EnvOrDefault("S3_PUBLIC_URL", fmt.Sprintf("http://%s/%s", endpoint, bucket))

		s3store, err := storage.NewS3Storage(storage.S3Config{
			Endpoint:  endpoint,
			AccessKey: accessKey,
			SecretKey: secretKey,
			Bucket:    bucket,
			PublicURL: publicURL,
			UseSSL:    useSSL,
		})
		if err != nil {
			log.Fatal().Err(err).Msg("S3 storage init failed")
		}
		fileStorage = s3store
		log.Info().Str("provider", "s3").Str("endpoint", endpoint).Str("bucket", bucket).Msg("storage provider")
	default:
		log.Fatal().Str("provider", storageProvider).Msg("unknown storage provider")
	}

	chatStore := chat.NewStore(pool, fileStorage.PublicURL)
	chatSvc := chat.NewService(chatStore)

	isBlockedInRoom := func(ctx context.Context, senderID, roomID string) bool {
		members, err := chatSvc.ListMembers(ctx, roomID)
		if err != nil {
			return false
		}
		// Filter out the sender from the members list
		otherMembers := make([]string, 0, max(0, len(members)-1))
		for _, uid := range members {
			if uid != senderID {
				otherMembers = append(otherMembers, uid)
			}
		}
		// Batch check if sender is blocked by any member in one query
		blocked, err := contactSvc.IsBlockedInRoom(ctx, senderID, otherMembers)
		if err != nil {
			return false
		}
		return blocked
	}

	chatPrivacy := func(ctx context.Context, userID string) (chat.PrivacyFlags, error) {
		flags, err := profileSvc.GetPrivacyFlags(ctx, userID)
		if err != nil {
			return chat.PrivacyFlags{}, err
		}
		return chat.PrivacyFlags{
			HideReadReceipts: flags.HideReadReceipts,
			HideTyping:       flags.HideTypingIndicator,
		}, nil
	}

	wsTicketStore := wsticket.NewStore(rdb)
	chatWSHandler := chat.NewWSHandler(chatSvc, chatHub, wsTicketStore, func(recipientID, roomID string) {
		notifyUser(recipientID, notifications.Event{
			Type:    "new_message",
			Payload: map[string]string{"room_id": roomID},
		}, push.Notification{
			Title: "New message",
			Body:  "You have a new message",
			URL:   "/chat/" + roomID,
		}, func(f profiles.NotificationFlags) bool { return f.ChatMessages })
	}, chat.HandlerConfig{
		IsBlockedInRoom: isBlockedInRoom,
		AllowedOrigins:  corsOrigins,
		PrivacyResolver: chatPrivacy,
	})

	notifyDeleted := func(roomID, messageID string) {
		data, _ := json.Marshal(map[string]string{
			"event":   "message_deleted",
			"id":      messageID,
			"room_id": roomID,
		})
		chatHub.Publish(appCtx, roomID, data) //nolint:errcheck
	}
	chatHandler := chat.NewHandler(chatSvc, chat.HandlerConfig{
		Hub:                  chatHub,
		NotifyMessageDeleted: notifyDeleted,
		NotifyRoomRead: func(roomID, userID string, readAt time.Time) {
			// Emit gate: drop the broadcast entirely when the reader has
			// opted out of read receipts. The hub's deliver loop also
			// applies the symmetric receive gate per recipient.
			if flags, err := profileSvc.GetPrivacyFlags(appCtx, userID); err == nil && flags.HideReadReceipts {
				return
			}
			data, _ := json.Marshal(map[string]any{
				"event":   "read_receipt",
				"room_id": roomID,
				"user_id": userID,
				"read_at": readAt,
			})
			chatHub.Publish(appCtx, roomID, data) //nolint:errcheck
		},
		DeleteFiles: func(ctx context.Context, keys []string) {
			for _, key := range keys {
				if err := fileStorage.Delete(ctx, key); err != nil {
					log.Error().Err(err).Str("storage_key", key).Msg("chat: delete file failed")
				}
			}
		},
		ReadFile:  fileStorage.GetObject,
		IsBlocked: contactSvc.IsBlocked,
	})

	uploadStore := uploads.NewStore(pool)
	uploadSvc := uploads.NewService(uploadStore, fileStorage, log)

	redisConnOpt := asynq.RedisClientOpt{
		Addr:     redisOpt.Addr,
		Password: redisOpt.Password,
		DB:       redisOpt.DB,
	}
	workerClient := worker.NewClient(redisConnOpt)
	defer workerClient.Close() //nolint:errcheck
	uploadSvc.SetEnqueuer(func(ctx context.Context, uploadID, storageKey, contentType string) error {
		return worker.EnqueueProcessImage(ctx, workerClient, worker.ImageProcessPayload{
			UploadID:    uploadID,
			StorageKey:  storageKey,
			ContentType: contentType,
		})
	})

	imageProcessor := worker.NewImageProcessor(fileStorage, uploadStore, config.EnvIntOrDefault("IMAGE_MAX_PX", 0), log)

	// Data export (Habeas Data / GDPR Art. 20) — request → asynq build → email.
	apiPublicURL := config.EnvOrDefault("API_PUBLIC_URL", "http://localhost:"+port)
	exportStore := exports.NewStore(pool)
	exportSource := exports.NewSource(pool)
	exportSvc := exports.NewService(exports.Config{
		Store:           exportStore,
		Source:          exportSource,
		Storage:         fileStorage,
		Mailer:          exportMailer{sender: mailer},
		Enqueue: func(ctx context.Context, requestID, userID string) error {
			return worker.EnqueueExportUser(ctx, workerClient, worker.ExportPayload{
				RequestID: requestID,
				UserID:    userID,
			})
		},
		StoragePrefix:   "exports/",
		DownloadURLBase: apiPublicURL + "/account/export",
		Log:             log,
	})
	exportHandler := worker.NewExportHandler(exportSvc, log)
	exportAuthedHandler := exports.NewAuthedHandler(exportSvc)
	exportDownloadHandler := exports.NewDownloadHandler(exportSvc)

	workerServer := worker.NewServer(redisConnOpt, 4, log)
	if err := workerServer.Start(imageProcessor, exportHandler); err != nil {
		log.Fatal().Err(err).Msg("worker server failed to start")
	}
	defer workerServer.Shutdown()

	ephemeralCleaner := worker.NewEphemeralCleaner(chatStore, fileStorage, func(roomID, messageID string) {
		data, _ := json.Marshal(map[string]string{
			"event":   "message_deleted",
			"id":      messageID,
			"room_id": roomID,
		})
		chatHub.Publish(appCtx, roomID, data) //nolint:errcheck
	}, log)
	ephemeralCleaner.Start(appCtx)

	// Daily purge of accounts past the 30-day deletion grace period.
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		worker.PurgeDeletedAccounts(appCtx, log, authStore, fileStorage)
		for range ticker.C {
			worker.PurgeDeletedAccounts(appCtx, log, authStore, fileStorage)
		}
	}()

	uploadHandler := uploads.NewHandler(uploadSvc)
	adminPresenceLookup := func(ctx context.Context, ids []string) (map[string]admin.UserPresence, error) {
		info, err := presenceStore.GetPresence(ctx, ids)
		if err != nil {
			return nil, err
		}
		out := make(map[string]admin.UserPresence, len(info))
		for _, i := range info {
			out[i.UserID] = admin.UserPresence{Online: i.Online, LastSeenAt: i.LastSeenAt}
		}
		return out, nil
	}
	adminHandler := admin.NewHandler(adminSvc, adminPresenceLookup, admin.WithAppealsHandler(appealsAdminHandler))

	requireAuth := middleware.RequireAuth(jwtSecret, adminSvc)

	var testHandler http.Handler
	if config.EnvOrDefault("TEST_ENDPOINTS_ENABLED", "false") == "true" {
		log.Warn().Msg("test endpoints enabled — do not use in production")
		testHandler = newTestHandler(pool, authSvc, profileStore, jwtSecret, tokenExpiry)
	}

	m := metrics.New(pool)
	m.RegisterWSHub(chatHub)

	h := server.New(server.Config{
		DB:          pool,
		RedisPing:   func(ctx context.Context) error { return rdb.Ping(ctx).Err() },
		Log:         log,
		Env:         env,
		Version:     config.EnvOrDefault("BUILD_VERSION", "dev"),
		CORSOrigins: corsOrigins,

		TracingMiddleware: tracing.HTTPMiddleware("circl-api"),
		MetricsHandler:    m.Handler(config.EnvOrDefault("METRICS_TOKEN", "")),
		MetricsMiddleware: m.Middleware(),

		RequireAuth: requireAuth,

		Auth:          authHandler,
		WSTicket:      wsticket.NewHandler(wsTicketStore),
		Account:       accountHandler,
		Profile:       profileHandler,
		Available:     profiles.PublicAvailableHandler(profileSvc),
		Contacts:      contactsHandler,
		Notifications: notificationsHandler,
		Chat:          chatHandler,
		ChatWS:        chatWSHandler,
		Presence:      presenceHandler,
		Upload:        uploadHandler,
		Reports:       reportsHandler,
		Push:          pushHandler,
		Admin:          adminHandler,
		Appeals:        appealsPublicHandler,
		Export:         exportAuthedHandler,
		ExportDownload: exportDownloadHandler,
		LocalStorage:   localStorageHandler,
		Test:           testHandler,
	})

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      h,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Info().Str("port", port).Str("env", env).Msg("server starting")

	srvErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			srvErr <- err
		}
	}()

	select {
	case err := <-srvErr:
		log.Fatal().Err(err).Msg("server error")
	case <-appCtx.Done():
	}

	stop() // release signal watcher resources

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	log.Info().Msg("shutting down")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("server shutdown error")
	}
	if err := tracerShutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("tracer shutdown error")
	}
	flushLogs()
	log.Info().Msg("server stopped")
}

// newTestHandler returns a handler for test-only endpoints.
// It must only be mounted when TEST_ENDPOINTS_ENABLED=true.
//
// POST /test/users — creates a verified user, seeds the profile, returns token + id + email + password.
func newTestHandler(pool *pgxpool.Pool, authSvc *auth.Service, profileStore profiles.Store, jwtSecret string, tokenExpiry time.Duration) http.Handler {
	mux := chi.NewRouter()
	mux.Post("/users", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
			Username string `json:"username"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.Username == "" {
			local, _, _ := strings.Cut(req.Email, "@")
			req.Username = "u_" + strings.ReplaceAll(local, ".", "_")
		}

		user, err := authSvc.Register(r.Context(), auth.RegistrationInput{
			Email:                 req.Email,
			Password:              req.Password,
			AcceptedPolicyVersion: auth.CurrentPolicyVersion,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}) //nolint:errcheck
			return
		}

		// Mark email as verified directly in the database.
		pool.Exec(r.Context(), `UPDATE users SET email_verified_at = now() WHERE id = $1`, user.ID) //nolint:errcheck,exhaustruct

		// Seed profile — mark as onboarded so tests skip the wizard.
		dob := time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)
		now := time.Now()
		profileStore.Upsert(r.Context(), user.ID, profiles.ProfileInput{ //nolint:errcheck
			Username:    req.Username,
			DisplayName: req.Username,
			DateOfBirth: &dob,
			OnboardedAt: &now,
		})

		tok, err := token.Generate(user.ID, user.Role, jwtSecret, tokenExpiry)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
			"id":       user.ID,
			"email":    user.Email,
			"password": req.Password,
			"token":    tok,
		})
	})
	return mux
}

// contactNotifier implements notifications.Notifier and enriches contact events
// with web push notifications for offline users. Pushes are gated on the
// recipient's NotifyContactRequests preference; SSE events are always sent.
type contactNotifier struct {
	notifyFn func(userID string, e notifications.Event, n push.Notification, wants func(profiles.NotificationFlags) bool)
}

func (c *contactNotifier) Notify(userID string, e notifications.Event) {
	var n push.Notification
	switch e.Type {
	case "contact_request":
		n = push.Notification{Title: "New contact request", URL: "/contacts"}
	case "contact_accepted":
		n = push.Notification{Title: "Contact request accepted", URL: "/contacts"}
	case "contact_removed":
		n = push.Notification{Title: "Contact removed", URL: "/contacts"}
	}
	c.notifyFn(userID, e, n, func(f profiles.NotificationFlags) bool { return f.ContactRequests })
}

// presencePrivacy adapts *profiles.Service to presence.PrivacyLookup so the
// presence handler can gate visibility on the symmetric hide_presence flag
// without taking a dependency on the profiles package.
type presencePrivacy struct {
	svc *profiles.Service
}

func (p presencePrivacy) HidePresenceByIDs(ctx context.Context, userIDs []string) (map[string]bool, error) {
	flags, err := p.svc.GetPrivacyFlagsByIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(flags))
	for id, f := range flags {
		if f.HidePresence {
			out[id] = true
		}
	}
	return out, nil
}

// suspensionNotifier implements admin.SuspensionNotifier: it mints an appeal
// token via the appeals service and emails the user the appeal link. Errors
// are logged — never surfaced — because the suspension itself already
// succeeded and admin should not have to retry on a flaky downstream.
type suspensionNotifier struct {
	svc         *appeals.Service
	mailer      email.Sender
	frontendURL string
	log         zerolog.Logger
}

// exportMailer adapts the in-tree email.Sender to exports.Mailer without
// pulling the email package into the exports tests.
type exportMailer struct {
	sender email.Sender
}

func (m exportMailer) SendReadyEmail(ctx context.Context, to, downloadURL string, expiresAt time.Time) error {
	return m.sender.Send(ctx, email.ExportReadyMessage(to, downloadURL, expiresAt))
}

func (m exportMailer) SendFailedEmail(ctx context.Context, to string) error {
	return m.sender.Send(ctx, email.ExportFailedMessage(to))
}

func (n suspensionNotifier) NotifyOfSuspension(ctx context.Context, user admin.UserRecord, susp admin.Suspension) {
	bgCtx := context.WithoutCancel(ctx)
	logger := n.log.With().Str("component", "appeals").Str("user_id", user.ID).Str("suspension_id", susp.ID).Logger()
	go func() {
		plainToken, _, err := n.svc.CreateForSuspension(bgCtx, user.ID, susp.ID)
		if err != nil {
			logger.Error().Err(err).Msg("appeals: create token failed")
			return
		}
		appealURL := n.frontendURL + "/appeal/" + plainToken
		msg := email.AppealMessage(user.Email, appealURL, susp.Reason, susp.SuspendedUntil == nil)
		if err := n.mailer.Send(bgCtx, msg); err != nil {
			logger.Warn().Err(err).Msg("appeals: send appeal email failed")
		}
	}()
}
