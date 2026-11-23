package evidence

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/apierror"
	"github.com/mayloo89/circl/backend/internal/middleware"
)

// Reader is what the admin handler needs from the Service.
type Reader interface {
	List(ctx context.Context, reportID string) ([]File, error)
	Open(ctx context.Context, reportID, fileID string) (*File, io.ReadCloser, error)
}

// NewAdminHandler serves a report's evidence to admins. It is mounted by the
// reports router at /reports/{id}/evidence, behind RequireAdmin, and reads the
// report ID from that route's "id" parameter.
//
//	GET            — list the report's preserved files (metadata only)
//	GET /{fileID}  — stream one file; refused for CSAM reports
func NewAdminHandler(svc Reader) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listHandler(svc))
	r.Get("/{fileID}", openHandler(svc))
	return r
}

func listHandler(svc Reader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		files, err := svc.List(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		apierror.WriteJSON(w, http.StatusOK, files)
	}
}

func openHandler(svc Reader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reportID, fileID := chi.URLParam(r, "id"), chi.URLParam(r, "fileID")
		adminID, _ := middleware.UserIDFromContext(r.Context())
		log := zerolog.Ctx(r.Context()).With().
			Str("event", "evidence_access").
			Str("admin_id", adminID).
			Str("report_id", reportID).
			Str("file_id", fileID).
			Logger()

		f, rc, err := svc.Open(r.Context(), reportID, fileID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apierror.Write(w, http.StatusNotFound, apierror.CodeNotFound, "evidence file not found")
				return
			}
			log.Error().Err(err).Msg("evidence open failed")
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		if rc == nil {
			log.Warn().Msg("evidence view refused: restricted report")
			apierror.Write(w, http.StatusForbidden, apierror.CodeEvidenceRestricted, "this evidence is preserved for law enforcement and cannot be viewed")
			return
		}
		defer func() { _ = rc.Close() }()

		log.Info().Msg("evidence viewed")
		h := w.Header()
		h.Set("Content-Type", f.ContentType)
		h.Set("Content-Length", strconv.FormatInt(f.SizeBytes, 10))
		// Never cached, never sniffed into another type, and downloaded rather
		// than rendered if someone opens the URL directly.
		h.Set("Cache-Control", "no-store")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Content-Disposition", `attachment; filename="`+f.ID+`"`)
		w.WriteHeader(http.StatusOK)
		if _, err := io.Copy(w, rc); err != nil {
			log.Warn().Err(err).Msg("evidence stream interrupted")
		}
	}
}
