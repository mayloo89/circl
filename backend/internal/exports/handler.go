package exports

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/apierror"
	"github.com/mayloo89/circl/backend/internal/middleware"
)

// NewAuthedHandler returns the authenticated routes for the export flow.
// Mount behind RequireAuth at a path the user is already at — e.g.
// /users/me/exports. Routes:
//
//	POST /         — request a new export
//	GET  /         — get the latest request's status
func NewAuthedHandler(svc *Service) http.Handler {
	r := chi.NewRouter()
	r.Post("/", requestExport(svc))
	r.Get("/", latestStatus(svc))
	return r
}

// NewDownloadHandler returns the un-authenticated download route mounted at
// e.g. /account/export/{token}. The single-use token in the path is the
// caller's bearer credential — the user may have lost their session by the
// time the email arrives, and asking them to log back in to download data
// they already requested would be hostile.
func NewDownloadHandler(svc *Service) http.Handler {
	r := chi.NewRouter()
	r.Get("/{token}", download(svc))
	return r
}

func requestExport(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		req, err := svc.Request(r.Context(), userID)
		if err != nil {
			switch {
			case errors.Is(err, ErrAlreadyPending):
				apierror.Write(w, http.StatusConflict, apierror.CodeExportAlreadyPending, "an export is already being prepared")
			case errors.Is(err, ErrRateLimited):
				apierror.Write(w, http.StatusTooManyRequests, apierror.CodeRateLimited, "an export was requested recently; please try again later")
			default:
				zerolog.Ctx(r.Context()).Error().Err(err).Msg("exports: request failed")
				apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			}
			return
		}
		apierror.WriteJSON(w, http.StatusAccepted, PublicStatus{
			ID:          req.ID,
			Status:      req.Status,
			RequestedAt: req.RequestedAt,
			CompletedAt: req.CompletedAt,
			ExpiresAt:   req.ExpiresAt,
		})
	}
}

func latestStatus(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		st, err := svc.Status(r.Context(), userID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apierror.WriteJSON(w, http.StatusOK, map[string]any{"status": "none"})
				return
			}
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		apierror.WriteJSON(w, http.StatusOK, st)
	}
}

func download(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := chi.URLParam(r, "token")
		req, rc, err := svc.Download(r.Context(), token)
		if err != nil {
			switch {
			case errors.Is(err, ErrInvalidToken):
				apierror.Write(w, http.StatusNotFound, apierror.CodeInvalidToken, "invalid or expired download token")
			case errors.Is(err, ErrNotReady):
				apierror.Write(w, http.StatusConflict, apierror.CodeExportNotReady, "export not ready")
			default:
				zerolog.Ctx(r.Context()).Error().Err(err).Msg("exports: download failed")
				apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			}
			return
		}
		defer func() { _ = rc.Close() }()
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="circl-export-%s.zip"`, req.ID))
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, rc)
	}
}
