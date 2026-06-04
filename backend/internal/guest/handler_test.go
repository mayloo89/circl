package guest_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mayloo89/circl/backend/internal/chat"
	"github.com/mayloo89/circl/backend/internal/guest"
)

type mockSessionStore struct {
	session *guest.Session
	err     error
}

func (m *mockSessionStore) Create(_ context.Context, nickname string) (*guest.Session, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &guest.Session{ID: "sess-1", Nickname: nickname}, nil
}

func (m *mockSessionStore) Get(_ context.Context, _ string) (*guest.Session, error) {
	return m.session, m.err
}

type mockManager struct {
	rooms   []chat.PublicRoomSummary
	room    *chat.Room
	roomErr error
	err     error
}

func (m *mockManager) ListPublicRooms(_ context.Context) ([]chat.PublicRoomSummary, error) {
	return m.rooms, m.err
}

func (mockManager) CreatePublicRoom(context.Context, string, string, string) (*chat.Room, error) {
	panic("unexpected")
}

func (mockManager) NicknameTaken(context.Context, string) (bool, error) {
	panic("unexpected")
}

func (mockManager) GetOrCreateDM(context.Context, string, string) (*chat.Room, error) {
	panic("unexpected")
}

func (mockManager) CreateGroup(context.Context, string, string, []string) (*chat.Room, error) {
	panic("unexpected")
}

func (mockManager) CreateChannel(context.Context, string, string, string) (*chat.Room, error) {
	panic("unexpected")
}

func (mockManager) ListChannels(context.Context) ([]chat.ChannelSummary, error) {
	panic("unexpected")
}

func (m *mockManager) GetRoom(context.Context, string) (*chat.Room, error) {
	return m.room, m.roomErr
}

func (mockManager) IsMember(context.Context, string, string) (bool, error) {
	panic("unexpected")
}

func (mockManager) ListMembers(context.Context, string) ([]string, error) {
	panic("unexpected")
}

func (mockManager) ListMemberProfiles(context.Context, string) ([]chat.MemberProfile, error) {
	panic("unexpected")
}

func (mockManager) AddGroupMember(context.Context, string, string, string) error {
	panic("unexpected")
}

func (mockManager) RemoveGroupMember(context.Context, string, string, string) error {
	panic("unexpected")
}

func (mockManager) UpdateGroupName(context.Context, string, string, string) error {
	panic("unexpected")
}

func (mockManager) ListRooms(context.Context, string) ([]chat.RoomSummary, error) {
	panic("unexpected")
}

func (mockManager) SaveMessage(context.Context, chat.SaveMessageParams) (*chat.Message, error) {
	panic("unexpected")
}

func (mockManager) ListMessages(context.Context, string, *time.Time, int) ([]chat.Message, error) {
	panic("unexpected")
}

func (mockManager) MarkRead(context.Context, string, string) (time.Time, error) {
	panic("unexpected")
}

func (mockManager) ViewOnceMessage(context.Context, string, string, string) (*chat.Message, []string, error) {
	panic("unexpected")
}

func (mockManager) DeleteMessage(context.Context, string) (string, []string, error) {
	panic("unexpected")
}

func (mockManager) TombstoneMessage(context.Context, string) (string, []string, error) {
	panic("unexpected")
}

func (mockManager) ListExpiredMessages(context.Context) ([]string, error) {
	panic("unexpected")
}

func (mockManager) GetDisplayName(context.Context, string) (string, error) {
	panic("unexpected")
}

func (mockManager) GetAvatarURL(context.Context, string) (string, error) {
	panic("unexpected")
}

func (mockManager) GetUsername(context.Context, string) (string, error) {
	panic("unexpected")
}

func (mockManager) GetDMPeerID(context.Context, string, string) (string, error) {
	panic("unexpected")
}

func (mockManager) ListRetentionEligibleMessages(context.Context) ([]string, error) {
	panic("unexpected")
}

