package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mayloo89/circl/backend/internal/token"
)

// Authenticator is the interface the handler depends on.
// *Service satisfies this interface.
type Authenticator interface {
	Login(ctx context.Context, email, password string) (*User, error)
	Register(ctx context.Context, email, password string) (*User, error)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Token string `json:"token"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// generateTokenFn is a variable so tests can inject a failing implementation.
var generateTokenFn = token.Generate

// NewHandler returns an http.Handler with all auth routes registered.
// jwtSecret and tokenExpiry are used to issue a signed JWT on login/register.
func NewHandler(auth Authenticator, jwtSecret string, tokenExpiry time.Duration) http.Handler {
	r := chi.NewRouter()
	r.Post("/login", loginHandler(auth, jwtSecret, tokenExpiry))
	r.Post("/register", registerHandler(auth, jwtSecret, tokenExpiry))
	return r
}

func loginHandler(auth Authenticator, jwtSecret string, tokenExpiry time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{"invalid request body"})
			return
		}

		if req.Email == "" || req.Password == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{"email and password are required"})
			return
		}

		if len(req.Password) > maxPasswordLen {
			writeJSON(w, http.StatusBadRequest, errorResponse{"password too long"})
			return
		}

		user, err := auth.Login(r.Context(), req.Email, req.Password)
		if err != nil {
			if errors.Is(err, ErrInvalidCredentials) {
				writeJSON(w, http.StatusUnauthorized, errorResponse{"invalid credentials"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, errorResponse{"internal server error"})
			return
		}

		tok, err := generateTokenFn(user.ID, jwtSecret, tokenExpiry)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{"internal server error"})
			return
		}

		writeJSON(w, http.StatusOK, userResponse{ID: user.ID, Email: user.Email, Token: tok})
	}
}

func registerHandler(auth Authenticator, jwtSecret string, tokenExpiry time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{"invalid request body"})
			return
		}

		if req.Email == "" || req.Password == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{"email and password are required"})
			return
		}

		user, err := auth.Register(r.Context(), req.Email, req.Password)
		if err != nil {
			switch {
			case errors.Is(err, ErrInvalidInput):
				writeJSON(w, http.StatusBadRequest, errorResponse{err.Error()})
			case errors.Is(err, ErrEmailTaken):
				writeJSON(w, http.StatusConflict, errorResponse{"email already taken"})
			default:
				writeJSON(w, http.StatusInternalServerError, errorResponse{"internal server error"})
			}
			return
		}

		tok, err := generateTokenFn(user.ID, jwtSecret, tokenExpiry)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{"internal server error"})
			return
		}

		writeJSON(w, http.StatusCreated, userResponse{ID: user.ID, Email: user.Email, Token: tok})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
