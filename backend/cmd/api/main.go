package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment")
	}

	port := envOrDefault("PORT", "8080")
	env := envOrDefault("ENV", "development")
	corsOrigins := strings.Split(envOrDefault("CORS_ALLOWED_ORIGINS", "http://localhost:3000"), ",")
	databaseURL := mustEnv("DATABASE_URL")

	// Correr migraciones antes de aceptar tráfico
	runMigrations(databaseURL)

	// Abrir pool de conexiones
	db := mustOpenDB(databaseURL)
	defer db.Close()

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

	log.Printf("Server running on :%s (env: %s)\n", port, env)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// healthHandler responde con el estado del servidor y de la base de datos.
func healthHandler(db *pgxpool.Pool, env string) http.HandlerFunc {
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

// runMigrations aplica todas las migraciones pendientes al iniciar.
func runMigrations(databaseURL string) {
	// golang-migrate/pgx5 espera el scheme "pgx5://"
	migrateURL := strings.Replace(databaseURL, "postgres://", "pgx5://", 1)
	migrateURL = strings.Replace(migrateURL, "postgresql://", "pgx5://", 1)

	m, err := migrate.New("file://migrations", migrateURL)
	if err != nil {
		log.Fatalf("Migration init error: %v", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("Migration failed: %v", err)
	}

	log.Println("Migrations applied successfully")
}

// mustOpenDB crea el pool de conexiones y verifica que la DB sea accesible.
func mustOpenDB(databaseURL string) *pgxpool.Pool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("Unable to create DB pool: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("Unable to connect to DB: %v", err)
	}

	log.Println("Database connection established")
	return pool
}

// envOrDefault retorna el valor de la variable de entorno o un valor por defecto.
func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// mustEnv retorna el valor de la variable de entorno o termina el proceso.
// Usar para variables obligatorias.
func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("Required environment variable %q is not set", key)
	}
	return v
}
