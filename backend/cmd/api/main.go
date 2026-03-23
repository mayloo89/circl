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

	"github.com/mayloo89/circl/backend/internal/auth"
	"github.com/mayloo89/circl/backend/internal/chat"
	"github.com/mayloo89/circl/backend/internal/config"
	"github.com/mayloo89/circl/backend/internal/contacts"
	"github.com/mayloo89/circl/backend/internal/db"
	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/notifications"
	"github.com/mayloo89/circl/backend/internal/presence"
	"github.com/mayloo89/circl/backend/internal/profiles"
	"github.com/mayloo89/circl/backend/internal/server"
	"github.com/mayloo89/circl/backend/internal/storage"
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

	authStore := auth.NewStore(pool)
	authSvc := auth.NewService(authStore)
	authHandler := auth.NewHandler(authSvc, jwtSecret, tokenExpiry)

	profileStore := profiles.NewStore(pool)
	profileSvc := profiles.NewService(profileStore)
	profileHandler := profiles.NewHandler(profileSvc)

	hub := notifications.NewHub()
	notificationsHandler := notifications.NewHandler(hub, jwtSecret)

	contactStore := contacts.NewStore(pool)
	contactSvc := contacts.NewService(contactStore)
	contactsHandler := contacts.NewHandler(contactSvc, contacts.WithNotifier(hub))

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

	chatHub := chat.NewHub(rdb)
	go chatHub.Run(appCtx)

	chatStore := chat.NewStore(pool)
	chatSvc := chat.NewService(chatStore)
	chatWSHandler := chat.NewWSHandler(chatSvc, chatHub, jwtSecret, func(recipientID, roomID string) {
		hub.Notify(recipientID, notifications.Event{
			Type:    "new_message",
			Payload: map[string]string{"room_id": roomID},
		})
	})

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

	notifyDeleted := func(roomID, messageID string) {
		data, _ := json.Marshal(map[string]string{
			"event":   "message_deleted",
			"id":      messageID,
			"room_id": roomID,
		})
		chatHub.Publish(appCtx, roomID, data) //nolint:errcheck
	}
	chatHandler := chat.NewHandler(chatSvc, chat.HandlerConfig{
		NotifyMessageDeleted: notifyDeleted,
		DeleteFiles: func(ctx context.Context, keys []string) {
			for _, key := range keys {
				if err := fileStorage.Delete(ctx, key); err != nil {
					log.Printf("chat: delete file %s: %v", key, err)
				}
			}
		},
		ReadFile: fileStorage.GetObject,
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

	imageProcessor := worker.NewImageProcessor(fileStorage, uploadStore)
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

	uploadHandler := uploads.NewHandler(uploadSvc)

	requireAuth := middleware.RequireAuth(jwtSecret)

	h := server.New(pool, env, corsOrigins, authHandler, profileHandler, contactsHandler, notificationsHandler, chatHandler, chatWSHandler, presenceHandler, uploadHandler, localStorageHandler, requireAuth)

	log.Printf("Server running on :%s (env: %s)\n", port, env)
	if err := http.ListenAndServe(":"+port, h); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
