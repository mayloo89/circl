package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mayloo89/circl/backend/internal/auth"
	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/profiles"
)

const testUserID = "test-user-uuid"

const (
	testSecret = "supersecretfortesting-mustbe32chars!!"
	testExpiry = time.Hour
)

// mockAuth is a test double for Authenticator.
type mockAuth struct {
	user        *auth.User
	loginErr    error
	registerErr error
}

func (m *mockAuth) Login(_ context.Context, _, _ string) (*auth.User, error) {
	return m.user, m.loginErr
}

func (m *mockAuth) Register(_ context.Context, _, _ string) (*auth.User, error) {
	return m.user, m.registerErr
}

// mockLocker is a test double for LoginLocker.
type mockLocker struct {
	isLockedVal    bool
	isLockedErr    error
	recordLocked   bool
	recordErr      error
	resetErr       error
	resetCalled    bool
}

func (m *mockLocker) IsLocked(_ context.Context, _ string, _ int) (bool, error) {
	return m.isLockedVal, m.isLockedErr
}

func (m *mockLocker) RecordFailure(_ context.Context, _ string, _ int, _ time.Duration) (bool, error) {
	return m.recordLocked, m.recordErr
}

func (m *mockLocker) Reset(_ context.Context, _ string) error {
	m.resetCalled = true
	return m.resetErr
}

// mockLimiter is a test double for RequestLimiter.
type mockLimiter struct {
	allowed bool
	err     error
}

func (m *mockLimiter) Allow(_ context.Context, _ string, _ int, _ time.Duration) (bool, error) {
	return m.allowed, m.err
}

func newHandler(mock *mockAuth, opts ...auth.HandlerOption) http.Handler {
	return auth.NewHandler(mock, testSecret, testExpiry, opts...)
}

// --- Login handler ---

func TestLoginHandler_Success(t *testing.T) {
	h := newHandler(&mockAuth{user: &auth.User{ID: "abc-123", Email: "user@example.com"}})

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"user@example.com","password":"secret"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.Bytes()
	assertJSONField(t, body, "id", "abc-123")
	assertJSONField(t, body, "email", "user@example.com")
	assertJSONFieldNonEmpty(t, body, "token")
}

