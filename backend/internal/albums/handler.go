package albums

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/mayloo89/circl/backend/internal/apierror"
	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/storage"
)

// NewHandler returns the chi router for the albums HTTP surface. Mount
// behind requireAuth — every endpoint reads the caller from the auth
// context and gates access through the service layer.
func NewHandler(svc *Service, store storage.Storage) http.Handler {
	r := chi.NewRouter()
	r.Post("/", createAlbum(svc))
	r.Get("/me", listMyAlbums(svc))
	r.Get("/shared-with-me", listSharedAlbums(svc))
	r.Get("/{id}", getAlbum(svc))
	r.Patch("/{id}", updateAlbum(svc))
	r.Delete("/{id}", deleteAlbum(svc))

	r.Get("/{id}/photos", listPhotos(svc))
	r.Post("/{id}/photos", addPhoto(svc))
	r.Delete("/{id}/photos/{uploadID}", removePhoto(svc))
	r.Get("/{id}/photos/{uploadID}/file", streamPhoto(svc, store))

	r.Get("/{id}/grants", listGrants(svc))
	r.Post("/{id}/grants/invite", inviteGrant(svc))
	r.Post("/{id}/grants/request", requestGrant(svc))
	r.Post("/grants/{grantID}/accept", acceptGrant(svc))
	r.Post("/grants/{grantID}/deny", denyGrant(svc))
	r.Post("/grants/{grantID}/revoke", revokeGrant(svc))

	r.Post("/{id}/share-in-chat", shareInChat(svc))
	return r
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		apierror.Write(w, http.StatusNotFound, apierror.CodeNotFound, "not found")
	case errors.Is(err, ErrForbidden):
		apierror.Write(w, http.StatusForbidden, apierror.CodeForbidden, "forbidden")
	case errors.Is(err, ErrInvalidRequest):
		apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request")
	case errors.Is(err, ErrGrantExists):
		apierror.Write(w, http.StatusConflict, apierror.CodeInvalidRequest, "an open grant already exists for this user")
	case errors.Is(err, ErrSelfGrant):
		apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "cannot grant access to yourself")
	default:
		apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
	}
}

func userID(r *http.Request, w http.ResponseWriter) (string, bool) {
	id, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
		return "", false
	}
	return id, true
}

func createAlbum(svc *Service) http.HandlerFunc {
	type request struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := userID(r, w)
		if !ok {
			return
		}
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
			return
		}
		a, err := svc.CreateAlbum(r.Context(), uid, req.Name, req.Description)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		a.Role = "owner"
		apierror.WriteJSON(w, http.StatusCreated, a)
	}
}

func listMyAlbums(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := userID(r, w)
		if !ok {
			return
		}
		rows, err := svc.ListMyAlbums(r.Context(), uid)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, rows)
	}
}

func listSharedAlbums(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := userID(r, w)
		if !ok {
			return
		}
		rows, err := svc.ListSharedWithMe(r.Context(), uid)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, rows)
	}
}

func getAlbum(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := userID(r, w)
		if !ok {
			return
		}
		a, err := svc.GetAlbum(r.Context(), uid, chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, a)
	}
}

func updateAlbum(svc *Service) http.HandlerFunc {
	type request struct {
		Name          *string `json:"name,omitempty"`
		Description   *string `json:"description,omitempty"`
		CoverUploadID *string `json:"cover_upload_id,omitempty"`
		ClearCover    bool    `json:"clear_cover,omitempty"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := userID(r, w)
		if !ok {
			return
		}
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
			return
		}
		a, err := svc.UpdateAlbum(r.Context(), uid, chi.URLParam(r, "id"), AlbumPatch{
			Name:          req.Name,
			Description:   req.Description,
			CoverUploadID: req.CoverUploadID,
			ClearCover:    req.ClearCover,
		})
		if err != nil {
			writeServiceError(w, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, a)
	}
}

func deleteAlbum(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := userID(r, w)
		if !ok {
			return
		}
		if err := svc.DeleteAlbum(r.Context(), uid, chi.URLParam(r, "id")); err != nil {
			writeServiceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func listPhotos(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := userID(r, w)
		if !ok {
			return
		}
		rows, err := svc.ListPhotos(r.Context(), uid, chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, rows)
	}
}

func addPhoto(svc *Service) http.HandlerFunc {
	type request struct {
		UploadID string `json:"upload_id"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := userID(r, w)
		if !ok {
			return
		}
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UploadID == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
			return
		}
		if err := svc.AddPhoto(r.Context(), uid, chi.URLParam(r, "id"), req.UploadID); err != nil {
			writeServiceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func removePhoto(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := userID(r, w)
		if !ok {
			return
		}
		if err := svc.RemovePhoto(r.Context(), uid, chi.URLParam(r, "id"), chi.URLParam(r, "uploadID")); err != nil {
			writeServiceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func streamPhoto(svc *Service, store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := userID(r, w)
		if !ok {
			return
		}
		key, contentType, err := svc.StreamPhoto(r.Context(), uid, chi.URLParam(r, "id"), chi.URLParam(r, "uploadID"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		obj, err := store.GetObject(r.Context(), key)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "failed to fetch photo")
			return
		}
		defer obj.Close()
		w.Header().Set("Content-Type", contentType)
		// Private cache only — short max-age trades a little freshness on
		// revoke for fewer round-trips while the viewer scrolls.
		w.Header().Set("Cache-Control", "private, max-age=60")
		_, _ = io.Copy(w, obj)
	}
}

func listGrants(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := userID(r, w)
		if !ok {
			return
		}
		rows, err := svc.ListGrants(r.Context(), uid, chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, rows)
	}
}

func inviteGrant(svc *Service) http.HandlerFunc {
	type request struct {
		GranteeID string `json:"grantee_id"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := userID(r, w)
		if !ok {
			return
		}
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.GranteeID == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
			return
		}
		g, err := svc.InviteUser(r.Context(), uid, chi.URLParam(r, "id"), req.GranteeID)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		apierror.WriteJSON(w, http.StatusCreated, g)
	}
}

func requestGrant(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := userID(r, w)
		if !ok {
			return
		}
		g, err := svc.RequestAccess(r.Context(), uid, chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		apierror.WriteJSON(w, http.StatusCreated, g)
	}
}

func acceptGrant(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := userID(r, w)
		if !ok {
			return
		}
		g, err := svc.AcceptGrant(r.Context(), uid, chi.URLParam(r, "grantID"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, g)
	}
}

func denyGrant(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := userID(r, w)
		if !ok {
			return
		}
		g, err := svc.DenyGrant(r.Context(), uid, chi.URLParam(r, "grantID"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, g)
	}
}

func shareInChat(svc *Service) http.HandlerFunc {
	type request struct {
		RoomID string `json:"room_id"`
	}
	type response struct {
		Album *Album `json:"album"`
		Grant *Grant `json:"grant"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := userID(r, w)
		if !ok {
			return
		}
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RoomID == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
			return
		}
		a, g, err := svc.ShareInChatRoom(r.Context(), uid, chi.URLParam(r, "id"), req.RoomID)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		apierror.WriteJSON(w, http.StatusCreated, response{Album: a, Grant: g})
	}
}

func revokeGrant(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := userID(r, w)
		if !ok {
			return
		}
		g, err := svc.RevokeGrant(r.Context(), uid, chi.URLParam(r, "grantID"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, g)
	}
}
