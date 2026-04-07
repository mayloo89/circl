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
	"github.com/mayloo89/circl/backend/internal/profiles"
	"github.com/mayloo89/circl/backend/internal/token"
)

const (
	loginMaxFailures = 10
	loginLockWindow  = 15 * time.Minute

	defaultLoginIPLimit     = 20
	defaultLoginIPWindow    = 15 * time.Minute
	defaultRegisterIPLimit  = 10
	defaultRegisterIPWindow = time.Hour
)

// LoginLocker tracks consecutive login failures per account and enforces lockouts.
type LoginLocker interface {
	IsLocked(ctx context.Context, key string, limit int) (bool, error)
	RecordFailure(ctx context.Context, key string, limit int, window time.Duration) (locked bool, err error)
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
	emailFlow        EmailFlowService
	frontendURL      string
	profileStore     profiles.Store
}

func WithLocker(l LoginLocker) HandlerOption { return func(c *handlerConfig) { c.locker = l } }
func WithLimiter(l RequestLimiter) HandlerOption {
	return func(c *handlerConfig) { c.limiter = l }
}

func WithLoginIPLimit(limit int, window time.Duration) HandlerOption {
	return func(c *handlerConfig) { c.loginIPLimit = limit; c.loginIPWindow = window }
}

func WithRegisterIPLimit(limit int, window time.Duration) HandlerOption {
	return func(c *handlerConfig) { c.registerIPLimit = limit; c.registerIPWindow = window }
}

// WithEmailFlow enables the email-based auth routes (forgot-password, reset-password,
// verify-email, resend-verification) and wires up verification on registration.
func WithEmailFlow(svc EmailFlowService, frontendURL string) HandlerOption {
	return func(c *handlerConfig) { c.emailFlow = svc; c.frontendURL = frontendURL }
}

// WithProfileStore enables profile seeding at registration time (username + DOB).
func WithProfileStore(s profiles.Store) HandlerOption {
	return func(c *handlerConfig) { c.profileStore = s }
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	Username    string `json:"username"`
	DateOfBirth string `json:"date_of_birth"` // "YYYY-MM-DD"
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
	r.Post("/register", registerHandler(auth, cfg))
	if cfg.emailFlow != nil {
		r.Post("/forgot-password", forgotPasswordHandler(cfg.emailFlow, cfg.frontendURL))
		r.Post("/reset-password", resetPasswordHandler(cfg.emailFlow))
		r.Post("/verify-email", verifyEmailHandler(cfg.emailFlow))
		r.Post("/resend-verification", resendVerificationHandler(cfg.emailFlow, cfg.frontendURL))
	}
	return r
}

func loginHandler(auth Authenticator, jwtSecret string, tokenExpiry time.Duration, cfg *handlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
			if errors.Is(err, ErrEmailNotVerified) {
				writeJSON(w, http.StatusForbidden, errorResponse{"email_not_verified"})
				return
			}
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

func registerHandler(auth Authenticator, cfg *handlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		var req registerRequest
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

		// Seed the profile with username and date of birth if a profile store is configured.
		if cfg.profileStore != nil && (req.Username != "" || req.DateOfBirth != "") {
			in := profiles.ProfileInput{Username: req.Username}
			if req.DateOfBirth != "" {
				if dob, err := time.Parse("2006-01-02", req.DateOfBirth); err == nil {
					in.DateOfBirth = &dob
				}
			}
			if _, err := cfg.profileStore.Upsert(r.Context(), user.ID, in); err != nil {
				if errors.Is(err, profiles.ErrUsernameTaken) {
					// User was created but the username is taken — roll back user creation
					// is not trivial here, so return a conflict; the user must retry with a
					// different username. The unverified account will remain but is inert.
					writeJSON(w, http.StatusConflict, errorResponse{"username already taken"})
					return
				}
				log.Printf("auth: seed profile for user %s: %v", user.ID, err)
				// Non-fatal: account exists, user can set profile later.
			}
		}

		// Send verification email asynchronously; ignore send errors — the user
		// can request a resend from the login page.
		if cfg.emailFlow != nil {
			go func() {
				if err := cfg.emailFlow.SendVerificationEmail(r.Context(), user.ID, user.Email, cfg.frontendURL); err != nil {
					log.Printf("auth: send verification email: %v", err)
				}
			}()
		}

		writeJSON(w, http.StatusCreated, map[string]string{
			"message": "account created — check your email to verify your address before logging in",
		})
	}
}

func forgotPasswordHandler(svc EmailFlowService, frontendURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email string `json:"email"`
		}
		// Silently ignore decode errors — always respond with 200.
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Email != "" {
			_ = svc.ForgotPassword(r.Context(), req.Email, frontendURL)
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"message": "if that email is registered you will receive a password reset link",
		})
	}
}

func resetPasswordHandler(svc EmailFlowService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Token    string `json:"token"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{"invalid request body"})
			return
		}
		if req.Token == "" || req.Password == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{"token and password are required"})
			return
		}
		if err := svc.ResetPassword(r.Context(), req.Token, req.Password); err != nil {
			switch {
			case errors.Is(err, ErrInvalidToken):
				writeJSON(w, http.StatusBadRequest, errorResponse{"invalid or expired reset token"})
			case errors.Is(err, ErrInvalidInput):
				writeJSON(w, http.StatusBadRequest, errorResponse{err.Error()})
			default:
				writeJSON(w, http.StatusInternalServerError, errorResponse{"internal server error"})
			}
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "password reset successfully"})
	}
}

func verifyEmailHandler(svc EmailFlowService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{"token is required"})
			return
		}
		if err := svc.VerifyEmail(r.Context(), req.Token); err != nil {
			switch {
			case errors.Is(err, ErrInvalidToken):
				writeJSON(w, http.StatusBadRequest, errorResponse{"invalid or expired verification token"})
			default:
				writeJSON(w, http.StatusInternalServerError, errorResponse{"internal server error"})
			}
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "email verified — you can now log in"})
	}
}

func resendVerificationHandler(svc EmailFlowService, frontendURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email string `json:"email"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Email != "" {
			_ = svc.ResendVerification(r.Context(), req.Email, frontendURL)
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"message": "if that email is registered and unverified you will receive a new verification link",
		})
	}
}

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

// AccountHandlerOption configures optional features on the account handler.
type AccountHandlerOption func(*accountHandlerConfig)

type accountHandlerConfig struct{}

// NewAccountHandler returns an http.Handler for user account management routes.
// Routes are expected to be mounted at /users/me and run behind RequireAuth.
//
//	PUT    /password  — change password
//	DELETE /          — delete (soft) account
func NewAccountHandler(svc AccountManager, _ ...AccountHandlerOption) http.Handler {
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
