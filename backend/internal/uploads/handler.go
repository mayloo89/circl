package uploads

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/apierror"
	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/storage"
)

// NewHandler returns a chi router with the upload endpoints.
// Must be mounted behind requireAuth.
func NewHandler(svc *Service) http.Handler {
	r := chi.NewRouter()
	r.Post("/request", requestHandler(svc))
	r.Post("/{id}/confirm", confirmHandler(svc))
	r.Get("/{id}", getHandler(svc))
	return r
}

// GET /uploads/{id} returns the owner-only view of an upload row. The
// frontend polls this after confirm to discover the moderation outcome — a
// rejected status carries the human-readable reason for the user.
func getHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		uploadID := chi.URLParam(r, "id")
		u, err := svc.GetUploadForUser(r.Context(), uploadID, userID)
		if err != nil {
			switch {
			case errors.Is(err, ErrNotFound):
				apierror.Write(w, http.StatusNotFound, apierror.CodeNotFound, "upload not found")
			case errors.Is(err, ErrForbidden):
				apierror.Write(w, http.StatusForbidden, apierror.CodeForbidden, "forbidden")
			default:
				zerolog.Ctx(r.Context()).Error().Err(err).Msg("uploads: get upload failed")
				apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			}
			return
		}
		apierror.WriteJSON(w, http.StatusOK, u)
	}
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
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
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
				apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, err.Error())
			default:
				zerolog.Ctx(r.Context()).Error().Err(err).Msg("uploads: request failed")
				apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			}
			return
		}

		apierror.WriteJSON(w, http.StatusCreated, out)
	}
}

// POST /uploads/{id}/confirm
func confirmHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}

		uploadID := chi.URLParam(r, "id")
		if uploadID == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "missing upload id")
			return
		}

		out, err := svc.ConfirmUpload(r.Context(), uploadID, userID)
		if err != nil {
			switch {
			case errors.Is(err, ErrNotFound):
				apierror.Write(w, http.StatusNotFound, apierror.CodeNotFound, "upload not found")
			case errors.Is(err, ErrForbidden):
				apierror.Write(w, http.StatusForbidden, apierror.CodeForbidden, "forbidden")
			case errors.Is(err, ErrNotPending):
				apierror.Write(w, http.StatusConflict, apierror.CodeInvalidRequest, "upload already confirmed")
			default:
				zerolog.Ctx(r.Context()).Error().Err(err).Msg("uploads: confirm upload failed")
				apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			}
			return
		}

		apierror.WriteJSON(w, http.StatusOK, out)
	}
}
