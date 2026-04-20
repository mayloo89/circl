package push

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/rs/zerolog"
)

// mockStore is a test double for Store.
type mockStore struct {
	saved            []Subscription
	listResult       []Subscription
	listErr          error
	saveErr          error
	deleteErr        error
	deletedEndpoints []string
}

func (m *mockStore) Save(_ context.Context, sub Subscription) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.saved = append(m.saved, sub)
	return nil
}

func (m *mockStore) Delete(_ context.Context, _, _ string) error {
	return m.deleteErr
}

func (m *mockStore) ListByUser(_ context.Context, _ string) ([]Subscription, error) {
	return m.listResult, m.listErr
}

func (m *mockStore) DeleteByEndpoint(_ context.Context, endpoint string) error {
	m.deletedEndpoints = append(m.deletedEndpoints, endpoint)
	return nil
}

func fakeResponse(status int) (*http.Response, error) {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader("")),
	}, nil
}

func newTestService(store Store) *Service {
	return &Service{
		store:        store,
		vapidPublic:  "public",
		vapidPrivate: "private",
		vapidSubject: "mailto:test@example.com",
		log:          zerolog.Nop(),
		sender: func(_ context.Context, _ []byte, _ *webpush.Subscription, _ *webpush.Options) (*http.Response, error) {
			return fakeResponse(http.StatusCreated)
		},
	}
}

// --- Enabled ---

func TestService_Enabled_WithKeys(t *testing.T) {
	svc := newTestService(&mockStore{})
	if !svc.Enabled() {
		t.Error("expected Enabled()=true when VAPID keys are set")
	}
}

func TestService_Enabled_WithoutKeys(t *testing.T) {
	svc := NewService(&mockStore{}, "", "", "", zerolog.Nop())
	if svc.Enabled() {
		t.Error("expected Enabled()=false when VAPID keys are empty")
	}
}

// --- Subscribe ---

func TestService_Subscribe_SavesCalled(t *testing.T) {
	store := &mockStore{}
	svc := newTestService(store)

	if err := svc.Subscribe(t.Context(), "user-1", "https://ep", "p256", "auth"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.saved) != 1 {
		t.Errorf("saved = %d, want 1", len(store.saved))
	}
	if store.saved[0].Endpoint != "https://ep" {
		t.Errorf("endpoint = %q, want %q", store.saved[0].Endpoint, "https://ep")
	}
}

func TestService_Subscribe_PropagatesStoreError(t *testing.T) {
	store := &mockStore{saveErr: errNotFound}
	svc := newTestService(store)

	if err := svc.Subscribe(t.Context(), "u", "ep", "p", "a"); err == nil {
		t.Error("expected error from store, got nil")
	}
}

// --- Send ---

func TestService_Send_Disabled(t *testing.T) {
	store := &mockStore{listResult: []Subscription{{Endpoint: "https://ep", P256DH: "p", Auth: "a"}}}
	svc := NewService(store, "", "", "", zerolog.Nop()) // disabled — no VAPID keys

	called := false
	svc.sender = func(_ context.Context, _ []byte, _ *webpush.Subscription, _ *webpush.Options) (*http.Response, error) {
		called = true
		return fakeResponse(http.StatusCreated)
	}

	svc.Send(t.Context(), "user-1", Notification{Title: "Hi"})
	if called {
		t.Error("sender must not be called when service is disabled")
	}
}

func TestService_Send_NoSubscriptions(t *testing.T) {
	store := &mockStore{listResult: nil}
	svc := newTestService(store)

	called := false
	svc.sender = func(_ context.Context, _ []byte, _ *webpush.Subscription, _ *webpush.Options) (*http.Response, error) {
		called = true
		return fakeResponse(http.StatusCreated)
	}

	svc.Send(t.Context(), "user-1", Notification{Title: "Hi"})
	if called {
		t.Error("sender must not be called when there are no subscriptions")
	}
}

func TestService_Send_CallsSenderForEachSubscription(t *testing.T) {
	store := &mockStore{listResult: []Subscription{
		{Endpoint: "https://ep1", P256DH: "p", Auth: "a"},
		{Endpoint: "https://ep2", P256DH: "p", Auth: "a"},
	}}
	svc := newTestService(store)

	var called int
	svc.sender = func(_ context.Context, _ []byte, _ *webpush.Subscription, _ *webpush.Options) (*http.Response, error) {
		called++
		return fakeResponse(http.StatusCreated)
	}

	svc.Send(t.Context(), "user-1", Notification{Title: "Hi"})
	if called != 2 {
		t.Errorf("sender called %d times, want 2", called)
	}
}

func TestService_Send_DeletesStaleSubscriptionOn410(t *testing.T) {
	store := &mockStore{listResult: []Subscription{
		{Endpoint: "https://ep-gone", P256DH: "p", Auth: "a"},
	}}
	svc := newTestService(store)
	svc.sender = func(_ context.Context, _ []byte, _ *webpush.Subscription, _ *webpush.Options) (*http.Response, error) {
		return fakeResponse(http.StatusGone)
	}

	svc.Send(t.Context(), "user-1", Notification{Title: "Hi"})
	if len(store.deletedEndpoints) != 1 || store.deletedEndpoints[0] != "https://ep-gone" {
		t.Errorf("stale endpoint not deleted; got %v", store.deletedEndpoints)
	}
}

func TestService_Send_DeletesStaleSubscriptionOn404(t *testing.T) {
	store := &mockStore{listResult: []Subscription{
		{Endpoint: "https://ep-missing", P256DH: "p", Auth: "a"},
	}}
	svc := newTestService(store)
	svc.sender = func(_ context.Context, _ []byte, _ *webpush.Subscription, _ *webpush.Options) (*http.Response, error) {
		return fakeResponse(http.StatusNotFound)
	}

	svc.Send(t.Context(), "user-1", Notification{Title: "Hi"})
	if len(store.deletedEndpoints) != 1 {
		t.Errorf("stale endpoint not deleted; got %v", store.deletedEndpoints)
	}
}

func TestService_Send_ContinuesAfterSenderError(t *testing.T) {
	store := &mockStore{listResult: []Subscription{
		{Endpoint: "https://ep1", P256DH: "p", Auth: "a"},
		{Endpoint: "https://ep2", P256DH: "p", Auth: "a"},
	}}
	svc := newTestService(store)

	var called int
	svc.sender = func(_ context.Context, _ []byte, s *webpush.Subscription, _ *webpush.Options) (*http.Response, error) {
		called++
		if s.Endpoint == "https://ep1" {
			return nil, io.ErrUnexpectedEOF
		}
		return fakeResponse(http.StatusCreated)
	}

	svc.Send(t.Context(), "user-1", Notification{Title: "Hi"})
	if called != 2 {
		t.Errorf("sender called %d times, want 2 (should continue after error)", called)
	}
}

func TestService_Send_ListError(t *testing.T) {
	store := &mockStore{listErr: io.ErrUnexpectedEOF}
	svc := newTestService(store)

	called := false
	svc.sender = func(_ context.Context, _ []byte, _ *webpush.Subscription, _ *webpush.Options) (*http.Response, error) {
		called = true
		return fakeResponse(http.StatusCreated)
	}

	// Must not panic; sender must not be called.
	svc.Send(t.Context(), "user-1", Notification{Title: "Hi"})
	if called {
		t.Error("sender must not be called when list fails")
	}
}
