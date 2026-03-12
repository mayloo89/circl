package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/joho/godotenv"

	"github.com/mayloo89/circl/backend/internal/auth"
	"github.com/mayloo89/circl/backend/internal/config"
	"github.com/mayloo89/circl/backend/internal/db"
	"github.com/mayloo89/circl/backend/internal/server"
)

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

	if err := db.Migrate("migrations", databaseURL); err != nil {
		log.Fatalf("Migrations failed: %v", err)
	}
	log.Println("Migrations applied successfully")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.Open(ctx, databaseURL)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer pool.Close()
	log.Println("Database connection established")

	authStore := auth.NewStore(pool)
	authSvc := auth.NewService(authStore)
	authHandler := auth.NewHandler(authSvc)

	h := server.New(pool, env, corsOrigins, authHandler)

	log.Printf("Server running on :%s (env: %s)\n", port, env)
	if err := http.ListenAndServe(":"+port, h); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
