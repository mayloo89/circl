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
}

type profileResponse struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
	AvatarURL   string `json:"avatar_url"`
}

type updateRequest struct {
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
	AvatarURL   string `json:"avatar_url"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// NewHandler returns an http.Handler with all profile routes.
func NewHandler(svc ProfileManager) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /profiles/me", getMyProfile(svc))
	mux.HandleFunc("PUT /profiles/me", updateMyProfile(svc))
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

func toResponse(p *Profile) profileResponse {
	return profileResponse{
		ID:          p.ID,
		UserID:      p.UserID,
		DisplayName: p.DisplayName,
		Bio:         p.Bio,
		AvatarURL:   p.AvatarURL,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
