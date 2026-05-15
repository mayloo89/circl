package appeals_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mayloo89/circl/backend/internal/appeals"
	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/token"
)

const testSecret = "supersecretfortesting-mustbe32chars!!"

func TestPublicHandler_Get_ValidToken(t *testing.T) {
	expires := time.Now().Add(time.Hour)
	store := &mockStore{byToken: &appeals.Appeal{ID: "a-1", Status: appeals.StatusOpen, ExpiresAt: expires}}
	svc := appeals.NewService(store, nil)
	h := appeals.NewPublicHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/anytoken", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (body=%q)", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestPublicHandler_Get_NotFound(t *testing.T) {
	store := &mockStore{byTokenErr: appeals.ErrNotFound}
	svc := appeals.NewService(store, nil)
	h := appeals.NewPublicHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/bogus", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestPublicHandler_Submit_HappyPath(t *testing.T) {
	expires := time.Now().Add(time.Hour)
	store := &mockStore{
		byToken: &appeals.Appeal{ID: "a-1", Status: appeals.StatusOpen, ExpiresAt: expires},
		submit:  &appeals.Appeal{ID: "a-1", Status: appeals.StatusSubmitted, Body: "I think this was a mistake"},
	}
	svc := appeals.NewService(store, nil)
	h := appeals.NewPublicHandler(svc)

	body := `{"body":"I think this was a mistake"}`
	req := httptest.NewRequest(http.MethodPost, "/anytoken", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestPublicHandler_Submit_EmptyBody(t *testing.T) {
	store := &mockStore{byToken: &appeals.Appeal{ID: "a-1", Status: appeals.StatusOpen, ExpiresAt: time.Now().Add(time.Hour)}}
	svc := appeals.NewService(store, nil)
	h := appeals.NewPublicHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/anytoken", strings.NewReader(`{"body":""}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPublicHandler_Submit_BodyTooLong(t *testing.T) {
	store := &mockStore{byToken: &appeals.Appeal{ID: "a-1", Status: appeals.StatusOpen, ExpiresAt: time.Now().Add(time.Hour)}}
	svc := appeals.NewService(store, nil)
	h := appeals.NewPublicHandler(svc)

	tooLong := strings.Repeat("x", appeals.MaxBodyLen+1)
	req := httptest.NewRequest(http.MethodPost, "/anytoken", strings.NewReader(`{"body":"`+tooLong+`"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAdminHandler_List_RequiresAdmin(t *testing.T) {
	svc := appeals.NewService(&mockStore{}, nil)
	h := appeals.NewAdminHandler(svc)

	// Plain user token — should be forbidden.
	tok, _ := token.Generate("regular-user", token.RoleUser, testSecret, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	middleware.RequireAuth(testSecret)(h).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestAdminHandler_List_Admin(t *testing.T) {
	svc := appeals.NewService(&mockStore{list: []appeals.AppealWithUserInfo{{Appeal: appeals.Appeal{ID: "a-1"}}}}, nil)
	h := appeals.NewAdminHandler(svc)

	tok, _ := token.Generate("admin-1", token.RoleAdmin, testSecret, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	middleware.RequireAuth(testSecret)(h).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAdminHandler_List_InvalidStatus(t *testing.T) {
	svc := appeals.NewService(&mockStore{}, nil)
	h := appeals.NewAdminHandler(svc)

	tok, _ := token.Generate("admin-1", token.RoleAdmin, testSecret, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/?status=garbage", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	middleware.RequireAuth(testSecret)(h).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAdminHandler_Resolve_Approve(t *testing.T) {
	store := &mockStore{
		byID:    &appeals.Appeal{ID: "a-1", Status: appeals.StatusSubmitted, UserID: "u-1"},
		resolve: &appeals.Appeal{ID: "a-1", Status: appeals.StatusApproved, UserID: "u-1"},
	}
	reactivator := &mockReactivator{}
	svc := appeals.NewService(store, reactivator)
	h := appeals.NewAdminHandler(svc)

	tok, _ := token.Generate("admin-1", token.RoleAdmin, testSecret, time.Hour)
	body := `{"status":"approved","note":"OK"}`
	req := httptest.NewRequest(http.MethodPut, "/a-1", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()

	// Route through chi to capture {id}.
	router := chi.NewRouter()
	router.Mount("/", h)
	middleware.RequireAuth(testSecret)(router).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (body=%q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !reactivator.called {
		t.Error("expected reactivator to be called")
	}
}

func TestAdminHandler_Resolve_InvalidStatus(t *testing.T) {
	svc := appeals.NewService(&mockStore{}, nil)
	h := appeals.NewAdminHandler(svc)

	tok, _ := token.Generate("admin-1", token.RoleAdmin, testSecret, time.Hour)
	req := httptest.NewRequest(http.MethodPut, "/a-1", strings.NewReader(`{"status":"maybe"}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()

	router := chi.NewRouter()
	router.Mount("/", h)
	middleware.RequireAuth(testSecret)(router).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