func TestCreateGuestSession_Success(t *testing.T) {
	sessions := &mockSessionStore{}
	nicknameTaken := func(_ context.Context, _ string) (bool, error) { return false, nil }
	mgr := &mockManager{}
	h := guest.NewHandler(guest.HandlerConfig{
		Sessions:      sessions,
		NicknameTaken: nicknameTaken,
		RoomLister:    mgr,
	})

	body := `{"nickname":"Wanderer","age_attestation":true}`
	req := httptest.NewRequest(http.MethodPost, "/session", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["nickname"] != "Wanderer" {
		t.Errorf("nickname = %q, want Wanderer", got["nickname"])
	}
	if got["session_id"] == "" {
		t.Error("session_id should not be empty")
	}
}

func TestCreateGuestSession_CaptchaFails(t *testing.T) {
	h := guest.NewHandler(guest.HandlerConfig{
		Sessions:      &mockSessionStore{},
		NicknameTaken: func(_ context.Context, _ string) (bool, error) { return false, nil },
		RoomLister:    &mockManager{},
		Captcha:       func(_ context.Context, _, _ string) (bool, error) { return false, nil },
	})

	body := `{"nickname":"Bot","age_attestation":true,"captcha_token":"bad"}`
	req := httptest.NewRequest(http.MethodPost, "/session", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (captcha failed)", rec.Code)
	}
}

func TestCreateGuestSession_CaptchaPasses(t *testing.T) {
	var gotToken string
	h := guest.NewHandler(guest.HandlerConfig{
		Sessions:      &mockSessionStore{},
		NicknameTaken: func(_ context.Context, _ string) (bool, error) { return false, nil },
		RoomLister:    &mockManager{},
		Captcha: func(_ context.Context, token, _ string) (bool, error) {
			gotToken = token
			return true, nil
		},
	})

	body := `{"nickname":"Human","age_attestation":true,"captcha_token":"good"}`
	req := httptest.NewRequest(http.MethodPost, "/session", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if gotToken != "good" {
		t.Errorf("captcha token forwarded = %q, want good", gotToken)
	}
}

func TestCreateGuestSession_MissingNickname(t *testing.T) {
	sessions := &mockSessionStore{}
	h := guest.NewHandler(guest.HandlerConfig{
		Sessions:   sessions,
		RoomLister: &mockManager{},
	})

	body := `{"age_attestation":true}`
	req := httptest.NewRequest(http.MethodPost, "/session", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestCreateGuestSession_NoAgeAttestation(t *testing.T) {
	sessions := &mockSessionStore{}
	h := guest.NewHandler(guest.HandlerConfig{
		Sessions:   sessions,
		RoomLister: &mockManager{},
	})

	body := `{"nickname":"Wanderer","age_attestation":false}`
	req := httptest.NewRequest(http.MethodPost, "/session", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestCreateGuestSession_NicknameTaken(t *testing.T) {
	sessions := &mockSessionStore{}
	nicknameTaken := func(_ context.Context, _ string) (bool, error) { return true, nil }
	h := guest.NewHandler(guest.HandlerConfig{
		Sessions:      sessions,
		NicknameTaken: nicknameTaken,
		RoomLister:    &mockManager{},
	})

	body := `{"nickname":"AdminNick","age_attestation":true}`
	req := httptest.NewRequest(http.MethodPost, "/session", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

func TestCreateGuestSession_NicknameTakenError(t *testing.T) {
	sessions := &mockSessionStore{}
	nicknameTaken := func(_ context.Context, _ string) (bool, error) { return false, errors.New("db") }
	h := guest.NewHandler(guest.HandlerConfig{
		Sessions:      sessions,
		NicknameTaken: nicknameTaken,
		RoomLister:    &mockManager{},
	})

	body := `{"nickname":"Wanderer","age_attestation":true}`
	req := httptest.NewRequest(http.MethodPost, "/session", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestCreateGuestSession_SessionStoreError(t *testing.T) {
	sessions := &mockSessionStore{err: errors.New("redis down")}
	nicknameTaken := func(_ context.Context, _ string) (bool, error) { return false, nil }
	h := guest.NewHandler(guest.HandlerConfig{
		Sessions:      sessions,
		NicknameTaken: nicknameTaken,
		RoomLister:    &mockManager{},
	})

	body := `{"nickname":"Wanderer","age_attestation":true}`
	req := httptest.NewRequest(http.MethodPost, "/session", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestCreateGuestSession_NicknameTooLong(t *testing.T) {
	sessions := &mockSessionStore{}
	h := guest.NewHandler(guest.HandlerConfig{
		Sessions:   sessions,
		RoomLister: &mockManager{},
	})

	longName := strings.Repeat("a", 31)
	body := `{"nickname":"` + longName + `","age_attestation":true}`
	req := httptest.NewRequest(http.MethodPost, "/session", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestListPublicRooms_Success(t *testing.T) {
	rooms := []chat.PublicRoomSummary{
		{ID: "pr-1", Name: "Lounge", Description: "Chat lounge", ActiveCount: 5},
	}
	mgr := &mockManager{rooms: rooms}
	h := guest.NewHandler(guest.HandlerConfig{
		Sessions:   &mockSessionStore{},
		RoomLister: mgr,
	})

	req := httptest.NewRequest(http.MethodGet, "/rooms", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []chat.PublicRoomSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(got) != 1 || got[0].ID != "pr-1" {
		t.Errorf("got = %v, want 1 room with ID pr-1", got)
	}
}

func TestListPublicRooms_Empty(t *testing.T) {
	mgr := &mockManager{rooms: []chat.PublicRoomSummary{}}
	h := guest.NewHandler(guest.HandlerConfig{
		Sessions:   &mockSessionStore{},
		RoomLister: mgr,
	})

	req := httptest.NewRequest(http.MethodGet, "/rooms", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []chat.PublicRoomSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestListPublicRooms_Error(t *testing.T) {
	mgr := &mockManager{err: errors.New("db error")}
	h := guest.NewHandler(guest.HandlerConfig{
		Sessions:   &mockSessionStore{},
		RoomLister: mgr,
	})

	req := httptest.NewRequest(http.MethodGet, "/rooms", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestListRoomParticipants_Success(t *testing.T) {
	mgr := &mockManager{room: &chat.Room{ID: "pr-1", Type: chat.RoomTypePublic}}
	h := guest.NewHandler(guest.HandlerConfig{
		Sessions:   &mockSessionStore{},
		RoomLister: mgr,
		Participants: func(context.Context, string) []chat.ClientInfo {
			return []chat.ClientInfo{
				{UserID: "u1", Username: "alice_handle", DisplayName: "Alice", IsGuest: false},
				{UserID: "guest:1", DisplayName: "Bob", IsGuest: true},
			}
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/rooms/pr-1/participants", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	// A registered user's @handle must never reach a guest-facing payload.
	if strings.Contains(rec.Body.String(), "alice_handle") {
		t.Error("response leaked a registered user's username")
	}
	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if _, ok := got[0]["username"]; ok {
		t.Error("participant payload must not include username")
	}
	if got[0]["display_name"] != "Alice" || got[0]["is_guest"] != false {
		t.Errorf("participant[0] = %v", got[0])
	}
	if got[1]["display_name"] != "Bob" || got[1]["is_guest"] != true {
		t.Errorf("participant[1] = %v", got[1])
	}
}

// TestGuestRoutes_MountedUnderChiPrefix guards the real wiring: the handler is
// mounted at /guest by the server, so its routes must resolve under that prefix
// (a stdlib ServeMux with relative patterns would 404 here).
func TestGuestRoutes_MountedUnderChiPrefix(t *testing.T) {
	mgr := &mockManager{rooms: []chat.PublicRoomSummary{{ID: "pr-1", Name: "Lounge"}}}
	api := chi.NewRouter()
	api.Mount("/guest", guest.NewHandler(guest.HandlerConfig{
		Sessions:   &mockSessionStore{},
		RoomLister: mgr,
	}))

	req := httptest.NewRequest(http.MethodGet, "/guest/rooms", nil)
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /guest/rooms via chi mount = %d, want 200 (routes must resolve under the /guest prefix)", rec.Code)
	}
}

func TestListRoomParticipants_NotPublic(t *testing.T) {
	mgr := &mockManager{room: &chat.Room{ID: "c-1", Type: chat.RoomTypeChannel}}
	h := guest.NewHandler(guest.HandlerConfig{
		Sessions:   &mockSessionStore{},
		RoomLister: mgr,
		Participants: func(context.Context, string) []chat.ClientInfo {
			return []chat.ClientInfo{{UserID: "u1", DisplayName: "Alice"}}
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/rooms/c-1/participants", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for a non-public room", rec.Code)
	}
}
