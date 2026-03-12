package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

const (
	maxPasswordLen = 128 // prevent bcrypt DoS via oversized input
)

// Authenticator is the interface the handler depends on.
// *Service satisfies this interface.
type Authenticator interface {
	Login(ctx context.Context, email, password string) (*User, error)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// NewHandler returns an http.Handler with all auth routes registered.
func NewHandler(auth Authenticator) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/login", loginHandler(auth))
	return mux
}

func loginHandler(auth Authenticator) http.HandlerFunc {
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

		writeJSON(w, http.StatusOK, loginResponse{ID: user.ID, Email: user.Email})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
