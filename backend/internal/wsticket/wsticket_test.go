package wsticket_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/token"
	"github.com/mayloo89/circl/backend/internal/wsticket"
)

const testSecret = "supersecretfortesting-mustbe32chars!!"

func newStore(t *testing.T) (*wsticket.Store, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return wsticket.NewStore(rdb), mr
}

func authedReq(r *http.Request, userID string) *http.Request {
	tok, _ := token.Generate(userID, token.RoleUser, testSecret, time.Hour)
	r.Header.Set("Authorization", "Bearer "+tok)
	return r
}

// --- Store tests ---

func TestStore_IssueAndRedeem(t *testing.T) {
	store, _ := newStore(t)
	ctx := t.Context()

	ticket, err := store.Issue(ctx, "user-1")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if ticket == "" {
		t.Fatal("ticket should not be empty")
	}

	userID, err := store.Redeem(ctx, ticket)
	if err != nil {
		t.Fatalf("Redeem: %v", err)
	}
	if userID != "user-1" {
		t.Errorf("userID = %q, want %q", userID, "user-1")
	}
}

func TestStore_TicketSingleUse(t *testing.T) {
	store, _ := newStore(t)
	ctx := t.Context()

	ticket, _ := store.Issue(ctx, "user-1")
	store.Redeem(ctx, ticket) //nolint:errcheck

	_, err := store.Redeem(ctx, ticket)
	if err != wsticket.ErrInvalid {
		t.Errorf("second redeem: err = %v, want ErrInvalid", err)
	}
}

func TestStore_RedeemUnknownTicket(t *testing.T) {
	store, _ := newStore(t)

	_, err := store.Redeem(t.Context(), "no-such-ticket")
	if err != wsticket.ErrInvalid {
		t.Errorf("err = %v, want ErrInvalid", err)
	}
}

func TestStore_TicketExpires(t *testing.T) {
	store, mr := newStore(t)
	ctx := t.Context()

	ticket, _ := store.Issue(ctx, "user-1")

	// Advance Redis time past the 60s TTL.
	mr.FastForward(61 * time.Second)

	_, err := store.Redeem(ctx, ticket)
	if err != wsticket.ErrInvalid {
		t.Errorf("err = %v, want ErrInvalid after expiry", err)
	}
}

// --- Handler tests ---

func TestHandler_NoAuth(t *testing.T) {
	store, _ := newStore(t)
	h := wsticket.NewHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/ws-ticket", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req) // no auth middleware — context has no userID
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestHandler_ReturnsTicket(t *testing.T) {
	store, _ := newStore(t)
	h := wsticket.NewHandler(store)

	req := authedReq(httptest.NewRequest(http.MethodPost, "/ws-ticket", nil), "user-1")
	rec := httptest.NewRecorder()
	middleware.RequireAuth(testSecret)(h).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !containsStr(body, `"ticket"`) {
		t.Error("response should contain a ticket field")
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && func() bool {
		for i := range len(s) - len(sub) + 1 {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}
