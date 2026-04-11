package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	"github.com/hibiken/asynq"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mayloo89/circl/backend/internal/admin"
	"github.com/mayloo89/circl/backend/internal/auth"
	"github.com/mayloo89/circl/backend/internal/email"
	"github.com/mayloo89/circl/backend/internal/chat"
	"github.com/mayloo89/circl/backend/internal/config"
	"github.com/mayloo89/circl/backend/internal/contacts"
	"github.com/mayloo89/circl/backend/internal/db"
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
	"github.com/mayloo89/circl/backend/internal/uploads"
	"github.com/mayloo89/circl/backend/internal/worker"
)

const tokenExpiry = 24 * time.Hour

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment")
	}

	port := config.EnvOrDefault("PORT", "8080")
	env := config.EnvOrDefault("ENV", "development")
	corsOrigins := server.NormalizeCORSOrigins(config.EnvOrDefault("CORS_ALLOWED_ORIGINS", "http://localhost:3000"))

	databaseURL, err := config.RequireEnv("DATABASE_URL")
	if err != nil {
		log.Fatalf("%v", err)
	}

	jwtSecret, err := config.RequireEnv("JWT_SECRET")
	if err != nil {
		log.Fatalf("%v", err)
	}

	redisURL, err := config.RequireEnv("REDIS_URL")
	if err != nil {
		log.Fatalf("%v", err)
	}

	if err := db.Migrate("migrations", databaseURL); err != nil {
		log.Fatalf("Migrations failed: %v", err)
	}
	log.Println("Migrations applied successfully")

	initCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.Open(initCtx, databaseURL)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer pool.Close()
	log.Println("Database connection established")

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
		log.Printf("Email provider: SMTP (%s:%d)", config.EnvOrDefault("SMTP_HOST", "localhost"), smtpPort)
	default:
		mailer = email.NewConsoleSender()
		log.Println("Email provider: console (stdout)")
	}

	authStore := auth.NewStore(pool)
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
		log.Fatalf("Invalid Redis URL: %v", err)
	}
	rdb := redis.NewClient(redisOpt)
	defer rdb.Close()

	appCtx := context.Background()
	if err := rdb.Ping(appCtx).Err(); err != nil {
		log.Fatalf("Redis connection failed: %v", err)
	}
	log.Println("Redis connection established")

	limiter := ratelimit.NewRedisLimiter(rdb)

	adminStore := admin.NewStore(pool)
	adminSvc := admin.NewService(adminStore)

	loginIPLimit := config.EnvIntOrDefault("LOGIN_IP_LIMIT", 20)
	registerIPLimit := config.EnvIntOrDefault("REGISTER_IP_LIMIT", 10)
	accountHandler := auth.NewAccountHandler(authSvc)

	authHandler := auth.NewHandler(authSvc, jwtSecret, tokenExpiry,
		auth.WithLocker(limiter),
		auth.WithLimiter(limiter),
		auth.WithLoginIPLimit(loginIPLimit, 15*time.Minute),
		auth.WithRegisterIPLimit(registerIPLimit, time.Hour),
		auth.WithEmailFlow(authSvc, frontendURL),
		auth.WithProfileStore(profileStore),
	)

	reportMgr := reports.NewManager(reportSvc,
		reports.WithModerator(adminSvc),
		reports.WithLimiter(limiter),
	)
	reportsHandler := reports.NewHandler(reportMgr)

	pushStore := push.NewStore(pool)
	pushSvc := push.NewService(pushStore,
		config.EnvOrDefault("VAPID_PUBLIC_KEY", ""),
		config.EnvOrDefault("VAPID_PRIVATE_KEY", ""),
		config.EnvOrDefault("VAPID_SUBJECT", "mailto:admin@circl.app"),
	)
	pushHandler := push.NewHandler(pushSvc)
	if pushSvc.Enabled() {
		log.Println("Web push notifications enabled")
	}

	// notifyUser sends an SSE event and a web push notification (if enabled).
	// Push is always attempted so users receive notifications regardless of
	// whether the app is currently open — the service worker handles dedup.
	notifyUser := func(userID string, e notifications.Event, n push.Notification) {
		hub.Notify(userID, e)
		if pushSvc.Enabled() {
			go pushSvc.Send(appCtx, userID, n)
		}
	}

	// contactPushNotifier implements notifications.Notifier and enriches contact
	// events with web push notifications for offline users.
	contactPushNotifier := &contactNotifier{notifyFn: notifyUser}
	contactsHandler := contacts.NewHandler(contactSvc, contacts.WithNotifier(contactPushNotifier))

	chatHub := chat.NewHub(rdb)
	go chatHub.Run(appCtx)

	presenceStore := presence.NewStore(rdb, pool)
	presenceHandler := presence.NewHandler(presenceStore, hub)

	// Storage provider: LocalStorage for dev, S3Storage for production.
	storageProvider := config.EnvOrDefault("STORAGE_PROVIDER", "local")
	var fileStorage storage.Storage
	var localStorageHandler http.Handler

	switch storageProvider {
	case "local":
		ls := storage.NewLocalStorage("./data/uploads", fmt.Sprintf("http://localhost:%s/uploads/files", port))
		fileStorage = ls
		localStorageHandler = storage.NewLocalHandler(ls)
		log.Println("Storage provider: local (./data/uploads)")
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
			log.Fatalf("S3 storage init failed: %v", err)
		}
		fileStorage = s3store
		log.Printf("Storage provider: S3-compatible (%s, bucket: %s)", endpoint, bucket)
	default:
		log.Fatalf("Unknown storage provider: %s", storageProvider)
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

	chatWSHandler := chat.NewWSHandler(chatSvc, chatHub, jwtSecret, func(recipientID, roomID string) {
		notifyUser(recipientID, notifications.Event{
			Type:    "new_message",
			Payload: map[string]string{"room_id": roomID},
		}, push.Notification{
			Title: "New message",
			Body:  "You have a new message",
			URL:   "/chat/" + roomID,
		})
	}, chat.HandlerConfig{
		IsBlockedInRoom: isBlockedInRoom,
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
					log.Printf("chat: delete file %s: %v", key, err)
				}
			}
		},
		ReadFile:  fileStorage.GetObject,
		IsBlocked: contactSvc.IsBlocked,
	})

	uploadStore := uploads.NewStore(pool)
	uploadSvc := uploads.NewService(uploadStore, fileStorage)

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

	imageProcessor := worker.NewImageProcessor(fileStorage, uploadStore, config.EnvIntOrDefault("IMAGE_MAX_PX", 0))
	workerServer := worker.NewServer(redisConnOpt, 4)
	if err := workerServer.Start(imageProcessor); err != nil {
		log.Fatalf("Worker server failed to start: %v", err)
	}
	defer workerServer.Shutdown()

	ephemeralCleaner := worker.NewEphemeralCleaner(chatStore, fileStorage, func(roomID, messageID string) {
		data, _ := json.Marshal(map[string]string{
			"event":   "message_deleted",
			"id":      messageID,
			"room_id": roomID,
		})
		chatHub.Publish(appCtx, roomID, data) //nolint:errcheck
	})
	ephemeralCleaner.Start(appCtx)

	// Daily purge of accounts past the 30-day deletion grace period.
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		worker.PurgeDeletedAccounts(appCtx, authStore, fileStorage)
		for range ticker.C {
			worker.PurgeDeletedAccounts(appCtx, authStore, fileStorage)
		}
	}()

	uploadHandler := uploads.NewHandler(uploadSvc)
	adminHandler := admin.NewHandler(adminSvc)

	requireAuth := middleware.RequireAuth(jwtSecret, adminSvc)

	var testHandler http.Handler
	if config.EnvOrDefault("TEST_ENDPOINTS_ENABLED", "false") == "true" {
		log.Println("WARNING: test endpoints enabled — do not use in production")
		testHandler = newTestHandler(pool, authSvc, profileStore, jwtSecret, tokenExpiry)
	}

	h := server.New(pool, env, corsOrigins, authHandler, accountHandler, profileHandler, profiles.PublicAvailableHandler(profileSvc), contactsHandler, notificationsHandler, chatHandler, chatWSHandler, presenceHandler, uploadHandler, reportsHandler, pushHandler, adminHandler, localStorageHandler, testHandler, requireAuth)

	log.Printf("Server running on :%s (env: %s)\n", port, env)
	if err := http.ListenAndServe(":"+port, h); err != nil {
		log.Fatalf("Server error: %v", err)
	}
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
			req.Username = "u_" + strings.ReplaceAll(req.Email[:strings.Index(req.Email, "@")], ".", "_")
		}

		user, err := authSvc.Register(r.Context(), req.Email, req.Password)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}) //nolint:errcheck
			return
		}

		// Mark email as verified directly in the database.
		pool.Exec(r.Context(), `UPDATE users SET email_verified_at = now() WHERE id = $1`, user.ID) //nolint:errcheck,exhaustruct

		// Seed profile.
		dob := time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)
		profileStore.Upsert(r.Context(), user.ID, profiles.ProfileInput{ //nolint:errcheck
			Username:    req.Username,
			DisplayName: req.Username,
			DateOfBirth: &dob,
		})

		tok, err := token.Generate(user.ID, user.IsAdmin, jwtSecret, tokenExpiry)
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
// with web push notifications for offline users.
type contactNotifier struct {
	notifyFn func(userID string, e notifications.Event, n push.Notification)
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
	c.notifyFn(userID, e, n)
}
