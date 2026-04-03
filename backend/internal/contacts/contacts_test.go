package contacts_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mayloo89/circl/backend/internal/contacts"
)

// mockStore is a test double for contacts.Store.
type mockStore struct {
	contact         *contacts.Contact
	accepted        []contacts.AcceptedContact
	users           []contacts.UserSummary
	pending         []contacts.PendingRequest
	sent            []contacts.SentRequest
	blocked         []contacts.BlockedUser
	sendErr         error
	acceptErr       error
	deleteErr       error
	listErr         error
	pendingErr      error
	sentErr         error
	searchErr       error
	blockErr        error
	unblockErr      error
	listBlockedErr  error
	isBlockedResult bool
	isBlockedErr    error
}

func (m *mockStore) SendRequest(_ context.Context, _, _ string) (*contacts.Contact, error) {
	return m.contact, m.sendErr
}
func (m *mockStore) Accept(_ context.Context, _, _ string) (*contacts.Contact, error) {
	return m.contact, m.acceptErr
}
func (m *mockStore) Delete(_ context.Context, _, _ string) (*contacts.Contact, error) {
	return m.contact, m.deleteErr
}
func (m *mockStore) ListAccepted(_ context.Context, _ string) ([]contacts.AcceptedContact, error) {
	return m.accepted, m.listErr
}
func (m *mockStore) ListPending(_ context.Context, _ string) ([]contacts.PendingRequest, error) {
	return m.pending, m.pendingErr
}
func (m *mockStore) ListSent(_ context.Context, _ string) ([]contacts.SentRequest, error) {
	return m.sent, m.sentErr
}
func (m *mockStore) SearchUsers(_ context.Context, _, _ string) ([]contacts.UserSummary, error) {
	return m.users, m.searchErr
}
func (m *mockStore) Block(_ context.Context, _, _ string) error {
	return m.blockErr
}
func (m *mockStore) Unblock(_ context.Context, _, _ string) error {
	return m.unblockErr
}
func (m *mockStore) ListBlocked(_ context.Context, _ string) ([]contacts.BlockedUser, error) {
	return m.blocked, m.listBlockedErr
}
func (m *mockStore) IsBlocked(_ context.Context, _, _ string) (bool, error) {
	return m.isBlockedResult, m.isBlockedErr
}

func newService(store contacts.Store) *contacts.Service {
	return contacts.NewService(store)
}

// --- SendRequest ---

func TestService_SendRequest_Success(t *testing.T) {
	c := &contacts.Contact{ID: "c-1", RequesterID: "u-1", AddresseeID: "u-2", Status: contacts.StatusPending}
	svc := newService(&mockStore{contact: c})

	got, err := svc.SendRequest(t.Context(), "u-1", "u-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "c-1" {
		t.Errorf("ID = %q, want %q", got.ID, "c-1")
	}
}

func TestService_SendRequest_SelfContact(t *testing.T) {
	svc := newService(&mockStore{})

	_, err := svc.SendRequest(t.Context(), "u-1", "u-1")
	if !errors.Is(err, contacts.ErrSelfContact) {
		t.Errorf("err = %v, want ErrSelfContact", err)
	}
}

func TestService_SendRequest_AlreadyExists(t *testing.T) {
	svc := newService(&mockStore{sendErr: contacts.ErrAlreadyExists})

	_, err := svc.SendRequest(t.Context(), "u-1", "u-2")
	if !errors.Is(err, contacts.ErrAlreadyExists) {
		t.Errorf("err = %v, want ErrAlreadyExists", err)
	}
}

