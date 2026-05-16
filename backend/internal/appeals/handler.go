package appeals

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/mayloo89/circl/backend/internal/apierror"
	"github.com/mayloo89/circl/backend/internal/middleware"
)

// MaxBodyLen caps the size of a user-submitted appeal statement. 4kB is
// generous for a free-form text-area while keeping the row bounded.
const MaxBodyLen = 4096

// NewPublicHandler returns an http.Handler covering the un-authenticated
// /appeal/{token} routes used by suspended users. Mount at /appeal.
func NewPublicHandler(svc *Service) http.Handler {
	r := chi.NewRouter()
	r.Get("/{token}", getPublicAppeal(svc))
	r.Post("/{token}", submitPublicAppeal(svc))
	return r
}

// NewAdminHandler returns an http.Handler for admin appeal-review routes.
// Mount behind RequireAdmin. Suggested mount: /admin/appeals.
func NewAdminHandler(svc *Service) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequireAdmin)
	r.Get("/", listAppeals(svc))
	r.Put("/{id}", resolveAppeal(svc))
	return r
}

func getPublicAppeal(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := chi.URLParam(r, "token")
		view, err := svc.GetPublic(r.Context(), token)
		if err != nil {
			writePublicError(w, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, view)
	}
}

func submitPublicAppeal(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := chi.URLParam(r, "token")
		var req struct {
			Body string `json:"body"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
			return
		}
		if req.Body == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "body is required")
			return
		}
		if len(req.Body) > MaxBodyLen {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "body too long")
			return
		}
		view, err := svc.SubmitPublic(r.Context(), token, req.Body)
		if err != nil {
			writePublicError(w, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, view)
	}
}

func listAppeals(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		list, err := svc.List(r.Context(), status)
		if err != nil {
			if errors.Is(err, ErrInvalidStatus) {
				apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid status")
				return
			}
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		apierror.WriteJSON(w, http.StatusOK, list)
	}
}

func resolveAppeal(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		id := chi.URLParam(r, "id")
		var req struct {
			Status string `json:"status"`
			Note   string `json:"note"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
			return
		}
		appeal, err := svc.Resolve(r.Context(), id, req.Status, adminID, req.Note)
		if err != nil {
			switch {
			case errors.Is(err, ErrInvalidStatus):
				apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid status: must be approved or denied")
			case errors.Is(err, ErrNotFound):
				apierror.Write(w, http.StatusNotFound, apierror.CodeNotFound, "appeal not found")
			case errors.Is(err, ErrAlreadyResolved):
				apierror.Write(w, http.StatusConflict, apierror.CodeAppealAlreadyResolved, "appeal already resolved")
			default:
				apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			}
			return
		}
		apierror.WriteJSON(w, http.StatusOK, appeal)
	}
}

func writePublicError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidToken):
		apierror.Write(w, http.StatusNotFound, apierror.CodeInvalidToken, "invalid or expired appeal token")
	case errors.Is(err, ErrAlreadyResolved):
		apierror.Write(w, http.StatusConflict, apierror.CodeAppealAlreadyResolved, "appeal has already been resolved")
	default:
		apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
	}
}