func TestLoginHandler_InvalidCredentials(t *testing.T) {
	h := newHandler(&mockAuth{loginErr: auth.ErrInvalidCredentials})

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"user@example.com","password":"wrong"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLoginHandler_InternalError(t *testing.T) {
	h := newHandler(&mockAuth{loginErr: errors.New("unexpected db error")})

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"user@example.com","password":"secret"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestLoginHandler_MalformedJSON(t *testing.T) {
	h := newHandler(&mockAuth{})

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("{not json"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLoginHandler_MissingFields(t *testing.T) {
	h := newHandler(&mockAuth{})

	for _, body := range []string{
		`{"email":"","password":"secret"}`,
		`{"email":"user@example.com","password":""}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %s: status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestLoginHandler_PasswordTooLong(t *testing.T) {
	h := newHandler(&mockAuth{})

	body, _ := json.Marshal(map[string]string{
		"email":    "user@example.com",
		"password": strings.Repeat("a", 129),
	})
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLoginHandler_AccountReactivated(t *testing.T) {
	h := newHandler(&mockAuth{user: &auth.User{ID: "1", Email: "u@u.com", Reactivated: true}})

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"u@u.com","password":"pass"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp["reactivated"] != true {
		t.Errorf("reactivated = %v, want true", resp["reactivated"])
	}
	if tok, _ := resp["token"].(string); tok == "" {
		t.Error("expected non-empty token")
	}
}

func TestLoginHandler_ContentType(t *testing.T) {
	h := newHandler(&mockAuth{user: &auth.User{ID: "1", Email: "u@u.com"}})

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"u@u.com","password":"pass1234"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

// --- Register handler ---

// mockProfileStore is a minimal test double for profiles.Store used in register tests.
type mockProfileStore struct {
	upsertErr error
}

func (m *mockProfileStore) Upsert(_ context.Context, _ string, _ profiles.ProfileInput) (*profiles.Profile, error) {
	return &profiles.Profile{}, m.upsertErr
}
func (m *mockProfileStore) GetByUserID(_ context.Context, _ string) (*profiles.Profile, error) {
	return nil, nil
}
func (m *mockProfileStore) GetByUsername(_ context.Context, _ string) (*profiles.Profile, error) {
	return nil, nil
}
func (m *mockProfileStore) IsUsernameAvailable(_ context.Context, _ string) (bool, error) {
	return true, nil
}
func (m *mockProfileStore) SyncInterests(_ context.Context, _ string, _ []string) error { return nil }
func (m *mockProfileStore) UpdateAvatar(_ context.Context, _, _ string) error           { return nil }
func (m *mockProfileStore) GetPhotosByUserID(_ context.Context, _ string) ([]profiles.ProfilePhoto, error) {
	return nil, nil
}
func (m *mockProfileStore) CountPhotos(_ context.Context, _ string) (int, error) { return 0, nil }
func (m *mockProfileStore) AddPhoto(_ context.Context, _, _ string) (*profiles.ProfilePhoto, error) {
	return nil, nil
}
func (m *mockProfileStore) DeletePhoto(_ context.Context, _, _ string) error { return nil }
func (m *mockProfileStore) GetPreferences(_ context.Context, _ string) (*profiles.ProfilePreferences, error) {
	return nil, nil
}
func (m *mockProfileStore) UpsertPreferences(_ context.Context, _ string, _ profiles.PreferencesUpdate) (*profiles.ProfilePreferences, error) {
	return nil, nil
}
func (m *mockProfileStore) SearchInterests(_ context.Context, _ string, _ int) ([]profiles.InterestSuggestion, error) {
	return nil, nil
}
func (m *mockProfileStore) Browse(_ context.Context, _ string, _ int, _ string, _ bool, _ []string) ([]profiles.BrowseProfile, error) {
	return nil, nil
}
func (m *mockProfileStore) GetPrivacyFlagsByIDs(_ context.Context, _ []string) (map[string]profiles.PrivacyFlags, error) {
	return map[string]profiles.PrivacyFlags{}, nil
}
func (m *mockProfileStore) AcceptedContactIDs(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

func TestRegisterHandler_Success(t *testing.T) {
	h := newHandler(&mockAuth{user: &auth.User{ID: "new-uuid", Email: "new@example.com"}})

	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{"email":"new@example.com","password":"securepass"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	assertJSONFieldNonEmpty(t, rec.Body.Bytes(), "message")
}

func TestRegisterHandler_UsernameTaken(t *testing.T) {
	ps := &mockProfileStore{upsertErr: profiles.ErrUsernameTaken}
	h := newHandler(
		&mockAuth{user: &auth.User{ID: "new-uuid", Email: "new@example.com"}},
		auth.WithProfileStore(ps),
	)

	body := `{"email":"new@example.com","password":"securepass","username":"taken_user","date_of_birth":"1990-01-01"}`
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestRegisterHandler_EmailTaken(t *testing.T) {
	h := newHandler(&mockAuth{registerErr: auth.ErrEmailTaken})

	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{"email":"taken@example.com","password":"securepass"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestRegisterHandler_InvalidInput(t *testing.T) {
	h := newHandler(&mockAuth{registerErr: auth.ErrInvalidInput})

	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{"email":"bad","password":"short"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRegisterHandler_InternalError(t *testing.T) {
	h := newHandler(&mockAuth{registerErr: errors.New("unexpected error")})

	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{"email":"user@example.com","password":"securepass"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestRegisterHandler_MalformedJSON(t *testing.T) {
	h := newHandler(&mockAuth{})

	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader("{not json"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRegisterHandler_MissingFields(t *testing.T) {
	h := newHandler(&mockAuth{})

	for _, body := range []string{
		`{"email":"","password":"securepass"}`,
		`{"email":"user@example.com","password":""}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %s: status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}

// --- Login lockout ---

func TestLoginHandler_AlreadyLocked(t *testing.T) {
	locker := &mockLocker{isLockedVal: true}
	h := newHandler(&mockAuth{}, auth.WithLocker(locker))

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"u@u.com","password":"secret"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestLoginHandler_LockoutOnFailingAttempt(t *testing.T) {
	locker := &mockLocker{recordLocked: true}
	h := newHandler(&mockAuth{loginErr: auth.ErrInvalidCredentials}, auth.WithLocker(locker))

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"u@u.com","password":"wrong"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestLoginHandler_FailureBeforeLockout(t *testing.T) {
	locker := &mockLocker{recordLocked: false}
	h := newHandler(&mockAuth{loginErr: auth.ErrInvalidCredentials}, auth.WithLocker(locker))

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"u@u.com","password":"wrong"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLoginHandler_SuccessResetsLockout(t *testing.T) {
	locker := &mockLocker{}
	h := newHandler(&mockAuth{user: &auth.User{ID: "1", Email: "u@u.com"}}, auth.WithLocker(locker))

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"u@u.com","password":"secret"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !locker.resetCalled {
		t.Error("expected locker.Reset to be called on successful login")
	}
}

func TestLoginHandler_LockerIsLockedError(t *testing.T) {
	// When IsLocked returns an error, the handler logs and continues (does not block).
	locker := &mockLocker{isLockedErr: errors.New("redis down"), isLockedVal: false}
	h := newHandler(&mockAuth{user: &auth.User{ID: "1", Email: "u@u.com"}}, auth.WithLocker(locker))

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"u@u.com","password":"secret"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (locker error must not block login)", rec.Code, http.StatusOK)
	}
}

// --- IP rate limiting ---

func TestLoginHandler_IPRateLimitExceeded(t *testing.T) {
	limiter := &mockLimiter{allowed: false}
	h := newHandler(&mockAuth{}, auth.WithLimiter(limiter))

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"u@u.com","password":"secret"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestRegisterHandler_IPRateLimitExceeded(t *testing.T) {
	limiter := &mockLimiter{allowed: false}
	h := newHandler(&mockAuth{}, auth.WithLimiter(limiter))

	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{"email":"u@u.com","password":"Secure1pass"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestLoginHandler_LimiterError(t *testing.T) {
	// When the limiter returns an error, the handler logs and continues (fail open).
	limiter := &mockLimiter{allowed: false, err: errors.New("redis down")}
	h := newHandler(&mockAuth{user: &auth.User{ID: "1", Email: "u@u.com"}}, auth.WithLimiter(limiter))

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"u@u.com","password":"secret"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (limiter error must not block login)", rec.Code, http.StatusOK)
	}
}

// --- Login: email not verified ---

func TestLoginHandler_EmailNotVerified(t *testing.T) {
	h := newHandler(&mockAuth{loginErr: auth.ErrEmailNotVerified})

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"u@u.com","password":"secret"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	assertJSONField(t, rec.Body.Bytes(), "error", "email_not_verified")
}

// --- Email flow handlers ---

// mockEmailFlow is a test double for EmailFlowService.
type mockEmailFlow struct {
	forgotErr   error
	resetErr    error
	sendVerErr  error
	resendErr   error
	verifyErr   error
}

func (m *mockEmailFlow) ForgotPassword(_ context.Context, _, _ string) error    { return m.forgotErr }
func (m *mockEmailFlow) ResetPassword(_ context.Context, _, _ string) error     { return m.resetErr }
func (m *mockEmailFlow) SendVerificationEmail(_ context.Context, _, _, _ string) error {
	return m.sendVerErr
}
func (m *mockEmailFlow) ResendVerification(_ context.Context, _, _ string) error { return m.resendErr }
func (m *mockEmailFlow) VerifyEmail(_ context.Context, _ string) error           { return m.verifyErr }

func newHandlerWithEmailFlow(a *mockAuth, ef auth.EmailFlowService) http.Handler {
	return auth.NewHandler(a, testSecret, testExpiry,
		auth.WithEmailFlow(ef, "http://localhost:3000"),
	)
}

func TestForgotPasswordHandler_AlwaysOK(t *testing.T) {
	h := newHandlerWithEmailFlow(&mockAuth{}, &mockEmailFlow{})

	for _, body := range []string{
		`{"email":"user@example.com"}`,
		`{}`,
		`not-json`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/forgot-password", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("body=%q: status = %d, want 200", body, rec.Code)
		}
	}
}

func TestResetPasswordHandler_Success(t *testing.T) {
	h := newHandlerWithEmailFlow(&mockAuth{}, &mockEmailFlow{})

	req := httptest.NewRequest(http.MethodPost, "/reset-password",
		strings.NewReader(`{"token":"abc","password":"NewPass1"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestResetPasswordHandler_InvalidToken(t *testing.T) {
	h := newHandlerWithEmailFlow(&mockAuth{}, &mockEmailFlow{resetErr: auth.ErrInvalidToken})

	req := httptest.NewRequest(http.MethodPost, "/reset-password",
		strings.NewReader(`{"token":"bad","password":"NewPass1"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestResetPasswordHandler_MissingFields(t *testing.T) {
	h := newHandlerWithEmailFlow(&mockAuth{}, &mockEmailFlow{})

	for _, body := range []string{
		`{"token":"","password":"NewPass1"}`,
		`{"token":"abc","password":""}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/reset-password", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body=%q: status = %d, want 400", body, rec.Code)
		}
	}
}

func TestVerifyEmailHandler_Success(t *testing.T) {
	h := newHandlerWithEmailFlow(&mockAuth{}, &mockEmailFlow{})

	req := httptest.NewRequest(http.MethodPost, "/verify-email",
		strings.NewReader(`{"token":"abc123"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestVerifyEmailHandler_InvalidToken(t *testing.T) {
	h := newHandlerWithEmailFlow(&mockAuth{}, &mockEmailFlow{verifyErr: auth.ErrInvalidToken})

	req := httptest.NewRequest(http.MethodPost, "/verify-email",
		strings.NewReader(`{"token":"bad"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestVerifyEmailHandler_MissingToken(t *testing.T) {
	h := newHandlerWithEmailFlow(&mockAuth{}, &mockEmailFlow{})

	req := httptest.NewRequest(http.MethodPost, "/verify-email", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestResendVerificationHandler_AlwaysOK(t *testing.T) {
	h := newHandlerWithEmailFlow(&mockAuth{}, &mockEmailFlow{})

	for _, body := range []string{
		`{"email":"user@example.com"}`,
		`{}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/resend-verification", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("body=%q: status = %d, want 200", body, rec.Code)
		}
	}
}

// --- Account handler ---

// mockAccountManager is a test double for AccountManager.
type mockAccountManager struct {
	changePasswordErr error
	deleteAccountErr  error
}

func (m *mockAccountManager) ChangePassword(_ context.Context, _, _, _ string) error {
	return m.changePasswordErr
}

func (m *mockAccountManager) DeleteAccount(_ context.Context, _, _ string) error {
	return m.deleteAccountErr
}

// authedReq creates a test request with the test user ID injected into context
// via middleware.ContextWithUserID (simulating a request that has passed RequireAuth).
func authedReq(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	ctx := middleware.ContextWithUserID(req.Context(), testUserID)
	return req.WithContext(ctx)
}

func newAccountHandler(mock *mockAccountManager) http.Handler {
	return auth.NewAccountHandler(mock)
}

// TestChangePassword_Success tests successful password change.
func TestChangePassword_Success(t *testing.T) {
	h := newAccountHandler(&mockAccountManager{})
	req := authedReq(http.MethodPut, "/password", `{"current_password":"OldPass1","new_password":"NewPass2"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestChangePassword_MissingFields(t *testing.T) {
	h := newAccountHandler(&mockAccountManager{})

	for _, body := range []string{
		`{"current_password":"","new_password":"NewPass2"}`,
		`{"current_password":"OldPass1","new_password":""}`,
	} {
		req := authedReq(http.MethodPut, "/password", body)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %s: status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestChangePassword_WrongCurrentPassword(t *testing.T) {
	h := newAccountHandler(&mockAccountManager{changePasswordErr: auth.ErrInvalidCredentials})
	req := authedReq(http.MethodPut, "/password", `{"current_password":"wrong","new_password":"NewPass2"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestChangePassword_WeakNewPassword(t *testing.T) {
	h := newAccountHandler(&mockAccountManager{changePasswordErr: auth.ErrInvalidInput})
	req := authedReq(http.MethodPut, "/password", `{"current_password":"OldPass1","new_password":"weak"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestChangePassword_NoAuth(t *testing.T) {
	h := newAccountHandler(&mockAccountManager{})
	req := httptest.NewRequest(http.MethodPut, "/password", strings.NewReader(`{"current_password":"OldPass1","new_password":"NewPass2"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestChangePassword_MalformedJSON(t *testing.T) {
	h := newAccountHandler(&mockAccountManager{})
	req := authedReq(http.MethodPut, "/password", "{not json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestChangePassword_ServiceError(t *testing.T) {
	h := newAccountHandler(&mockAccountManager{changePasswordErr: errors.New("unexpected db error")})
	req := authedReq(http.MethodPut, "/password", `{"current_password":"OldPass1","new_password":"NewPass2"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestDeleteAccount_Success(t *testing.T) {
	h := newAccountHandler(&mockAccountManager{})
	req := authedReq(http.MethodDelete, "/", `{"password":"MyPass1"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestDeleteAccount_MissingPassword(t *testing.T) {
	h := newAccountHandler(&mockAccountManager{})
	req := authedReq(http.MethodDelete, "/", `{"password":""}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestDeleteAccount_WrongPassword(t *testing.T) {
	h := newAccountHandler(&mockAccountManager{deleteAccountErr: auth.ErrInvalidCredentials})
	req := authedReq(http.MethodDelete, "/", `{"password":"wrong"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestDeleteAccount_NoAuth(t *testing.T) {
	h := newAccountHandler(&mockAccountManager{})
	req := httptest.NewRequest(http.MethodDelete, "/", strings.NewReader(`{"password":"MyPass1"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestDeleteAccount_ServiceError(t *testing.T) {
	h := newAccountHandler(&mockAccountManager{deleteAccountErr: errors.New("unexpected db error")})
	req := authedReq(http.MethodDelete, "/", `{"password":"MyPass1"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- helpers ---

func assertJSONField(t *testing.T, body []byte, key, want string) {
	t.Helper()
	var m map[string]string
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if m[key] != want {
		t.Errorf("%s = %q, want %q", key, m[key], want)
	}
}

func assertJSONFieldNonEmpty(t *testing.T, body []byte, key string) {
	t.Helper()
	var m map[string]string
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if m[key] == "" {
		t.Errorf("%s is empty, want non-empty", key)
	}
}
