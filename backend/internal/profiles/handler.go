package profiles

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/mayloo89/circl/backend/internal/middleware"
)

// ProfileManager is the interface the handler depends on.
type ProfileManager interface {
	GetMyProfile(ctx context.Context, userID string) (*Profile, error)
	UpdateMyProfile(ctx context.Context, userID string, in ProfileInput) (*Profile, error)
	UpdateAvatar(ctx context.Context, userID, avatarURL string) error
	GetPublicProfile(ctx context.Context, userID string) (*Profile, error)
	GetPublicProfileByUsername(ctx context.Context, username string) (*Profile, error)
	IsUsernameAvailable(ctx context.Context, username string) (bool, error)
	AddPhoto(ctx context.Context, userID, url string) (*ProfilePhoto, error)
	DeletePhoto(ctx context.Context, userID, photoID string) error
	GetMyPreferences(ctx context.Context, userID string) (*ProfilePreferences, error)
	UpdateMyPreferences(ctx context.Context, userID string, prefs ProfilePreferences) (*ProfilePreferences, error)
	SearchInterests(ctx context.Context, query string) ([]InterestSuggestion, error)
	Browse(ctx context.Context, userID string, limit int, cursor string, sortByDistance bool, interests []string) (*BrowsePage, error)
}

type photoResponse struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type profileResponse struct {
	ID           string          `json:"id"`
	UserID       string          `json:"user_id"`
	Username     string          `json:"username"`
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
	Username     string   `json:"username"`
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
	mux.HandleFunc("GET /profiles/browse", browseProfiles(svc))
	mux.HandleFunc("GET /profiles/interests", searchInterests(svc))
	mux.HandleFunc("GET /profiles/available", checkUsernameAvailable(svc))
	mux.HandleFunc("GET /profiles/{ref}", getPublicProfileByRef(svc))
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
			Username:     req.Username,
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
			if errors.Is(err, ErrUsernameTaken) {
				writeJSON(w, http.StatusConflict, errorResponse{"username already taken"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, errorResponse{"internal server error"})
			return
		}

		writeJSON(w, http.StatusOK, toResponse(profile))
	}
}

// getPublicProfileByRef serves GET /profiles/{ref} where ref is either a UUID or a username.
func getPublicProfileByRef(svc ProfileManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errorResponse{"unauthorized"})
			return
		}

		ref := r.PathValue("ref")
		var (
			profile *Profile
			err     error
		)
		if isUUID(ref) {
			profile, err = svc.GetPublicProfile(r.Context(), ref)
		} else {
			profile, err = svc.GetPublicProfileByUsername(r.Context(), ref)
		}
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

// isUUID returns true if s looks like a UUID (8-4-4-4-12 hex format).
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func checkUsernameAvailable(svc ProfileManager) http.HandlerFunc {
	type availableResponse struct {
		Available bool `json:"available"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		_, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errorResponse{"unauthorized"})
			return
		}
		username := r.URL.Query().Get("username")
		if username == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{"username query parameter is required"})
			return
		}
		available, err := svc.IsUsernameAvailable(r.Context(), username)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{"internal server error"})
			return
		}
		writeJSON(w, http.StatusOK, availableResponse{Available: available})
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
		Username:     p.Username,
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

type browseProfileResponse struct {
	ID            string   `json:"id"`
	UserID        string   `json:"user_id"`
	Username      string   `json:"username"`
	DisplayName   string   `json:"display_name"`
	AvatarURL     string   `json:"avatar_url"`
	Age           *int     `json:"age,omitempty"`
	Gender        string   `json:"gender"`
	LocationText  string   `json:"location_text"`
	DistanceKm    *float64 `json:"distance_km"`
	FirstPhotoURL string   `json:"first_photo_url"`
	Interests     []string `json:"interests"`
}

type browsePageResponse struct {
	Profiles   []browseProfileResponse `json:"profiles"`
	NextCursor string                  `json:"next_cursor"`
	Limit      int                     `json:"limit"`
}

func browseProfiles(svc ProfileManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errorResponse{"unauthorized"})
			return
		}

		limit := 20
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 50 {
				limit = n
			}
		}
		cursor := r.URL.Query().Get("cursor")
		sortByDistance := r.URL.Query().Get("sort") == "distance"
		interests := r.URL.Query()["interests"]

		result, err := svc.Browse(r.Context(), userID, limit, cursor, sortByDistance, interests)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{"internal server error"})
			return
		}

		resp := make([]browseProfileResponse, len(result.Profiles))
		for i, p := range result.Profiles {
			pInterests := p.Interests
			if pInterests == nil {
				pInterests = []string{}
			}
			resp[i] = browseProfileResponse{
				ID:            p.ID,
				UserID:        p.UserID,
				Username:      p.Username,
				DisplayName:   p.DisplayName,
				AvatarURL:     p.AvatarURL,
				Age:           p.Age,
				Gender:        p.Gender,
				LocationText:  p.LocationText,
				DistanceKm:    p.DistanceKm,
				FirstPhotoURL: p.FirstPhotoURL,
				Interests:     pInterests,
			}
		}
		writeJSON(w, http.StatusOK, browsePageResponse{
			Profiles:   resp,
			NextCursor: result.NextCursor,
			Limit:      limit,
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