func TestService_SendRequest_StoreError(t *testing.T) {
	svc := newService(&mockStore{sendErr: errors.New("db error")})

	_, err := svc.SendRequest(t.Context(), "u-1", "u-2")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- Accept ---

func TestService_Accept_Success(t *testing.T) {
	c := &contacts.Contact{ID: "c-1", Status: contacts.StatusAccepted}
	svc := newService(&mockStore{contact: c})

	got, err := svc.Accept(t.Context(), "c-1", "u-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != contacts.StatusAccepted {
		t.Errorf("status = %q, want %q", got.Status, contacts.StatusAccepted)
	}
}

func TestService_Accept_NotFound(t *testing.T) {
	svc := newService(&mockStore{acceptErr: contacts.ErrNotFound})

	_, err := svc.Accept(t.Context(), "c-1", "u-2")
	if !errors.Is(err, contacts.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// --- Delete ---

func TestService_Delete_Success(t *testing.T) {
	c := &contacts.Contact{ID: "c-1", RequesterID: "u-1", AddresseeID: "u-2", Status: contacts.StatusAccepted}
	svc := newService(&mockStore{contact: c})

	got, err := svc.Delete(t.Context(), "c-1", "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "c-1" {
		t.Errorf("ID = %q, want c-1", got.ID)
	}
}

func TestService_Delete_NotFound(t *testing.T) {
	svc := newService(&mockStore{deleteErr: contacts.ErrNotFound})

	_, err := svc.Delete(t.Context(), "c-1", "u-1")
	if !errors.Is(err, contacts.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// --- ListAccepted ---

func TestService_ListAccepted_Success(t *testing.T) {
	accepted := []contacts.AcceptedContact{{ContactID: "c-1", UserID: "u-2", Email: "b@example.com"}}
	svc := newService(&mockStore{accepted: accepted})

	got, err := svc.ListAccepted(t.Context(), "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len = %d, want 1", len(got))
	}
}

func TestService_ListAccepted_StoreError(t *testing.T) {
	svc := newService(&mockStore{listErr: errors.New("db error")})

	_, err := svc.ListAccepted(t.Context(), "u-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- ListPending ---

func TestService_ListPending_Success(t *testing.T) {
	pending := []contacts.PendingRequest{{ContactID: "c-1", UserID: "u-3", Email: "c@example.com"}}
	svc := newService(&mockStore{pending: pending})

	got, err := svc.ListPending(t.Context(), "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len = %d, want 1", len(got))
	}
}

// --- ListSent ---

func TestService_ListSent_Success(t *testing.T) {
	sent := []contacts.SentRequest{{ContactID: "c-1", UserID: "u-2", Email: "b@example.com"}}
	svc := newService(&mockStore{sent: sent})

	got, err := svc.ListSent(t.Context(), "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len = %d, want 1", len(got))
	}
}

func TestService_ListSent_StoreError(t *testing.T) {
	svc := newService(&mockStore{sentErr: errors.New("db error")})

	_, err := svc.ListSent(t.Context(), "u-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- SearchUsers ---

func TestService_SearchUsers_Success(t *testing.T) {
	users := []contacts.UserSummary{{ID: "u-2", Email: "alice@example.com", DisplayName: "Alice"}}
	svc := newService(&mockStore{users: users})

	got, err := svc.SearchUsers(t.Context(), "alice", "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len = %d, want 1", len(got))
	}
}

func TestService_SearchUsers_EmptyQuery(t *testing.T) {
	svc := newService(&mockStore{})

	got, err := svc.SearchUsers(t.Context(), "", "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty result for empty query, got %d", len(got))
	}
}

func TestService_SearchUsers_StoreError(t *testing.T) {
	svc := newService(&mockStore{searchErr: errors.New("db error")})

	_, err := svc.SearchUsers(t.Context(), "alice", "u-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- SendRequest blocked ---

func TestService_SendRequest_Blocked(t *testing.T) {
	svc := newService(&mockStore{isBlockedResult: true})

	_, err := svc.SendRequest(t.Context(), "u-1", "u-2")
	if !errors.Is(err, contacts.ErrForbidden) {
		t.Errorf("err = %v, want ErrForbidden", err)
	}
}

func TestService_SendRequest_IsBlockedError(t *testing.T) {
	svc := newService(&mockStore{isBlockedErr: errors.New("db error")})

	_, err := svc.SendRequest(t.Context(), "u-1", "u-2")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- Block ---

func TestService_Block_Success(t *testing.T) {
	svc := newService(&mockStore{})

	if err := svc.Block(t.Context(), "u-1", "u-2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestService_Block_Self(t *testing.T) {
	svc := newService(&mockStore{})

	err := svc.Block(t.Context(), "u-1", "u-1")
	if !errors.Is(err, contacts.ErrSelfContact) {
		t.Errorf("err = %v, want ErrSelfContact", err)
	}
}

func TestService_Block_AlreadyBlocked(t *testing.T) {
	svc := newService(&mockStore{blockErr: contacts.ErrAlreadyBlocked})

	err := svc.Block(t.Context(), "u-1", "u-2")
	if !errors.Is(err, contacts.ErrAlreadyBlocked) {
		t.Errorf("err = %v, want ErrAlreadyBlocked", err)
	}
}

func TestService_Block_StoreError(t *testing.T) {
	svc := newService(&mockStore{blockErr: errors.New("db error")})

	if err := svc.Block(t.Context(), "u-1", "u-2"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- Unblock ---

func TestService_Unblock_Success(t *testing.T) {
	svc := newService(&mockStore{})

	if err := svc.Unblock(t.Context(), "u-1", "u-2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestService_Unblock_NotFound(t *testing.T) {
	svc := newService(&mockStore{unblockErr: contacts.ErrNotFound})

	err := svc.Unblock(t.Context(), "u-1", "u-2")
	if !errors.Is(err, contacts.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// --- ListBlocked ---

func TestService_ListBlocked_Success(t *testing.T) {
	bl := []contacts.BlockedUser{{BlockID: "b-1", UserID: "u-2", Email: "b@example.com"}}
	svc := newService(&mockStore{blocked: bl})

	got, err := svc.ListBlocked(t.Context(), "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len = %d, want 1", len(got))
	}
}

func TestService_ListBlocked_StoreError(t *testing.T) {
	svc := newService(&mockStore{listBlockedErr: errors.New("db error")})

	_, err := svc.ListBlocked(t.Context(), "u-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- IsBlocked ---

func TestService_IsBlocked_True(t *testing.T) {
	svc := newService(&mockStore{isBlockedResult: true})

	got, err := svc.IsBlocked(t.Context(), "u-1", "u-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("expected true, got false")
	}
}

func TestService_IsBlocked_False(t *testing.T) {
	svc := newService(&mockStore{})

	got, err := svc.IsBlocked(t.Context(), "u-1", "u-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected false, got true")
	}
}
