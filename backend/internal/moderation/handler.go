package moderation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mayloo89/circl/backend/internal/apierror"
	"github.com/mayloo89/circl/backend/internal/middleware"
)

// RejectedUpload is one row returned by GET /admin/moderation.
//
// Score and Categories are populated only for NSFW rejections; hash-list and
// heuristic rejections leave them at their zero values. FileRetained tells the
// UI whether the storage object can still be fetched via GET
// /admin/moderation/{id}/image — hash-list rejections always set this to
// false (CSAM/NCII matches are purged immediately by policy).
type RejectedUpload struct {
	UploadID     string    `json:"upload_id"`
	UserID       string    `json:"user_id"`
	UserEmail    string    `json:"user_email"`
	StorageKey   string    `json:"storage_key"`
	Filename     string    `json:"filename"`
	ContentType  string    `json:"content_type"`
	SizeBytes    int64     `json:"size_bytes"`
	Code         string    `json:"code"`
	Reason       string    `json:"reason"`
	ModeratedAt  time.Time `json:"moderated_at"`
	Score        *float64  `json:"score,omitempty"`
	Categories   []string  `json:"categories,omitempty"`
	FileRetained bool      `json:"file_retained"`
}

// HashAddRequest is the body of POST /admin/hashes.
type HashAddRequest struct {
	Hash   string `json:"hash"`
	Source string `json:"source"`
	Reason string `json:"reason"`
}

// AdminStore is the persistence interface the admin handler needs.
type AdminStore interface {
	ListRejected(ctx context.Context, filter ListRejectedFilter) ([]RejectedUpload, error)
	GetRejected(ctx context.Context, uploadID string) (*RejectedUpload, error)
}

// ListRejectedFilter narrows the admin queue listing. Empty Code means "all
// codes"; Limit defaults to 50 and is capped at 200 in the store.
type ListRejectedFilter struct {
	Code  string
	Limit int
}

// ImageFetcher streams the bytes of a stored upload. Implemented by the same
// storage interface the rest of the app uses; declared locally so the admin
// handler doesn't take a dependency on the full Storage contract.
type ImageFetcher interface {
	GetObject(ctx context.Context, key string) (io.ReadCloser, error)
}

// pgAdminStore reads the rejected-uploads view directly from Postgres.
type pgAdminStore struct {
	pool *pgxpool.Pool
}

// NewAdminStore returns a Postgres-backed AdminStore.
func NewAdminStore(pool *pgxpool.Pool) AdminStore {
	return &pgAdminStore{pool: pool}
}

func (s *pgAdminStore) ListRejected(ctx context.Context, filter ListRejectedFilter) ([]RejectedUpload, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	// Two queries instead of building dynamic SQL — code filter is the only
	// optional clause, keeps the SQL readable and trivially analysed by EXPLAIN.
	var rows pgx.Rows
	var err error
	const base = `
		SELECT u.id, u.user_id, users.email, u.storage_key, u.filename, u.content_type, u.size_bytes,
		       u.moderation_code, u.moderation_reason, u.moderated_at,
		       u.moderation_score, u.moderation_categories, u.moderation_file_retained
		  FROM uploads u
		  JOIN users ON users.id = u.user_id
		 WHERE u.moderation_status = 'rejected'`
	if filter.Code != "" {
		rows, err = s.pool.Query(ctx, base+`
		   AND u.moderation_code = $1
		 ORDER BY u.moderated_at DESC
		 LIMIT $2`, filter.Code, limit)
	} else {
		rows, err = s.pool.Query(ctx, base+`
		 ORDER BY u.moderated_at DESC
		 LIMIT $1`, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list rejected: %w", err)
	}
	defer rows.Close()
	out := []RejectedUpload{}
	for rows.Next() {
		r, err := scanRejected(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetRejected returns a single rejected upload by id, including the storage
// key the admin image-stream endpoint needs. Returns ErrNotFound when no row
// matches or the row is not in moderation_status='rejected'.
func (s *pgAdminStore) GetRejected(ctx context.Context, uploadID string) (*RejectedUpload, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT u.id, u.user_id, users.email, u.storage_key, u.filename, u.content_type, u.size_bytes,
		       u.moderation_code, u.moderation_reason, u.moderated_at,
		       u.moderation_score, u.moderation_categories, u.moderation_file_retained
		  FROM uploads u
		  JOIN users ON users.id = u.user_id
		 WHERE u.id = $1
		   AND u.moderation_status = 'rejected'`, uploadID)
	r, err := scanRejected(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRejectedNotFound
		}
		return nil, err
	}
	return &r, nil
}

// rowScanner abstracts over pgx.Row and pgx.Rows so scanRejected can be
// reused by both the list and the single-row queries.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanRejected(row rowScanner) (RejectedUpload, error) {
	var r RejectedUpload
	var score *float64
	var categories []string
	if err := row.Scan(&r.UploadID, &r.UserID, &r.UserEmail, &r.StorageKey, &r.Filename,
		&r.ContentType, &r.SizeBytes, &r.Code, &r.Reason, &r.ModeratedAt,
		&score, &categories, &r.FileRetained); err != nil {
		return RejectedUpload{}, fmt.Errorf("scan rejected: %w", err)
	}
	r.Score = score
	r.Categories = categories
	return r, nil
}

// ErrRejectedNotFound is returned when a single-row admin lookup misses.
var ErrRejectedNotFound = errors.New("moderation: rejected upload not found")

// NewAdminHandler returns the admin-only handler for moderation routes.
// Mount inside the existing /admin router so RequireAdmin already applies.
//
//	GET  /                — list rejected uploads (newest first, optional ?code= filter)
//	GET  /{id}/image      — stream the retained original (only when FileRetained=true)
//	POST /hashes          — add a SHA-256 to the local block list
//
// images may be nil — in that case the image-stream endpoint always returns
// 404. Useful for tests / deployments where the storage adapter isn't wired.
func NewAdminHandler(store AdminStore, hashes HashStore, images ImageFetcher) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequireAdmin)
	r.Get("/", listRejected(store))
	r.Get("/{id}/image", streamImage(store, images))
	r.Post("/hashes", addHash(hashes))
	return r
}

func listRejected(store AdminStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter := ListRejectedFilter{Code: r.URL.Query().Get("code")}
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				filter.Limit = n
			}
		}
		items, err := store.ListRejected(r.Context(), filter)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		apierror.WriteJSON(w, http.StatusOK, items)
	}
}

func streamImage(store AdminStore, images ImageFetcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uploadID := chi.URLParam(r, "id")
		if uploadID == "" || images == nil {
			apierror.Write(w, http.StatusNotFound, apierror.CodeNotFound, "image not available")
			return
		}
		row, err := store.GetRejected(r.Context(), uploadID)
		if err != nil {
			if errors.Is(err, ErrRejectedNotFound) {
				apierror.Write(w, http.StatusNotFound, apierror.CodeNotFound, "rejected upload not found")
				return
			}
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		if !row.FileRetained {
			// Policy: hash-list (CSAM / NCII) rejections are purged on rejection
			// and the file is never available for admin review.
			apierror.Write(w, http.StatusGone, apierror.CodeNotFound, "image was purged")
			return
		}
		obj, err := images.GetObject(r.Context(), row.StorageKey)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "failed to fetch image")
			return
		}
		defer obj.Close()
		w.Header().Set("Content-Type", row.ContentType)
		w.Header().Set("Cache-Control", "private, no-store")
		_, _ = io.Copy(w, obj)
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
