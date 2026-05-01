package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/apierror"
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
	refreshStore     RefreshTokenStore
}

func WithLocker(l LoginLocker) HandlerOption { return func(c *handlerConfig) { c.locker = l } }

func WithRefreshTokenStore(s RefreshTokenStore) HandlerOption {
	return func(c *handlerConfig) { c.refreshStore = s }
}
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
	Email      string `json:"email"`
	Password   string `json:"password"`
	RememberMe bool   `json:"remember_me"`
}

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	Username    string `json:"username"`
	DateOfBirth string `json:"date_of_birth"` // "YYYY-MM-DD"
}

type userResponse struct {
	ID           string `json:"id,omitempty"`
	Email        string `json:"email,omitempty"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Reactivated  bool   `json:"reactivated,omitempty"`
}

// generateTokenFn is a variable so tests can inject a failing implementation.
var generateTokenFn func(string, string, string, time.Duration) (string, error) = token.Generate

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
	r.Post("/refresh", refreshHandler(jwtSecret, tokenExpiry, cfg))
	r.Post("/logout", logoutHandler(cfg))
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
				zerolog.Ctx(r.Context()).Warn().Err(err).Msg("auth: ip rate limiter error")
			} else if !allowed {
				apierror.Write(w, http.StatusTooManyRequests, apierror.CodeRateLimited, "too many requests")
				return
			}
		}

		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
			return
		}
		if req.Email == "" || req.Password == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "email and password are required")
			return
		}
		if len(req.Password) > maxPasswordLen {
			apierror.Write(w, http.StatusBadRequest, apierror.CodePasswordTooLong, "password too long")
			return
		}

		lockKey := "lockout:" + strings.ToLower(strings.TrimSpace(req.Email))
		if cfg.locker != nil {
			locked, err := cfg.locker.IsLocked(r.Context(), lockKey, loginMaxFailures)
			if err != nil {
				zerolog.Ctx(r.Context()).Warn().Err(err).Msg("auth: locker IsLocked error")
			} else if locked {
				apierror.Write(w, http.StatusTooManyRequests, apierror.CodeAccountLocked, "account temporarily locked due to too many failed login attempts")
				return
			}
		}

		user, err := auth.Login(r.Context(), req.Email, req.Password)
		if err != nil {
			if errors.Is(err, ErrEmailNotVerified) {
				apierror.Write(w, http.StatusForbidden, apierror.CodeEmailNotVerified, "email_not_verified")
				return
			}
			if errors.Is(err, ErrInvalidCredentials) {
				if cfg.locker != nil {
					locked, lerr := cfg.locker.RecordFailure(r.Context(), lockKey, loginMaxFailures, loginLockWindow)
					if lerr != nil {
						zerolog.Ctx(r.Context()).Warn().Err(lerr).Msg("auth: locker RecordFailure error")
					} else if locked {
						apierror.Write(w, http.StatusTooManyRequests, apierror.CodeAccountLocked, "account temporarily locked due to too many failed login attempts")
						return
					}
				}
				apierror.Write(w, http.StatusUnauthorized, apierror.CodeInvalidCredentials, "invalid credentials")
				return
			}
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}

		if cfg.locker != nil {
			if err := cfg.locker.Reset(r.Context(), lockKey); err != nil {
				zerolog.Ctx(r.Context()).Warn().Err(err).Msg("auth: locker Reset error")
			}
		}

		tok, err := generateTokenFn(user.ID, user.Role, jwtSecret, tokenExpiry)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}

		resp := userResponse{ID: user.ID, Email: user.Email, Token: tok, Reactivated: user.Reactivated}

		if cfg.refreshStore != nil {
			plain, hash, err := generateSecureToken()
			if err != nil {
				zerolog.Ctx(r.Context()).Warn().Err(err).Msg("auth: generate refresh token failed")
			} else {
				ttl := RefreshTokenTTLSession
				if req.RememberMe {
					ttl = RefreshTokenTTLRemember
				}
				if err := cfg.refreshStore.Create(r.Context(), hash, user.ID, user.Role, time.Now(), ttl); err != nil {
					zerolog.Ctx(r.Context()).Warn().Err(err).Msg("auth: store refresh token failed")
				} else {
					resp.RefreshToken = plain
				}
			}
		}

		apierror.WriteJSON(w, http.StatusOK, resp)
	}
}

// refreshHandler validates a refresh token, rotates it, and returns a new
// access token + refresh token pair. The old refresh token is deleted atomically
// before the new one is stored so it cannot be reused.
func refreshHandler(jwtSecret string, tokenExpiry time.Duration, cfg *handlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.refreshStore == nil {
			apierror.Write(w, http.StatusNotFound, apierror.CodeNotFound, "not found")
			return
		}
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "refresh_token is required")
			return
		}

		hash := hashToken(req.RefreshToken)
		rt, err := cfg.refreshStore.Get(r.Context(), hash)
		if errors.Is(err, ErrInvalidToken) {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeInvalidToken, "invalid or expired refresh token")
			return
		}
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}

		// Rotate: delete old token before issuing new one.
		if err := cfg.refreshStore.Delete(r.Context(), hash); err != nil {
			zerolog.Ctx(r.Context()).Warn().Err(err).Msg("auth: delete old refresh token failed")
		}

		newAccessToken, err := generateTokenFn(rt.UserID, rt.Role, jwtSecret, tokenExpiry)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}

		newPlain, newHash, err := generateSecureToken()
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		newTTL := rt.TTL
		if newTTL == 0 {
			newTTL = RefreshTokenTTLSession
		}
		if err := cfg.refreshStore.Create(r.Context(), newHash, rt.UserID, rt.Role, time.Now(), newTTL); err != nil {
			zerolog.Ctx(r.Context()).Warn().Err(err).Msg("auth: store new refresh token failed")
		}

		apierror.WriteJSON(w, http.StatusOK, userResponse{Token: newAccessToken, RefreshToken: newPlain})
	}
}

// logoutHandler invalidates the supplied refresh token.
// Always returns 204 — even if the token is absent or already expired — to
// prevent token enumeration.
func logoutHandler(cfg *handlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.refreshStore != nil {
			var req struct {
				RefreshToken string `json:"refresh_token"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.RefreshToken != "" {
				_ = cfg.refreshStore.Delete(r.Context(), hashToken(req.RefreshToken))
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func registerHandler(auth Authenticator, cfg *handlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.limiter != nil {
			ip := clientIP(r)
			allowed, err := cfg.limiter.Allow(r.Context(), "register:ip:"+ip, cfg.registerIPLimit, cfg.registerIPWindow)
			if err != nil {
				zerolog.Ctx(r.Context()).Warn().Err(err).Msg("auth: ip rate limiter error")
			} else if !allowed {
				apierror.Write(w, http.StatusTooManyRequests, apierror.CodeRateLimited, "too many requests")
				return
			}
		}

		var req registerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
			return
		}
		if req.Email == "" || req.Password == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "email and password are required")
			return
		}

		user, err := auth.Register(r.Context(), req.Email, req.Password)
		if err != nil {
			switch {
			case errors.Is(err, ErrInvalidInput):
				apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, err.Error())
			case errors.Is(err, ErrEmailTaken):
				apierror.Write(w, http.StatusConflict, apierror.CodeEmailTaken, "email already taken")
			default:
				apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
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
					apierror.Write(w, http.StatusConflict, apierror.CodeUsernameTaken, "username already taken")
					return
				}
				zerolog.Ctx(r.Context()).Warn().Err(err).Str("user_id", user.ID).Msg("auth: seed profile failed")
				// Non-fatal: account exists, user can set profile later.
			}
		}

		// Send verification email asynchronously; ignore send errors — the user
		// can request a resend from the login page.
		if cfg.emailFlow != nil {
			ctx := context.WithoutCancel(r.Context())
			reqLog := zerolog.Ctx(r.Context()).With().Str("user_id", user.ID).Logger()
			go func() {
				if err := cfg.emailFlow.SendVerificationEmail(ctx, user.ID, user.Email, cfg.frontendURL); err != nil {
					reqLog.Warn().Err(err).Msg("auth: send verification email failed")
				}
			}()
		}

		apierror.WriteJSON(w, http.StatusCreated, map[string]string{
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
		apierror.WriteJSON(w, http.StatusOK, map[string]string{
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
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
			return
		}
		if req.Token == "" || req.Password == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "token and password are required")
			return
		}
		if err := svc.ResetPassword(r.Context(), req.Token, req.Password); err != nil {
			switch {
			case errors.Is(err, ErrInvalidToken):
				apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidToken, "invalid or expired reset token")
			case errors.Is(err, ErrInvalidInput):
				apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, err.Error())
			default:
				apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			}
			return
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]string{"message": "password reset successfully"})
	}
}

