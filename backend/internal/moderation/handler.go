package moderation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mayloo89/circl/backend/internal/apierror"
	"github.com/mayloo89/circl/backend/internal/middleware"
)

// RejectedUpload is one row returned by GET /admin/moderation.
type RejectedUpload struct {
	UploadID    string    `json:"upload_id"`
	UserID      string    `json:"user_id"`
	UserEmail   string    `json:"user_email"`
	StorageKey  string    `json:"storage_key"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	Code        string    `json:"code"`
	Reason      string    `json:"reason"`
	ModeratedAt time.Time `json:"moderated_at"`
}

// HashAddRequest is the body of POST /admin/hashes.
type HashAddRequest struct {
	Hash   string `json:"hash"`
	Source string `json:"source"`
	Reason string `json:"reason"`
}

// AdminStore is the persistence interface the admin handler needs.
type AdminStore interface {
	ListRejected(ctx context.Context, limit int) ([]RejectedUpload, error)
}

// pgAdminStore reads the rejected-uploads view directly from Postgres.
type pgAdminStore struct {
	pool *pgxpool.Pool
}

// NewAdminStore returns a Postgres-backed AdminStore.
func NewAdminStore(pool *pgxpool.Pool) AdminStore {
	return &pgAdminStore{pool: pool}
}

func (s *pgAdminStore) ListRejected(ctx context.Context, limit int) ([]RejectedUpload, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.user_id, users.email, u.storage_key, u.filename, u.content_type, u.size_bytes,
		       u.moderation_code, u.moderation_reason, u.moderated_at
		  FROM uploads u
		  JOIN users ON users.id = u.user_id
		 WHERE u.moderation_status = 'rejected'
		 ORDER BY u.moderated_at DESC
		 LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list rejected: %w", err)
	}
	defer rows.Close()
	out := []RejectedUpload{}
	for rows.Next() {
		var r RejectedUpload
		if err := rows.Scan(&r.UploadID, &r.UserID, &r.UserEmail, &r.StorageKey, &r.Filename,
			&r.ContentType, &r.SizeBytes, &r.Code, &r.Reason, &r.ModeratedAt); err != nil {
			return nil, fmt.Errorf("scan rejected: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// NewAdminHandler returns the admin-only handler for moderation routes.
// Mount inside the existing /admin router so RequireAdmin already applies.
//
//	GET  /          — list rejected uploads (newest first)
//	POST /hashes    — add a SHA-256 to the local block list
func NewAdminHandler(store AdminStore, hashes HashStore) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequireAdmin)
	r.Get("/", listRejected(store))
	r.Post("/hashes", addHash(hashes))
	return r
}

func listRejected(store AdminStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 50
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				limit = n
			}
		}
		items, err := store.ListRejected(r.Context(), limit)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		apierror.WriteJSON(w, http.StatusOK, items)
	}
}

func addHash(hashes HashStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		var req HashAddRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
			return
		}
		if req.Hash == "" || req.Reason == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "hash and reason are required")
			return
		}
		source := req.Source
		if source == "" {
			source = "local"
		}
		if err := hashes.Add(r.Context(), HashEntry{Hash: req.Hash, Source: source, Reason: req.Reason}, adminID); err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
