package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

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
	chatHandler := chat.NewHandler(chatSvc)
	chatWSHandler := chat.NewWSHandler(chatSvc, chatHub, jwtSecret, func(recipientID, roomID string) {
		hub.Notify(recipientID, notifications.Event{
			Type:    "new_message",
			Payload: map[string]string{"room_id": roomID},
		})
	})

	presenceStore := presence.NewStore(rdb, pool)
	presenceHandler := presence.NewHandler(presenceStore, hub)

	requireAuth := middleware.RequireAuth(jwtSecret)

	h := server.New(pool, env, corsOrigins, authHandler, profileHandler, contactsHandler, notificationsHandler, chatHandler, chatWSHandler, presenceHandler, requireAuth)

	log.Printf("Server running on :%s (env: %s)\n", port, env)
	if err := http.ListenAndServe(":"+port, h); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
