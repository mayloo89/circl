package contacts_test

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

	"github.com/mayloo89/circl/backend/internal/contacts"
	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/notifications"
	"github.com/mayloo89/circl/backend/internal/token"
)

const (
	testSecret = "supersecretfortesting-mustbe32chars!!"
	testUserID = "user-abc-123"
)

// mockManager is a test double for contacts.Manager.
type mockManager struct {
	contact    *contacts.Contact
	accepted   []contacts.AcceptedContact
	users      []contacts.UserSummary
	pending    []contacts.PendingRequest
	sent       []contacts.SentRequest
	sendErr    error
	acceptErr  error
	deleteErr  error
	listErr    error
	pendingErr error
	sentErr    error
	searchErr  error
}

func (m *mockManager) SendRequest(_ context.Context, _, _ string) (*contacts.Contact, error) {
	return m.contact, m.sendErr
}
func (m *mockManager) Accept(_ context.Context, _, _ string) (*contacts.Contact, error) {
	return m.contact, m.acceptErr
}
func (m *mockManager) Delete(_ context.Context, _, _ string) error { return m.deleteErr }
func (m *mockManager) ListAccepted(_ context.Context, _ string) ([]contacts.AcceptedContact, error) {
	return m.accepted, m.listErr
}
func (m *mockManager) ListPending(_ context.Context, _ string) ([]contacts.PendingRequest, error) {
	return m.pending, m.pendingErr
}
func (m *mockManager) ListSent(_ context.Context, _ string) ([]contacts.SentRequest, error) {
	return m.sent, m.sentErr
}
func (m *mockManager) SearchUsers(_ context.Context, _, _ string) ([]contacts.UserSummary, error) {
	return m.users, m.searchErr
}

// serve wraps the handler with RequireAuth and executes the request.
func serve(h http.Handler, r *http.Request, rec *httptest.ResponseRecorder) {
	middleware.RequireAuth(testSecret)(h).ServeHTTP(rec, r)
}

// authedRequest adds a valid Bearer token for testUserID to the request.
func authedRequest(r *http.Request) *http.Request {
	tok, _ := token.Generate(testUserID, testSecret, time.Hour)
	r.Header.Set("Authorization", "Bearer "+tok)
	return r
}

// --- Search users ---

func TestSearchUsers_Success(t *testing.T) {
	users := []contacts.UserSummary{{ID: "u-2", Email: "alice@example.com", DisplayName: "Alice"}}
	h := contacts.NewHandler(&mockManager{users: users})

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/users/search?q=alice", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var got []contacts.UserSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len = %d, want 1", len(got))
	}
}