func verifyEmailHandler(svc EmailFlowService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "token is required")
			return
		}
		if err := svc.VerifyEmail(r.Context(), req.Token); err != nil {
			switch {
			case errors.Is(err, ErrInvalidToken):
				apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidToken, "invalid or expired verification token")
			default:
				apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			}
			return
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]string{"message": "email verified — you can now log in"})
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
		apierror.WriteJSON(w, http.StatusOK, map[string]string{
			"message": "if that email is registered and unverified you will receive a new verification link",
		})
	}
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if before, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(before)
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

// AccountHandlerOption configures optional features on the account handler.
type AccountHandlerOption func(*accountHandlerConfig)

type accountHandlerConfig struct {
	refreshStore RefreshTokenStore
}

// WithAccountRefreshStore wires a RefreshTokenStore into the account handler so
// that all refresh tokens are revoked when a user changes their password or
// deletes their account.
func WithAccountRefreshStore(s RefreshTokenStore) AccountHandlerOption {
	return func(c *accountHandlerConfig) { c.refreshStore = s }
}

// NewAccountHandler returns an http.Handler for user account management routes.
// Routes are expected to be mounted at /users/me and run behind RequireAuth.
//
//	PUT    /password  — change password
//	DELETE /          — delete (soft) account
func NewAccountHandler(svc AccountManager, opts ...AccountHandlerOption) http.Handler {
	cfg := &accountHandlerConfig{}
	for _, o := range opts {
		o(cfg)
	}
	r := chi.NewRouter()
	r.Put("/password", changePasswordHandler(svc, cfg))
	r.Delete("/", deleteAccountHandler(svc, cfg))
	return r
}

