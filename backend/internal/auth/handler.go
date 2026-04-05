package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/token"
)

const (
	// loginMaxFailures is the number of consecutive failures before an account is locked.
	loginMaxFailures = 10
	// loginLockWindow is how long the lockout lasts after the threshold is reached.
	loginLockWindow = 15 * time.Minute

	// Default per-IP rate limit values. Override via WithLoginIPLimit / WithRegisterIPLimit.
	defaultLoginIPLimit     = 20
	defaultLoginIPWindow    = 15 * time.Minute
	defaultRegisterIPLimit  = 10
	defaultRegisterIPWindow = time.Hour
)

// LoginLocker tracks consecutive login failures per account and enforces lockouts.
type LoginLocker interface {
	// IsLocked returns true if the account key has reached the failure limit.
	// It does not modify the counter.
	IsLocked(ctx context.Context, key string, limit int) (bool, error)
	// RecordFailure increments the failure counter for key and returns true if
	// the account is now locked. The window TTL is anchored to the first failure.
	RecordFailure(ctx context.Context, key string, limit int, window time.Duration) (locked bool, err error)
	// Reset clears the failure counter after a successful login.
	Reset(ctx context.Context, key string) error
}

// RequestLimiter enforces per-IP rate limits on sensitive endpoints.
type RequestLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

// HandlerOption configures optional security features on the auth handler.
type HandlerOption func(*handlerConfig)

type handlerConfig struct {
	locker           LoginLocker
	limiter          RequestLimiter
	loginIPLimit     int
	loginIPWindow    time.Duration
	registerIPLimit  int
	registerIPWindow time.Duration
}

// WithLocker injects a LoginLocker for account lockout enforcement.
func WithLocker(l LoginLocker) HandlerOption { return func(c *handlerConfig) { c.locker = l } }

// WithLimiter injects a RequestLimiter for per-IP rate limiting.
func WithLimiter(l RequestLimiter) HandlerOption { return func(c *handlerConfig) { c.limiter = l } }

// WithLoginIPLimit overrides the default per-IP rate limit for the login endpoint.
func WithLoginIPLimit(limit int, window time.Duration) HandlerOption {
	return func(c *handlerConfig) { c.loginIPLimit = limit; c.loginIPWindow = window }
}

// WithRegisterIPLimit overrides the default per-IP rate limit for the register endpoint.
func WithRegisterIPLimit(limit int, window time.Duration) HandlerOption {
	return func(c *handlerConfig) { c.registerIPLimit = limit; c.registerIPWindow = window }
}

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
var generateTokenFn func(string, bool, string, time.Duration) (string, error) = token.Generate

// NewHandler returns an http.Handler with all auth routes registered.
// jwtSecret and tokenExpiry are used to issue a signed JWT on login/register.
func NewHandler(auth Authenticator, jwtSecret string, tokenExpiry time.Duration, opts ...HandlerOption) http.Handler {
	cfg := &handlerConfig{
		loginIPLimit:     defaultLoginIPLimit,
		loginIPWindow:    defaultLoginIPWindow,
		registerIPLimit:  defaultRegisterIPLimit,
		registerIPWindow: defaultRegisterIPWindow,
	}
	for _, o := range opts {
		o(cfg)
	}
	r := chi.NewRouter()
	r.Post("/login", loginHandler(auth, jwtSecret, tokenExpiry, cfg))
	r.Post("/register", registerHandler(auth, jwtSecret, tokenExpiry, cfg))
	return r
}