func TestSearchUsers_Unauthorized(t *testing.T) {
	h := contacts.NewHandler(&mockManager{})
	req := httptest.NewRequest(http.MethodGet, "/users/search?q=alice", nil)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestSearchUsers_ServiceError(t *testing.T) {
	h := contacts.NewHandler(&mockManager{searchErr: errors.New("db error")})
	req := authedRequest(httptest.NewRequest(http.MethodGet, "/users/search?q=alice", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- Send request ---

func TestSendRequest_Success(t *testing.T) {
	c := &contacts.Contact{ID: "c-1", Status: contacts.StatusPending}
	h := contacts.NewHandler(&mockManager{contact: c})

	body := `{"addressee_id":"u-2"}`
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/contacts", strings.NewReader(body)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestSendRequest_Unauthorized(t *testing.T) {
	h := contacts.NewHandler(&mockManager{})
	req := httptest.NewRequest(http.MethodPost, "/contacts", strings.NewReader(`{"addressee_id":"u-2"}`))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestSendRequest_MalformedJSON(t *testing.T) {
	h := contacts.NewHandler(&mockManager{})
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/contacts", strings.NewReader("{bad")))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestSendRequest_MissingAddresseeID(t *testing.T) {
	h := contacts.NewHandler(&mockManager{})
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/contacts", strings.NewReader(`{"addressee_id":""}`)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestSendRequest_SelfContact(t *testing.T) {
	h := contacts.NewHandler(&mockManager{sendErr: contacts.ErrSelfContact})
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/contacts", strings.NewReader(`{"addressee_id":"u-1"}`)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestSendRequest_AlreadyExists(t *testing.T) {
	h := contacts.NewHandler(&mockManager{sendErr: contacts.ErrAlreadyExists})
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/contacts", strings.NewReader(`{"addressee_id":"u-2"}`)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestSendRequest_InternalError(t *testing.T) {
	h := contacts.NewHandler(&mockManager{sendErr: errors.New("db error")})
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/contacts", strings.NewReader(`{"addressee_id":"u-2"}`)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- List accepted ---

func TestListAccepted_Success(t *testing.T) {
	accepted := []contacts.AcceptedContact{{ContactID: "c-1", UserID: "u-2", Email: "b@example.com"}}
	h := contacts.NewHandler(&mockManager{accepted: accepted})
	req := authedRequest(httptest.NewRequest(http.MethodGet, "/contacts", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestListAccepted_Unauthorized(t *testing.T) {
	h := contacts.NewHandler(&mockManager{})
	req := httptest.NewRequest(http.MethodGet, "/contacts", nil)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestListAccepted_ServiceError(t *testing.T) {
	h := contacts.NewHandler(&mockManager{listErr: errors.New("db error")})
	req := authedRequest(httptest.NewRequest(http.MethodGet, "/contacts", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- List pending ---

func TestListPending_Success(t *testing.T) {
	pending := []contacts.PendingRequest{{ContactID: "c-1", UserID: "u-3", Email: "c@example.com"}}
	h := contacts.NewHandler(&mockManager{pending: pending})
	req := authedRequest(httptest.NewRequest(http.MethodGet, "/contacts/pending", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestListPending_Unauthorized(t *testing.T) {
	h := contacts.NewHandler(&mockManager{})
	req := httptest.NewRequest(http.MethodGet, "/contacts/pending", nil)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestListPending_ServiceError(t *testing.T) {
	h := contacts.NewHandler(&mockManager{pendingErr: errors.New("db error")})
	req := authedRequest(httptest.NewRequest(http.MethodGet, "/contacts/pending", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- Accept ---

func TestAccept_Success(t *testing.T) {
	c := &contacts.Contact{ID: "c-1", Status: contacts.StatusAccepted}
	h := contacts.NewHandler(&mockManager{contact: c})
	req := authedRequest(httptest.NewRequest(http.MethodPut, "/contacts/c-1/accept", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAccept_Unauthorized(t *testing.T) {
	h := contacts.NewHandler(&mockManager{})
	req := httptest.NewRequest(http.MethodPut, "/contacts/c-1/accept", nil)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAccept_NotFound(t *testing.T) {
	h := contacts.NewHandler(&mockManager{acceptErr: contacts.ErrNotFound})
	req := authedRequest(httptest.NewRequest(http.MethodPut, "/contacts/c-1/accept", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAccept_InternalError(t *testing.T) {
	h := contacts.NewHandler(&mockManager{acceptErr: errors.New("db error")})
	req := authedRequest(httptest.NewRequest(http.MethodPut, "/contacts/c-1/accept", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- Delete ---

func TestDelete_Success(t *testing.T) {
	h := contacts.NewHandler(&mockManager{})
	req := authedRequest(httptest.NewRequest(http.MethodDelete, "/contacts/c-1", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestDelete_Unauthorized(t *testing.T) {
	h := contacts.NewHandler(&mockManager{})
	req := httptest.NewRequest(http.MethodDelete, "/contacts/c-1", nil)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestDelete_NotFound(t *testing.T) {
	h := contacts.NewHandler(&mockManager{deleteErr: contacts.ErrNotFound})
	req := authedRequest(httptest.NewRequest(http.MethodDelete, "/contacts/c-1", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestDelete_InternalError(t *testing.T) {
	h := contacts.NewHandler(&mockManager{deleteErr: errors.New("db error")})
	req := authedRequest(httptest.NewRequest(http.MethodDelete, "/contacts/c-1", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- Content-Type ---

func TestHandlers_ContentType(t *testing.T) {
	users := []contacts.UserSummary{}
	h := contacts.NewHandler(&mockManager{users: users})
	req := authedRequest(httptest.NewRequest(http.MethodGet, "/contacts", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// --- No userID in context (defensive branches) ---

func TestSearchUsers_NoUserIDInContext(t *testing.T) {
	h := contacts.NewHandler(&mockManager{})
	req := httptest.NewRequest(http.MethodGet, "/users/search?q=alice", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestSendRequest_NoUserIDInContext(t *testing.T) {
	h := contacts.NewHandler(&mockManager{})
	body, _ := json.Marshal(map[string]string{"addressee_id": "u-2"})
	req := httptest.NewRequest(http.MethodPost, "/contacts", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestListAccepted_NoUserIDInContext(t *testing.T) {
	h := contacts.NewHandler(&mockManager{})
	req := httptest.NewRequest(http.MethodGet, "/contacts", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestListPending_NoUserIDInContext(t *testing.T) {
	h := contacts.NewHandler(&mockManager{})
	req := httptest.NewRequest(http.MethodGet, "/contacts/pending", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAccept_NoUserIDInContext(t *testing.T) {
	h := contacts.NewHandler(&mockManager{})
	req := httptest.NewRequest(http.MethodPut, "/contacts/c-1/accept", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestDelete_NoUserIDInContext(t *testing.T) {
	h := contacts.NewHandler(&mockManager{})
	req := httptest.NewRequest(http.MethodDelete, "/contacts/c-1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// --- ListSent ---

func TestListSent_Success(t *testing.T) {
	sent := []contacts.SentRequest{{ContactID: "c-1", UserID: "u-2", Email: "b@example.com"}}
	h := contacts.NewHandler(&mockManager{sent: sent})
	req := authedRequest(httptest.NewRequest(http.MethodGet, "/contacts/sent", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var got []contacts.SentRequest
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].ContactID != "c-1" {
		t.Errorf("got = %v", got)
	}
}

func TestListSent_Unauthorized(t *testing.T) {
	h := contacts.NewHandler(&mockManager{})
	req := httptest.NewRequest(http.MethodGet, "/contacts/sent", nil)
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestListSent_InternalError(t *testing.T) {
	h := contacts.NewHandler(&mockManager{sentErr: errors.New("db error")})
	req := authedRequest(httptest.NewRequest(http.MethodGet, "/contacts/sent", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestListSent_NoUserIDInContext(t *testing.T) {
	h := contacts.NewHandler(&mockManager{})
	req := httptest.NewRequest(http.MethodGet, "/contacts/sent", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// --- Notifier ---

type mockNotifier struct {
	calledWith []struct {
		userID string
		event  notifications.Event
	}
}

func (m *mockNotifier) Notify(userID string, e notifications.Event) {
	m.calledWith = append(m.calledWith, struct {
		userID string
		event  notifications.Event
	}{userID, e})
}

func TestSendRequest_NotifiesAddressee(t *testing.T) {
	c := &contacts.Contact{ID: "c-1", RequesterID: testUserID, AddresseeID: "u-2", Status: contacts.StatusPending}
	notifier := &mockNotifier{}
	h := contacts.NewHandler(&mockManager{contact: c}, contacts.WithNotifier(notifier))

	req := authedRequest(httptest.NewRequest(http.MethodPost, "/contacts", strings.NewReader(`{"addressee_id":"u-2"}`)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if len(notifier.calledWith) != 1 {
		t.Fatalf("Notify called %d times, want 1", len(notifier.calledWith))
	}
	if notifier.calledWith[0].userID != "u-2" {
		t.Errorf("Notify userID = %q, want u-2", notifier.calledWith[0].userID)
	}
	if notifier.calledWith[0].event.Type != "contact_request" {
		t.Errorf("Notify event type = %q, want contact_request", notifier.calledWith[0].event.Type)
	}
}

func TestAccept_NotifiesRequester(t *testing.T) {
	c := &contacts.Contact{ID: "c-1", RequesterID: "u-3", AddresseeID: testUserID, Status: contacts.StatusAccepted}
	notifier := &mockNotifier{}
	h := contacts.NewHandler(&mockManager{contact: c}, contacts.WithNotifier(notifier))

	req := authedRequest(httptest.NewRequest(http.MethodPut, "/contacts/c-1/accept", nil))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(notifier.calledWith) != 1 {
		t.Fatalf("Notify called %d times, want 1", len(notifier.calledWith))
	}
	if notifier.calledWith[0].userID != "u-3" {
		t.Errorf("Notify userID = %q, want u-3", notifier.calledWith[0].userID)
	}
	if notifier.calledWith[0].event.Type != "contact_accepted" {
		t.Errorf("Notify event type = %q, want contact_accepted", notifier.calledWith[0].event.Type)
	}
}

func TestSendRequest_NoNotifier_NoPanic(t *testing.T) {
	c := &contacts.Contact{ID: "c-1", RequesterID: testUserID, AddresseeID: "u-2", Status: contacts.StatusPending}
	h := contacts.NewHandler(&mockManager{contact: c}) // no WithNotifier

	req := authedRequest(httptest.NewRequest(http.MethodPost, "/contacts", strings.NewReader(`{"addressee_id":"u-2"}`)))
	rec := httptest.NewRecorder()
	serve(h, req, rec)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
}