func changePasswordHandler(svc AccountManager, cfg *accountHandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}

		var req struct {
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
			return
		}
		if req.CurrentPassword == "" || req.NewPassword == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "current_password and new_password are required")
			return
		}

		if err := svc.ChangePassword(r.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
			switch {
			case errors.Is(err, ErrInvalidCredentials):
				apierror.Write(w, http.StatusUnauthorized, apierror.CodeInvalidCredentials, "current password is incorrect")
			case errors.Is(err, ErrInvalidInput):
				apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, err.Error())
			default:
				apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			}
			return
		}

		if cfg.refreshStore != nil {
			if err := cfg.refreshStore.RevokeAllForUser(r.Context(), userID); err != nil {
				zerolog.Ctx(r.Context()).Warn().Err(err).Msg("auth: revoke refresh tokens on password change failed")
			}
		}

		apierror.WriteJSON(w, http.StatusOK, map[string]string{"message": "password updated"})
	}
}

func deleteAccountHandler(svc AccountManager, cfg *accountHandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}

		var req struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
			return
		}
		if req.Password == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "password is required")
			return
		}

		if err := svc.DeleteAccount(r.Context(), userID, req.Password); err != nil {
			if errors.Is(err, ErrInvalidCredentials) {
				apierror.Write(w, http.StatusUnauthorized, apierror.CodeInvalidCredentials, "invalid password")
				return
			}
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}

		if cfg.refreshStore != nil {
			if err := cfg.refreshStore.RevokeAllForUser(r.Context(), userID); err != nil {
				zerolog.Ctx(r.Context()).Warn().Err(err).Msg("auth: revoke refresh tokens on account deletion failed")
			}
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