func loginHandler(auth Authenticator, jwtSecret string, tokenExpiry time.Duration, cfg *handlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Per-IP rate limit — checked before decoding to fail fast on floods.
		if cfg.limiter != nil {
			ip := clientIP(r)
			allowed, err := cfg.limiter.Allow(r.Context(), "login:ip:"+ip, cfg.loginIPLimit, cfg.loginIPWindow)
			if err != nil {
				log.Printf("auth: ip rate limiter error: %v", err)
			} else if !allowed {
				writeJSON(w, http.StatusTooManyRequests, errorResponse{"too many requests"})
				return
			}
		}

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

		// Account lockout check — before running bcrypt to avoid wasted CPU.
		lockKey := "lockout:" + strings.ToLower(strings.TrimSpace(req.Email))
		if cfg.locker != nil {
			locked, err := cfg.locker.IsLocked(r.Context(), lockKey, loginMaxFailures)
			if err != nil {
				log.Printf("auth: locker IsLocked error: %v", err)
			} else if locked {
				writeJSON(w, http.StatusTooManyRequests, errorResponse{"account temporarily locked due to too many failed login attempts"})
				return
			}
		}

		user, err := auth.Login(r.Context(), req.Email, req.Password)
		if err != nil {
			if errors.Is(err, ErrInvalidCredentials) {
				if cfg.locker != nil {
					locked, lerr := cfg.locker.RecordFailure(r.Context(), lockKey, loginMaxFailures, loginLockWindow)
					if lerr != nil {
						log.Printf("auth: locker RecordFailure error: %v", lerr)
					} else if locked {
						writeJSON(w, http.StatusTooManyRequests, errorResponse{"account temporarily locked due to too many failed login attempts"})
						return
					}
				}
				writeJSON(w, http.StatusUnauthorized, errorResponse{"invalid credentials"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, errorResponse{"internal server error"})
			return
		}

		// Successful login — reset the failure counter.
		if cfg.locker != nil {
			if err := cfg.locker.Reset(r.Context(), lockKey); err != nil {
				log.Printf("auth: locker Reset error: %v", err)
			}
		}

		tok, err := generateTokenFn(user.ID, user.IsAdmin, jwtSecret, tokenExpiry)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{"internal server error"})
			return
		}

		writeJSON(w, http.StatusOK, userResponse{ID: user.ID, Email: user.Email, Token: tok})
	}
}

func registerHandler(auth Authenticator, jwtSecret string, tokenExpiry time.Duration, cfg *handlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Per-IP rate limit on registration to slow down mass account creation.
		if cfg.limiter != nil {
			ip := clientIP(r)
			allowed, err := cfg.limiter.Allow(r.Context(), "register:ip:"+ip, cfg.registerIPLimit, cfg.registerIPWindow)
			if err != nil {
				log.Printf("auth: ip rate limiter error: %v", err)
			} else if !allowed {
				writeJSON(w, http.StatusTooManyRequests, errorResponse{"too many requests"})
				return
			}
		}

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

		tok, err := generateTokenFn(user.ID, user.IsAdmin, jwtSecret, tokenExpiry)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{"internal server error"})
			return
		}

		writeJSON(w, http.StatusCreated, userResponse{ID: user.ID, Email: user.Email, Token: tok})
	}
}

// clientIP extracts the originating client IP from the request.
// It honours X-Forwarded-For (leftmost entry) and X-Real-IP headers set by
// trusted reverse proxies, falling back to r.RemoteAddr.
// NOTE: X-Forwarded-For can be spoofed if no trusted proxy strips it first;
// ensure your proxy configuration removes untrusted values in production.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// NewAccountHandler returns an http.Handler for user account management routes.
// Routes are expected to be mounted at /users/me and run behind RequireAuth.
//
//	PUT  /password  — change password
//	DELETE /        — delete (soft) account
func NewAccountHandler(svc AccountManager) http.Handler {
	r := chi.NewRouter()
	r.Put("/password", changePasswordHandler(svc))
	r.Delete("/", deleteAccountHandler(svc))
	return r
}

func changePasswordHandler(svc AccountManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errorResponse{"unauthorized"})
			return
		}

		var req struct {
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{"invalid request body"})
			return
		}
		if req.CurrentPassword == "" || req.NewPassword == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{"current_password and new_password are required"})
			return
		}

		if err := svc.ChangePassword(r.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
			switch {
			case errors.Is(err, ErrInvalidCredentials):
				writeJSON(w, http.StatusUnauthorized, errorResponse{"current password is incorrect"})
			case errors.Is(err, ErrInvalidInput):
				writeJSON(w, http.StatusBadRequest, errorResponse{err.Error()})
			default:
				writeJSON(w, http.StatusInternalServerError, errorResponse{"internal server error"})
			}
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"message": "password updated"})
	}
}

func deleteAccountHandler(svc AccountManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errorResponse{"unauthorized"})
			return
		}

		var req struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{"invalid request body"})
			return
		}
		if req.Password == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{"password is required"})
			return
		}

		if err := svc.DeleteAccount(r.Context(), userID, req.Password); err != nil {
			if errors.Is(err, ErrInvalidCredentials) {
				writeJSON(w, http.StatusUnauthorized, errorResponse{"invalid password"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, errorResponse{"internal server error"})
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
