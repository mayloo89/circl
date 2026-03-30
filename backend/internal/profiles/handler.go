package profiles

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/mayloo89/circl/backend/internal/middleware"
)

// ProfileManager is the interface the handler depends on.
type ProfileManager interface {
	GetMyProfile(ctx context.Context, userID string) (*Profile, error)
	UpdateMyProfile(ctx context.Context, userID string, in ProfileInput) (*Profile, error)
	UpdateAvatar(ctx context.Context, userID, avatarURL string) error
	GetPublicProfile(ctx context.Context, userID string) (*Profile, error)
	AddPhoto(ctx context.Context, userID, url string) (*ProfilePhoto, error)
	DeletePhoto(ctx context.Context, userID, photoID string) error
	GetMyPreferences(ctx context.Context, userID string) (*ProfilePreferences, error)
	UpdateMyPreferences(ctx context.Context, userID string, prefs ProfilePreferences) (*ProfilePreferences, error)
	SearchInterests(ctx context.Context, query string) ([]InterestSuggestion, error)
}

type photoResponse struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type profileResponse struct {
	ID           string          `json:"id"`
	UserID       string          `json:"user_id"`
	DisplayName  string          `json:"display_name"`
	Bio          string          `json:"bio"`
	AvatarURL    string          `json:"avatar_url"`
	DateOfBirth  *string         `json:"date_of_birth,omitempty"`
	Gender       string          `json:"gender"`
	LocationText string          `json:"location_text"`
	Latitude     *float64        `json:"latitude,omitempty"`
	Longitude    *float64        `json:"longitude,omitempty"`
	Interests    []string        `json:"interests"`
	Photos       []photoResponse `json:"photos"`
}

type updateRequest struct {
	DisplayName  string   `json:"display_name"`
	Bio          string   `json:"bio"`
	AvatarURL    string   `json:"avatar_url"`
	DateOfBirth  *string  `json:"date_of_birth"`
	Gender       string   `json:"gender"`
	LocationText string   `json:"location_text"`
	Latitude     *float64 `json:"latitude"`
	Longitude    *float64 `json:"longitude"`
	Interests    []string `json:"interests"`
}

type preferencesResponse struct {
	MinAge           *int     `json:"min_age"`
	MaxAge           *int     `json:"max_age"`
	MaxDistanceKm    *int     `json:"max_distance_km"`
	GenderPreference []string `json:"gender_preference"`
}

type updatePreferencesRequest struct {
	MinAge           *int     `json:"min_age"`
	MaxAge           *int     `json:"max_age"`
	MaxDistanceKm    *int     `json:"max_distance_km"`
	GenderPreference []string `json:"gender_preference"`
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
	mux.HandleFunc("PUT /profiles/me/avatar", updateAvatar(svc))
	mux.HandleFunc("GET /profiles/me/preferences", getMyPreferences(svc))
	mux.HandleFunc("PUT /profiles/me/preferences", updateMyPreferences(svc))
	mux.HandleFunc("POST /profiles/me/photos", addPhoto(svc))
	mux.HandleFunc("DELETE /profiles/me/photos/{photoID}", deletePhoto(svc))
	mux.HandleFunc("GET /profiles/interests", searchInterests(svc))
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

		in := ProfileInput{
			DisplayName:  req.DisplayName,
			Bio:          req.Bio,
			AvatarURL:    req.AvatarURL,
			Gender:       req.Gender,
			LocationText: req.LocationText,
			Latitude:     req.Latitude,
			Longitude:    req.Longitude,
			Interests:    req.Interests,
		}
		if req.DateOfBirth != nil {
			t, err := time.Parse("2006-01-02", *req.DateOfBirth)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, errorResponse{"invalid date_of_birth format, expected YYYY-MM-DD"})
				return
			}
			in.DateOfBirth = &t
		}

		profile, err := svc.UpdateMyProfile(r.Context(), userID, in)
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

func updateAvatar(svc ProfileManager) http.HandlerFunc {
	type request struct {
		AvatarURL string `json:"avatar_url"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errorResponse{"unauthorized"})
			return
		}
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{"invalid request body"})
			return
		}
		if err := svc.UpdateAvatar(r.Context(), userID, req.AvatarURL); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{"internal server error"})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func getMyPreferences(svc ProfileManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errorResponse{"unauthorized"})
			return
		}

		prefs, err := svc.GetMyPreferences(r.Context(), userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{"internal server error"})
			return
		}

		writeJSON(w, http.StatusOK, toPreferencesResponse(prefs))
	}
}

func updateMyPreferences(svc ProfileManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errorResponse{"unauthorized"})
			return
		}

		var req updatePreferencesRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{"invalid request body"})
			return
		}

		genderPref := req.GenderPreference
		if genderPref == nil {
			genderPref = []string{}
		}

		prefs, err := svc.UpdateMyPreferences(r.Context(), userID, ProfilePreferences{
			MinAge:           req.MinAge,
			MaxAge:           req.MaxAge,
			MaxDistanceKm:    req.MaxDistanceKm,
			GenderPreference: genderPref,
		})
		if err != nil {
			if errors.Is(err, ErrInvalidInput) {
				writeJSON(w, http.StatusBadRequest, errorResponse{err.Error()})
				return
			}
			writeJSON(w, http.StatusInternalServerError, errorResponse{"internal server error"})
			return
		}

		writeJSON(w, http.StatusOK, toPreferencesResponse(prefs))
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

func searchInterests(svc ProfileManager) http.HandlerFunc {
	type interestResponse struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		_, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errorResponse{"unauthorized"})
			return
		}

		query := r.URL.Query().Get("q")
		suggestions, err := svc.SearchInterests(r.Context(), query)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{"internal server error"})
			return
		}

		resp := make([]interestResponse, len(suggestions))
		for i, s := range suggestions {
			resp[i] = interestResponse{Name: s.Name, Count: s.Count}
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func toResponse(p *Profile) profileResponse {
	photos := make([]photoResponse, len(p.Photos))
	for i, ph := range p.Photos {
		photos[i] = photoResponse{ID: ph.ID, URL: ph.URL}
	}
	interests := p.Interests
	if interests == nil {
		interests = []string{}
	}
	resp := profileResponse{
		ID:           p.ID,
		UserID:       p.UserID,
		DisplayName:  p.DisplayName,
		Bio:          p.Bio,
		AvatarURL:    p.AvatarURL,
		Gender:       p.Gender,
		LocationText: p.LocationText,
		Latitude:     p.Latitude,
		Longitude:    p.Longitude,
		Interests:    interests,
		Photos:       photos,
	}
	if p.DateOfBirth != nil {
		s := p.DateOfBirth.Format("2006-01-02")
		resp.DateOfBirth = &s
	}
	return resp
}

func toPreferencesResponse(p *ProfilePreferences) preferencesResponse {
	genderPref := p.GenderPreference
	if genderPref == nil {
		genderPref = []string{}
	}
	return preferencesResponse{
		MinAge:           p.MinAge,
		MaxAge:           p.MaxAge,
		MaxDistanceKm:    p.MaxDistanceKm,
		GenderPreference: genderPref,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
