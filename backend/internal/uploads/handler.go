package uploads

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/storage"
)

// NewHandler returns a chi router with the upload endpoints.
// Must be mounted behind requireAuth.
func NewHandler(svc *Service) http.Handler {
	r := chi.NewRouter()
	r.Post("/request", requestHandler(svc))
	r.Post("/{id}/confirm", confirmHandler(svc))
	return r
}

// POST /uploads/request
func requestHandler(svc *Service) http.HandlerFunc {
	type request struct {
		Category    string `json:"category"`
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
		SizeBytes   int64  `json:"size_bytes"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}

		out, err := svc.RequestUpload(r.Context(), RequestUploadInput{
			UserID:      userID,
			Category:    req.Category,
			Filename:    req.Filename,
			ContentType: req.ContentType,
			SizeBytes:   req.SizeBytes,
		})
		if err != nil {
			switch {
			case errors.Is(err, storage.ErrInvalidCategory),
				errors.Is(err, storage.ErrInvalidContentType),
				errors.Is(err, storage.ErrFileTooLarge),
				errors.Is(err, storage.ErrInvalidFilename):
				http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
			default:
				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(out) //nolint:errcheck
	}
}

// POST /uploads/{id}/confirm
func confirmHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		uploadID := chi.URLParam(r, "id")
		if uploadID == "" {
			http.Error(w, `{"error":"missing upload id"}`, http.StatusBadRequest)
			return
		}

		out, err := svc.ConfirmUpload(r.Context(), uploadID, userID)
		if err != nil {
			switch {
			case errors.Is(err, ErrNotFound):
				http.Error(w, `{"error":"upload not found"}`, http.StatusNotFound)
			case errors.Is(err, ErrForbidden):
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			case errors.Is(err, ErrNotPending):
				http.Error(w, `{"error":"upload already confirmed"}`, http.StatusConflict)
			default:
				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out) //nolint:errcheck
	}
}
