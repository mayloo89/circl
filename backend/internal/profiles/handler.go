package profiles

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mayloo89/circl/backend/internal/middleware"
)

// ProfileManager is the interface the handler depends on.
type ProfileManager interface {
	GetMyProfile(ctx context.Context, userID string) (*Profile, error)
	UpdateMyProfile(ctx context.Context, userID, displayName, bio, avatarURL string) (*Profile, error)
	GetPublicProfile(ctx context.Context, userID string) (*Profile, error)
	AddPhoto(ctx context.Context, userID, url string) (*ProfilePhoto, error)
	DeletePhoto(ctx context.Context, userID, photoID string) error
}

type photoResponse struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type profileResponse struct {
	ID          string          `json:"id"`
	UserID      string          `json:"user_id"`
	DisplayName string          `json:"display_name"`
	Bio         string          `json:"bio"`
	AvatarURL   string          `json:"avatar_url"`
	Photos      []photoResponse `json:"photos"`
}

type updateRequest struct {
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
	AvatarURL   string `json:"avatar_url"`
}

type addPhotoRequest struct {
	URL string `json:"url"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// NewHandler returns an http.Handler with all profile routes.
func NewHandler(svc ProfileManager) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /profiles/me", getMyProfile(svc))
	mux.HandleFunc("PUT /profiles/me", updateMyProfile(svc))
	mux.HandleFunc("POST /profiles/me/photos", addPhoto(svc))
	mux.HandleFunc("DELETE /profiles/me/photos/{photoID}", deletePhoto(svc))
	mux.HandleFunc("GET /profiles/{userID}", getPublicProfile(svc))
	return mux
}

func getMyProfile(svc ProfileManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errorResponse{"unauthorized"})
			return
		}

		profile, err := svc.GetMyProfile(r.Context(), userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{"internal server error"})
			return
		}

		writeJSON(w, http.StatusOK, toResponse(profile))
	}
}

func updateMyProfile(svc ProfileManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errorResponse{"unauthorized"})
			return
		}

		var req updateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{"invalid request body"})
			return
		}

		profile, err := svc.UpdateMyProfile(r.Context(), userID, req.DisplayName, req.Bio, req.AvatarURL)
		if err != nil {
			if errors.Is(err, ErrInvalidInput) {
				writeJSON(w, http.StatusBadRequest, errorResponse{err.Error()})
				return
			}
			writeJSON(w, http.StatusInternalServerError, errorResponse{"internal server error"})
			return
		}

		writeJSON(w, http.StatusOK, toResponse(profile))
	}
}

func getPublicProfile(svc ProfileManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errorResponse{"unauthorized"})
			return
		}

		targetUserID := r.PathValue("userID")
		profile, err := svc.GetPublicProfile(r.Context(), targetUserID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeJSON(w, http.StatusNotFound, errorResponse{"profile not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, errorResponse{"internal server error"})
			return
		}

		writeJSON(w, http.StatusOK, toResponse(profile))
	}
}

func addPhoto(svc ProfileManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errorResponse{"unauthorized"})
			return
		}

		var req addPhotoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{"invalid request body"})
			return
		}

		photo, err := svc.AddPhoto(r.Context(), userID, req.URL)
		if err != nil {
			if errors.Is(err, ErrInvalidInput) {
				writeJSON(w, http.StatusUnprocessableEntity, errorResponse{err.Error()})
				return
			}
			writeJSON(w, http.StatusInternalServerError, errorResponse{"internal server error"})
			return
		}

		writeJSON(w, http.StatusCreated, photoResponse{ID: photo.ID, URL: photo.URL})
	}
}

func deletePhoto(svc ProfileManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errorResponse{"unauthorized"})
			return
		}

		photoID := r.PathValue("photoID")
		if err := svc.DeletePhoto(r.Context(), userID, photoID); err != nil {
			if errors.Is(err, ErrPhotoNotFound) {
				writeJSON(w, http.StatusNotFound, errorResponse{"photo not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, errorResponse{"internal server error"})
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func toResponse(p *Profile) profileResponse {
	photos := make([]photoResponse, len(p.Photos))
	for i, ph := range p.Photos {
		photos[i] = photoResponse{ID: ph.ID, URL: ph.URL}
	}
	return profileResponse{
		ID:          p.ID,
		UserID:      p.UserID,
		DisplayName: p.DisplayName,
		Bio:         p.Bio,
		AvatarURL:   p.AvatarURL,
		Photos:      photos,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
