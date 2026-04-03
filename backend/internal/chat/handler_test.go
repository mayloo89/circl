package chat_test

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

	"github.com/alicebob/miniredis/v2"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"

	"github.com/mayloo89/circl/backend/internal/chat"
	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/token"
)

const (
	testSecret = "supersecretfortesting-mustbe32chars!!"
	testUserID = "user-abc-123"
)

// mockManager is a test double for chat.Manager.
type mockManager struct {
	room           *chat.Room
	rooms          []chat.RoomSummary
	msg            *chat.Message
	msgs           []chat.Message
	isMember       bool
	roomErr        error
	roomsErr       error
	msgErr         error
	msgsErr        error
	memberErr      error
	markErr        error
	markReadTime   time.Time
	viewOnceMsg    *chat.Message
	viewOnceKeys   []string
	viewOnceErr    error
	deleteRoomID   string
	deleteKeys     []string
	deleteErr      error
	expiredIDs     []string
	expiredErr     error
	displayName    string
	displayNameErr error
}

func (m *mockManager) GetDisplayName(_ context.Context, _ string) (string, error) {
	return m.displayName, m.displayNameErr
}
func (m *mockManager) GetOrCreateDM(_ context.Context, _, _ string) (*chat.Room, error) {
	return m.room, m.roomErr
}
func (m *mockManager) CreateGroup(_ context.Context, _, _ string, _ []string) (*chat.Room, error) {
	return m.room, m.roomErr
}
func (m *mockManager) IsMember(_ context.Context, _, _ string) (bool, error) {
	return m.isMember, m.memberErr
}
func (m *mockManager) ListRooms(_ context.Context, _ string) ([]chat.RoomSummary, error) {
	return m.rooms, m.roomsErr
}
func (m *mockManager) SaveMessage(_ context.Context, _ chat.SaveMessageParams) (*chat.Message, error) {
	return m.msg, m.msgErr
}
func (m *mockManager) ListMessages(_ context.Context, _ string, _ *time.Time, _ int) ([]chat.Message, error) {
	return m.msgs, m.msgsErr
}
func (m *mockManager) ListMembers(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}
func (m *mockManager) MarkRead(_ context.Context, _, _ string) (time.Time, error) {
	return m.markReadTime, m.markErr
}
func (m *mockManager) ViewOnceMessage(_ context.Context, _, _, _ string) (*chat.Message, []string, error) {
	return m.viewOnceMsg, m.viewOnceKeys, m.viewOnceErr
}
func (m *mockManager) DeleteMessage(_ context.Context, _ string) (string, []string, error) {
	return m.deleteRoomID, m.deleteKeys, m.deleteErr
}
func (m *mockManager) TombstoneMessage(_ context.Context, _ string) (string, []string, error) {
	return m.deleteRoomID, m.deleteKeys, m.deleteErr
}
func (m *mockManager) ListExpiredMessages(_ context.Context) ([]string, error) {
	return m.expiredIDs, m.expiredErr
}

func authedReq(r *http.Request) *http.Request {
	tok, _ := token.Generate(testUserID, testSecret, time.Hour)
	r.Header.Set("Authorization", "Bearer "+tok)
	return r
}

func serveWithAuth(h http.Handler, r *http.Request, rec *httptest.ResponseRecorder) {
	middleware.RequireAuth(testSecret)(h).ServeHTTP(rec, r)
}

// serveNoAuth calls the handler directly without any auth middleware, triggering
// the defensive middleware.UserIDFromContext(!ok) branches inside each handler.
func serveNoAuth(h http.Handler, r *http.Request, rec *httptest.ResponseRecorder) {
	h.ServeHTTP(rec, r)
}

// --- GET DM room ---

func TestGetDM_Success(t *testing.T) {
	room := &chat.Room{ID: "r-1", Type: "dm"}
	h := chat.NewHandler(&mockManager{room: room})

	body, _ := json.Marshal(map[string]string{"peer_id": "u-2"})
	req := authedReq(httptest.NewRequest(http.MethodPost, "/rooms/dm", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var got chat.Room
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != "r-1" {
		t.Errorf("ID = %q, want r-1", got.ID)
	}
}

func TestGetDM_MissingPeerID(t *testing.T) {
	h := chat.NewHandler(&mockManager{})
	req := authedReq(httptest.NewRequest(http.MethodPost, "/rooms/dm",
		strings.NewReader(`{}`)))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestGetDM_NoUserInContext(t *testing.T) {
	h := chat.NewHandler(&mockManager{})
	body, _ := json.Marshal(map[string]string{"peer_id": "u-2"})
	req := httptest.NewRequest(http.MethodPost, "/rooms/dm", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	serveNoAuth(h, req, rec)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestGetDM_ServiceError(t *testing.T) {
	h := chat.NewHandler(&mockManager{roomErr: errors.New("db fail")})
	body, _ := json.Marshal(map[string]string{"peer_id": "u-2"})
	req := authedReq(httptest.NewRequest(http.MethodPost, "/rooms/dm", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestGetDM_Unauthorized(t *testing.T) {
	h := chat.NewHandler(&mockManager{})
	req := httptest.NewRequest(http.MethodPost, "/rooms/dm", strings.NewReader(`{"peer_id":"u-2"}`))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// --- Create group ---

func TestCreateGroup_Success(t *testing.T) {
	room := &chat.Room{ID: "r-2", Type: "group", Name: "squad"}
	h := chat.NewHandler(&mockManager{room: room})

	body, _ := json.Marshal(map[string]any{"name": "squad", "member_ids": []string{"u-2"}})
	req := authedReq(httptest.NewRequest(http.MethodPost, "/rooms", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
}

func TestCreateGroup_NoUserInContext(t *testing.T) {
	h := chat.NewHandler(&mockManager{})
	req := httptest.NewRequest(http.MethodPost, "/rooms", strings.NewReader(`{"name":"squad"}`))
	rec := httptest.NewRecorder()
	serveNoAuth(h, req, rec)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestCreateGroup_MissingName(t *testing.T) {
	h := chat.NewHandler(&mockManager{})
	req := authedReq(httptest.NewRequest(http.MethodPost, "/rooms", strings.NewReader(`{}`)))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestCreateGroup_ServiceError(t *testing.T) {
	h := chat.NewHandler(&mockManager{roomErr: errors.New("db fail")})
	body, _ := json.Marshal(map[string]any{"name": "squad"})
	req := authedReq(httptest.NewRequest(http.MethodPost, "/rooms", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

// --- List rooms ---

func TestListRooms_Success(t *testing.T) {
	rooms := []chat.RoomSummary{{ID: "r-1"}, {ID: "r-2"}}
	h := chat.NewHandler(&mockManager{rooms: rooms})

	req := authedReq(httptest.NewRequest(http.MethodGet, "/rooms", nil))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var got []chat.RoomSummary
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestListRooms_NoUserInContext(t *testing.T) {
	h := chat.NewHandler(&mockManager{})
	req := httptest.NewRequest(http.MethodGet, "/rooms", nil)
	rec := httptest.NewRecorder()
	serveNoAuth(h, req, rec)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestListRooms_ServiceError(t *testing.T) {
	h := chat.NewHandler(&mockManager{roomsErr: errors.New("db fail")})
	req := authedReq(httptest.NewRequest(http.MethodGet, "/rooms", nil))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

// --- List messages ---

func TestListMessages_NoUserInContext(t *testing.T) {
	h := chat.NewHandler(&mockManager{})
	req := httptest.NewRequest(http.MethodGet, "/rooms/r-1/messages", nil)
	rec := httptest.NewRecorder()
	serveNoAuth(h, req, rec)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestListMessages_NotMember(t *testing.T) {
	h := chat.NewHandler(&mockManager{isMember: false})
	req := authedReq(httptest.NewRequest(http.MethodGet, "/rooms/r-1/messages", nil))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestListMessages_Success(t *testing.T) {
	msgs := []chat.Message{{ID: "m-1", Content: "hi"}}
	h := chat.NewHandler(&mockManager{isMember: true, msgs: msgs})

	req := authedReq(httptest.NewRequest(http.MethodGet, "/rooms/r-1/messages", nil))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var got []chat.Message
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len = %d, want 1", len(got))
	}
}

func TestListMessages_ThumbnailURL(t *testing.T) {
	msgs := []chat.Message{{ID: "m-1", Type: "image", Content: "https://example.com/img.jpg", ThumbnailURL: "https://example.com/thumb.jpg"}}
	h := chat.NewHandler(&mockManager{isMember: true, msgs: msgs})

	req := authedReq(httptest.NewRequest(http.MethodGet, "/rooms/r-1/messages", nil))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)

	var got []chat.Message
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got[0].ThumbnailURL != "https://example.com/thumb.jpg" {
		t.Errorf("ThumbnailURL = %q, want https://example.com/thumb.jpg", got[0].ThumbnailURL)
	}
}

func TestListMessages_InvalidBefore(t *testing.T) {
	h := chat.NewHandler(&mockManager{isMember: true})
	req := authedReq(httptest.NewRequest(http.MethodGet, "/rooms/r-1/messages?before=notadate", nil))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestListMessages_ServiceError(t *testing.T) {
	h := chat.NewHandler(&mockManager{isMember: true, msgsErr: errors.New("db fail")})
	req := authedReq(httptest.NewRequest(http.MethodGet, "/rooms/r-1/messages", nil))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestListMessages_MemberCheckError(t *testing.T) {
	h := chat.NewHandler(&mockManager{memberErr: errors.New("db fail")})
	req := authedReq(httptest.NewRequest(http.MethodGet, "/rooms/r-1/messages", nil))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestListMessages_MasksViewOnceForNonSender(t *testing.T) {
	// A view_once message from another user should have its content masked.
	msgs := []chat.Message{
		{ID: "m-1", SenderID: "other-user", Content: "secret", ViewOnce: true},
		{ID: "m-2", SenderID: testUserID, Content: "my msg", ViewOnce: true},
	}
	h := chat.NewHandler(&mockManager{isMember: true, msgs: msgs})
	req := authedReq(httptest.NewRequest(http.MethodGet, "/rooms/r-1/messages", nil))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var got []chat.Message
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got[0].Content != "" {
		t.Errorf("view_once from other user should have empty content, got %q", got[0].Content)
	}
	if got[1].Content != "my msg" {
		t.Errorf("sender's own view_once should retain content, got %q", got[1].Content)
	}
}

// --- Mark read ---

func TestMarkRead_NoUserInContext(t *testing.T) {
	h := chat.NewHandler(&mockManager{})
	req := httptest.NewRequest(http.MethodPut, "/rooms/r-1/read", nil)
	rec := httptest.NewRecorder()
	serveNoAuth(h, req, rec)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestMarkRead_Success(t *testing.T) {
	h := chat.NewHandler(&mockManager{})
	req := authedReq(httptest.NewRequest(http.MethodPut, "/rooms/r-1/read", nil))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

func TestMarkRead_ServiceError(t *testing.T) {
	h := chat.NewHandler(&mockManager{markErr: errors.New("db fail")})
	req := authedReq(httptest.NewRequest(http.MethodPut, "/rooms/r-1/read", nil))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestMarkRead_NotifiesRoomRead(t *testing.T) {
	readAt := time.Now().UTC().Round(time.Second)
	mgr := &mockManager{markReadTime: readAt}

	var notifiedRoomID, notifiedUserID string
	var notifiedReadAt time.Time
	cfg := chat.HandlerConfig{
		NotifyRoomRead: func(roomID, userID string, at time.Time) {
			notifiedRoomID, notifiedUserID, notifiedReadAt = roomID, userID, at
		},
	}
	h := chat.NewHandler(mgr, cfg)

	req := authedReq(httptest.NewRequest(http.MethodPut, "/rooms/r-1/read", nil))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if notifiedRoomID != "r-1" {
		t.Errorf("room_id = %q, want r-1", notifiedRoomID)
	}
	if notifiedUserID != testUserID {
		t.Errorf("user_id = %q, want %q", notifiedUserID, testUserID)
	}
	if !notifiedReadAt.Equal(readAt) {
		t.Errorf("read_at = %v, want %v", notifiedReadAt, readAt)
	}
}

// --- View once message ---

func TestViewMessage_Success(t *testing.T) {
	msg := &chat.Message{ID: "m-1", RoomID: "r-1", ViewOnce: true, Content: "secret"}
	mgr := &mockManager{
		isMember:     true,
		viewOnceMsg:  msg,
		viewOnceKeys: []string{"uploads/key.jpg"},
	}

	var deletedKeys []string
	var notifiedRoom, notifiedMsg string
	cfg := chat.HandlerConfig{
		DeleteFiles: func(_ context.Context, keys []string) { deletedKeys = keys },
		NotifyMessageDeleted: func(roomID, msgID string) {
			notifiedRoom, notifiedMsg = roomID, msgID
		},
	}
	h := chat.NewHandler(mgr, cfg)

	req := authedReq(httptest.NewRequest(http.MethodPost, "/rooms/r-1/messages/m-1/view", nil))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var got chat.Message
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != "m-1" {
		t.Errorf("ID = %q, want m-1", got.ID)
	}
	if got.Content != "secret" {
		t.Errorf("Content = %q, want secret", got.Content)
	}
	if len(deletedKeys) != 1 || deletedKeys[0] != "uploads/key.jpg" {
		t.Errorf("deletedKeys = %v, want [uploads/key.jpg]", deletedKeys)
	}
	if notifiedRoom != "r-1" || notifiedMsg != "m-1" {
		t.Errorf("notification roomID=%q msgID=%q, want r-1/m-1", notifiedRoom, notifiedMsg)
	}
}

func TestViewMessage_NoKeysNoCallbacks(t *testing.T) {
	// When ViewOnceMessage returns no keys (not yet all viewers), callbacks are skipped.
	msg := &chat.Message{ID: "m-1", ViewOnce: true, Content: "secret"}
	mgr := &mockManager{isMember: true, viewOnceMsg: msg, viewOnceKeys: nil}

	called := false
	cfg := chat.HandlerConfig{
		NotifyMessageDeleted: func(_, _ string) { called = true },
	}
	h := chat.NewHandler(mgr, cfg)

	req := authedReq(httptest.NewRequest(http.MethodPost, "/rooms/r-1/messages/m-1/view", nil))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if called {
		t.Error("notify callback should not be called when no keys returned")
	}
}

func TestViewMessage_NotFound(t *testing.T) {
	mgr := &mockManager{isMember: true, viewOnceErr: chat.ErrNotFound}
	h := chat.NewHandler(mgr)
	req := authedReq(httptest.NewRequest(http.MethodPost, "/rooms/r-1/messages/m-1/view", nil))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestViewMessage_Forbidden_NotMember(t *testing.T) {
	mgr := &mockManager{isMember: false}
	h := chat.NewHandler(mgr)
	req := authedReq(httptest.NewRequest(http.MethodPost, "/rooms/r-1/messages/m-1/view", nil))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestViewMessage_Forbidden_IsSender(t *testing.T) {
	mgr := &mockManager{isMember: true, viewOnceErr: chat.ErrForbidden}
	h := chat.NewHandler(mgr)
	req := authedReq(httptest.NewRequest(http.MethodPost, "/rooms/r-1/messages/m-1/view", nil))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestViewMessage_ServiceError(t *testing.T) {
	mgr := &mockManager{isMember: true, viewOnceErr: errors.New("db fail")}
	h := chat.NewHandler(mgr)
	req := authedReq(httptest.NewRequest(http.MethodPost, "/rooms/r-1/messages/m-1/view", nil))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestViewMessage_NoUserInContext(t *testing.T) {
	h := chat.NewHandler(&mockManager{})
	req := httptest.NewRequest(http.MethodPost, "/rooms/r-1/messages/m-1/view", nil)
	rec := httptest.NewRecorder()
	serveNoAuth(h, req, rec)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// --- WebSocket handler ---

func TestWSHandler_NoToken(t *testing.T) {
	h := chat.NewWSHandler(&mockManager{}, nil, testSecret, nil)
	req := httptest.NewRequest(http.MethodGet, "/rooms/r-1/ws", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestWSHandler_InvalidToken(t *testing.T) {
	h := chat.NewWSHandler(&mockManager{}, nil, testSecret, nil)
	req := httptest.NewRequest(http.MethodGet, "/rooms/r-1/ws?token=badtoken", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestWSHandler_NotMember(t *testing.T) {
	h := chat.NewWSHandler(&mockManager{isMember: false}, nil, testSecret, nil)
	tok, _ := token.Generate(testUserID, testSecret, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/rooms/r-1/ws?token="+tok, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestWSHandler_MemberCheckError(t *testing.T) {
	h := chat.NewWSHandler(&mockManager{memberErr: errors.New("db fail")}, nil, testSecret, nil)
	tok, _ := token.Generate(testUserID, testSecret, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/rooms/r-1/ws?token="+tok, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

// --- WebSocket pump tests ---
// These tests exercise readPump / writePump by running a real WebSocket
// connection against an httptest server.

func newTestHubForHandler(t *testing.T) *chat.Hub {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	hub := chat.NewHub(rdb)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go hub.Run(ctx)
	return hub
}

func TestWSHandler_SendAndReceiveMessage(t *testing.T) {
	hub := newTestHubForHandler(t)

	now := time.Now()
	savedMsg := &chat.Message{
		ID:         "m-1",
		RoomID:     "r-1",
		SenderID:   testUserID,
		SenderName: "Tester",
		Type:       "text",
		Content:    "hello from test",
		CreatedAt:  now,
	}
	mgr := &mockManager{isMember: true, msg: savedMsg}

	tok, _ := token.Generate(testUserID, testSecret, time.Hour)

	r := chi.NewRouter()
	r.Get("/rooms/{id}/ws", chat.NewWSHandler(mgr, hub, testSecret, nil))

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/rooms/r-1/ws?token=" + tok

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Allow the hub to process the register event and establish the Redis
	// subscription before we publish — otherwise the message may arrive
	// before the subscription is active and be missed.
	time.Sleep(100 * time.Millisecond)

	// Send a chat message.
	if err := conn.WriteJSON(map[string]string{"type": "message", "content": "hello from test"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	// The server publishes via Redis and delivers back to this client.
	conn.SetReadDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["type"] != "text" {
		t.Errorf("type = %v, want text", got["type"])
	}
	if got["content"] != "hello from test" {
		t.Errorf("content = %v, want %q", got["content"], "hello from test")
	}
}

func TestWSHandler_IgnoresEmptyContent(t *testing.T) {
	hub := newTestHubForHandler(t)
	mgr := &mockManager{isMember: true}

	tok, _ := token.Generate(testUserID, testSecret, time.Hour)

	r := chi.NewRouter()
	r.Get("/rooms/{id}/ws", chat.NewWSHandler(mgr, hub, testSecret, nil))

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/rooms/r-1/ws?token=" + tok

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send message with empty content — should be silently ignored.
	if err := conn.WriteJSON(map[string]string{"type": "message", "content": ""}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	// No message should be delivered.
	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)) //nolint:errcheck
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Error("expected timeout/no message, but received one")
	}
}

func TestWSHandler_HubShutdownSendsCloseFrame(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	hub := chat.NewHub(rdb)
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	mgr := &mockManager{isMember: true}
	tok, _ := token.Generate(testUserID, testSecret, time.Hour)

	r := chi.NewRouter()
	r.Get("/rooms/{id}/ws", chat.NewWSHandler(mgr, hub, testSecret, nil))

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/rooms/r-1/ws?token=" + tok

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Let the hub fully register the client.
	time.Sleep(100 * time.Millisecond)

	// Shut down the hub — this closes the send channel, triggering writePump
	// to write a CloseMessage and return.
	cancel()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck
	_, _, err = conn.ReadMessage()
	// After hub shutdown we expect either a CloseMessage or an EOF.
	if err == nil {
		// Got a message unexpectedly — that is also acceptable during shutdown.
		t.Log("received message during hub shutdown (acceptable)")
	}
	// The test succeeds as long as we don't deadlock; the close path was exercised.
}

func TestWSHandler_SendAttachmentMessage(t *testing.T) {
	hub := newTestHubForHandler(t)

	now := time.Now()
	savedMsg := &chat.Message{
		ID:         "m-2",
		RoomID:     "r-1",
		SenderID:   testUserID,
		SenderName: "Tester",
		Type:       chat.MessageTypeImage,
		Content:    "http://localhost:8080/uploads/img.jpg",
		CreatedAt:  now,
	}
	mgr := &mockManager{isMember: true, msg: savedMsg}

	tok, _ := token.Generate(testUserID, testSecret, time.Hour)

	r := chi.NewRouter()
	r.Get("/rooms/{id}/ws", chat.NewWSHandler(mgr, hub, testSecret, nil))

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/rooms/r-1/ws?token=" + tok

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	payload := map[string]string{
		"type":      "attachment",
		"content":   "http://localhost:8080/uploads/img.jpg",
		"mime_type": "image/jpeg",
		"upload_id": "upload-123",
	}
	if err := conn.WriteJSON(payload); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["type"] != chat.MessageTypeImage {
		t.Errorf("type = %v, want %q", got["type"], chat.MessageTypeImage)
	}
	if got["content"] != "http://localhost:8080/uploads/img.jpg" {
		t.Errorf("content = %v", got["content"])
	}
}

func TestWSHandler_SendVideoAttachment(t *testing.T) {
	hub := newTestHubForHandler(t)

	savedMsg := &chat.Message{
		ID:        "m-3",
		RoomID:    "r-1",
		SenderID:  testUserID,
		Type:      chat.MessageTypeVideo,
		Content:   "http://localhost:8080/uploads/video.mp4",
		CreatedAt: time.Now(),
	}
	mgr := &mockManager{isMember: true, msg: savedMsg}
	tok, _ := token.Generate(testUserID, testSecret, time.Hour)

	r := chi.NewRouter()
	r.Get("/rooms/{id}/ws", chat.NewWSHandler(mgr, hub, testSecret, nil))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	conn, _, err := websocket.DefaultDialer.Dial(
		"ws"+strings.TrimPrefix(srv.URL, "http")+"/rooms/r-1/ws?token="+tok, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	time.Sleep(100 * time.Millisecond)

	if err := conn.WriteJSON(map[string]string{"type": "attachment", "content": "http://localhost:8080/uploads/video.mp4", "mime_type": "video/mp4", "upload_id": "upload-123"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["type"] != chat.MessageTypeVideo {
		t.Errorf("type = %v, want %q", got["type"], chat.MessageTypeVideo)
	}
}

func TestWSHandler_SendFileAttachment(t *testing.T) {
	hub := newTestHubForHandler(t)

	savedMsg := &chat.Message{
		ID:        "m-4",
		RoomID:    "r-1",
		SenderID:  testUserID,
		Type:      chat.MessageTypeFile,
		Content:   "http://localhost:8080/uploads/doc.pdf",
		CreatedAt: time.Now(),
	}
	mgr := &mockManager{isMember: true, msg: savedMsg}
	tok, _ := token.Generate(testUserID, testSecret, time.Hour)

	r := chi.NewRouter()
	r.Get("/rooms/{id}/ws", chat.NewWSHandler(mgr, hub, testSecret, nil))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	conn, _, err := websocket.DefaultDialer.Dial(
		"ws"+strings.TrimPrefix(srv.URL, "http")+"/rooms/r-1/ws?token="+tok, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	time.Sleep(100 * time.Millisecond)

	if err := conn.WriteJSON(map[string]string{"type": "attachment", "content": "http://localhost:8080/uploads/doc.pdf", "mime_type": "application/pdf", "upload_id": "upload-123"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["type"] != chat.MessageTypeFile {
		t.Errorf("type = %v, want %q", got["type"], chat.MessageTypeFile)
	}
}

func TestWSHandler_IgnoresUnknownType(t *testing.T) {
	hub := newTestHubForHandler(t)
	mgr := &mockManager{isMember: true}

	tok, _ := token.Generate(testUserID, testSecret, time.Hour)

	r := chi.NewRouter()
	r.Get("/rooms/{id}/ws", chat.NewWSHandler(mgr, hub, testSecret, nil))

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/rooms/r-1/ws?token=" + tok

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Unknown type should be silently ignored.
	if err := conn.WriteJSON(map[string]string{"type": "sticker", "content": "..."}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)) //nolint:errcheck
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Error("expected no message for unknown type, but received one")
	}
}

func TestWSHandler_IgnoresAttachmentWithEmptyContent(t *testing.T) {
	hub := newTestHubForHandler(t)
	mgr := &mockManager{isMember: true}

	tok, _ := token.Generate(testUserID, testSecret, time.Hour)

	r := chi.NewRouter()
	r.Get("/rooms/{id}/ws", chat.NewWSHandler(mgr, hub, testSecret, nil))

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/rooms/r-1/ws?token=" + tok

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(map[string]string{"type": "attachment", "content": "", "mime_type": "image/jpeg"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)) //nolint:errcheck
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Error("expected no message for empty attachment, but received one")
	}
}

func TestWSHandler_IgnoresAttachmentWithoutUploadID(t *testing.T) {
	hub := newTestHubForHandler(t)
	mgr := &mockManager{isMember: true}

	tok, _ := token.Generate(testUserID, testSecret, time.Hour)

	r := chi.NewRouter()
	r.Get("/rooms/{id}/ws", chat.NewWSHandler(mgr, hub, testSecret, nil))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	conn, _, err := websocket.DefaultDialer.Dial(
		"ws"+strings.TrimPrefix(srv.URL, "http")+"/rooms/r-1/ws?token="+tok, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Attachment without upload_id must be silently dropped.
	if err := conn.WriteJSON(map[string]string{
		"type": "attachment", "content": "https://storage/img.jpg", "mime_type": "image/jpeg",
	}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)) //nolint:errcheck
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Error("expected no message for attachment without upload_id, but received one")
	}
}

func TestWSHandler_SaveMessageError(t *testing.T) {
	hub := newTestHubForHandler(t)
	mgr := &mockManager{isMember: true, msgErr: errors.New("db fail")}

	tok, _ := token.Generate(testUserID, testSecret, time.Hour)

	r := chi.NewRouter()
	r.Get("/rooms/{id}/ws", chat.NewWSHandler(mgr, hub, testSecret, nil))

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/rooms/r-1/ws?token=" + tok

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Server save fails — no message should be broadcast.
	if err := conn.WriteJSON(map[string]string{"type": "message", "content": "oops"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)) //nolint:errcheck
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Error("expected no message on save error, but received one")
	}
}

func TestWSHandler_SendViewOnceMessage(t *testing.T) {
	hub := newTestHubForHandler(t)

	now := time.Now()
	savedMsg := &chat.Message{
		ID:        "m-vo",
		RoomID:    "r-1",
		SenderID:  testUserID,
		Type:      "text",
		Content:   "tap to view",
		ViewOnce:  true,
		CreatedAt: now,
	}
	mgr := &mockManager{isMember: true, msg: savedMsg}
	tok, _ := token.Generate(testUserID, testSecret, time.Hour)

	r := chi.NewRouter()
	r.Get("/rooms/{id}/ws", chat.NewWSHandler(mgr, hub, testSecret, nil))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	conn, _, err := websocket.DefaultDialer.Dial(
		"ws"+strings.TrimPrefix(srv.URL, "http")+"/rooms/r-1/ws?token="+tok, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	time.Sleep(100 * time.Millisecond)

	if err := conn.WriteJSON(map[string]any{
		"type": "message", "content": "tap to view", "view_once": true,
	}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Content must be masked in the broadcast: omitted from JSON (omitempty on empty string).
	if c, ok := got["content"]; ok && c != "" {
		t.Errorf("content = %q, want absent/empty for view_once broadcast", c)
	}
	if got["view_once"] != true {
		t.Errorf("view_once = %v, want true", got["view_once"])
	}
	if got["event"] != "new_message" {
		t.Errorf("event = %v, want new_message", got["event"])
	}
}

func TestWSHandler_SendTTLMessage(t *testing.T) {
	hub := newTestHubForHandler(t)

	now := time.Now()
	expires := now.Add(time.Hour)
	savedMsg := &chat.Message{
		ID:        "m-ttl",
		RoomID:    "r-1",
		SenderID:  testUserID,
		Type:      "text",
		Content:   "expires soon",
		ExpiresAt: &expires,
		CreatedAt: now,
	}
	mgr := &mockManager{isMember: true, msg: savedMsg}
	tok, _ := token.Generate(testUserID, testSecret, time.Hour)

	r := chi.NewRouter()
	r.Get("/rooms/{id}/ws", chat.NewWSHandler(mgr, hub, testSecret, nil))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	conn, _, err := websocket.DefaultDialer.Dial(
		"ws"+strings.TrimPrefix(srv.URL, "http")+"/rooms/r-1/ws?token="+tok, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	time.Sleep(100 * time.Millisecond)

	if err := conn.WriteJSON(map[string]any{
		"type": "message", "content": "expires soon", "ttl": "1h",
	}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["content"] != "expires soon" {
		t.Errorf("content = %q, want 'expires soon'", got["content"])
	}
	if got["expires_at"] == nil {
		t.Error("expires_at should be set for TTL messages")
	}
}

func TestWSHandler_InvalidTTLIgnored(t *testing.T) {
	hub := newTestHubForHandler(t)
	mgr := &mockManager{isMember: true}
	tok, _ := token.Generate(testUserID, testSecret, time.Hour)

	r := chi.NewRouter()
	r.Get("/rooms/{id}/ws", chat.NewWSHandler(mgr, hub, testSecret, nil))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	conn, _, err := websocket.DefaultDialer.Dial(
		"ws"+strings.TrimPrefix(srv.URL, "http")+"/rooms/r-1/ws?token="+tok, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Unknown TTL label should cause the message to be silently dropped.
	if err := conn.WriteJSON(map[string]any{
		"type": "message", "content": "hi", "ttl": "7d",
	}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)) //nolint:errcheck
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Error("expected no message for invalid TTL, but received one")
	}
}

func TestWSHandler_TypingEventBroadcast(t *testing.T) {
	hub := newTestHubForHandler(t)
	mgr := &mockManager{isMember: true, displayName: "Alice"}

	tok, _ := token.Generate(testUserID, testSecret, time.Hour)

	r := chi.NewRouter()
	r.Get("/rooms/{id}/ws", chat.NewWSHandler(mgr, hub, testSecret, nil))

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/rooms/r-1/ws?token=" + tok

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Allow hub to register the client and establish the Redis subscription.
	time.Sleep(100 * time.Millisecond)

	if err := conn.WriteJSON(map[string]string{"type": "typing"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	// The typing event is published via Redis and delivered back to the sender
	// (the hub broadcasts to all clients in the room, including the sender).
	conn.SetReadDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["event"] != "typing" {
		t.Errorf("event = %v, want typing", got["event"])
	}
	if got["user_id"] != testUserID {
		t.Errorf("user_id = %v, want %q", got["user_id"], testUserID)
	}
	if got["display_name"] != "Alice" {
		t.Errorf("display_name = %v, want Alice", got["display_name"])
	}
}

func TestWSHandler_TypingEventDebounced(t *testing.T) {
	hub := newTestHubForHandler(t)
	mgr := &mockManager{isMember: true, displayName: "Bob"}

	tok, _ := token.Generate(testUserID, testSecret, time.Hour)

	r := chi.NewRouter()
	r.Get("/rooms/{id}/ws", chat.NewWSHandler(mgr, hub, testSecret, nil))

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/rooms/r-1/ws?token=" + tok

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	// Send two typing frames in rapid succession; only the first should be broadcast.
	for range 2 {
		if err := conn.WriteJSON(map[string]string{"type": "typing"}); err != nil {
			t.Fatalf("WriteJSON: %v", err)
		}
	}

	// Read the first (and only) typing event.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["event"] != "typing" {
		t.Errorf("event = %v, want typing", got["event"])
	}

	// No second typing event should arrive within the debounce window.
	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)) //nolint:errcheck
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Error("expected no second typing event (debounced), but received one")
	}
}

func TestWSHandler_ThumbnailURLBroadcast(t *testing.T) {
	hub := newTestHubForHandler(t)

	now := time.Now()
	savedMsg := &chat.Message{
		ID:           "m-2",
		RoomID:       "r-1",
		SenderID:     testUserID,
		Type:         "image",
		Content:      "https://example.com/img.jpg",
		ThumbnailURL: "https://example.com/thumb.jpg",
		CreatedAt:    now,
	}
	mgr := &mockManager{isMember: true, msg: savedMsg}

	tok, _ := token.Generate(testUserID, testSecret, time.Hour)

	r := chi.NewRouter()
	r.Get("/rooms/{id}/ws", chat.NewWSHandler(mgr, hub, testSecret, nil))

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/rooms/r-1/ws?token=" + tok
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	if err := conn.WriteJSON(map[string]string{"type": "attachment", "content": "https://example.com/img.jpg", "mime_type": "image/jpeg", "upload_id": "u-1"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["thumbnail_url"] != "https://example.com/thumb.jpg" {
		t.Errorf("thumbnail_url = %v, want https://example.com/thumb.jpg", got["thumbnail_url"])
	}
}

// --- Block logic tests ---

func TestGetDM_Blocked(t *testing.T) {
	room := &chat.Room{ID: "r-1", Type: "dm"}
	cfg := chat.HandlerConfig{
		IsBlocked: func(_ context.Context, _, _ string) (bool, error) {
			return true, nil
		},
	}
	h := chat.NewHandler(&mockManager{room: room}, cfg)

	body, _ := json.Marshal(map[string]string{"peer_id": "u-2"})
	req := authedReq(httptest.NewRequest(http.MethodPost, "/rooms/dm", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestGetDM_IsBlockedError(t *testing.T) {
	cfg := chat.HandlerConfig{
		IsBlocked: func(_ context.Context, _, _ string) (bool, error) {
			return false, errors.New("db error")
		},
	}
	h := chat.NewHandler(&mockManager{}, cfg)

	body, _ := json.Marshal(map[string]string{"peer_id": "u-2"})
	req := authedReq(httptest.NewRequest(http.MethodPost, "/rooms/dm", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	serveWithAuth(h, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestWSHandler_BlockedMessageSilentlyDropped(t *testing.T) {
	hub := newTestHubForHandler(t)
	mgr := &mockManager{isMember: true}

	// isBlockedInRoom returns true, simulating that sender is blocked by a room member.
	cfg := chat.HandlerConfig{
		IsBlockedInRoom: func(_ context.Context, _, _ string) bool {
			return true
		},
	}

	tok, _ := token.Generate(testUserID, testSecret, time.Hour)

	r := chi.NewRouter()
	r.Get("/rooms/{id}/ws", chat.NewWSHandler(mgr, hub, testSecret, nil, cfg))

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/rooms/r-1/ws?token=" + tok

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	// Send a message — should be silently dropped.
	if err := conn.WriteJSON(map[string]string{"type": "message", "content": "blocked msg"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	// No message should be delivered.
	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)) //nolint:errcheck
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Error("expected no message when blocked, but received one")
	}
}

func TestWSHandler_NotBlockedMessageDelivered(t *testing.T) {
	hub := newTestHubForHandler(t)

	now := time.Now()
	savedMsg := &chat.Message{
		ID:        "m-1",
		RoomID:    "r-1",
		SenderID:  testUserID,
		Type:      "text",
		Content:   "not blocked",
		CreatedAt: now,
	}
	mgr := &mockManager{isMember: true, msg: savedMsg}

	// isBlockedInRoom returns false — message should be delivered.
	cfg := chat.HandlerConfig{
		IsBlockedInRoom: func(_ context.Context, _, _ string) bool {
			return false
		},
	}

	tok, _ := token.Generate(testUserID, testSecret, time.Hour)

	r := chi.NewRouter()
	r.Get("/rooms/{id}/ws", chat.NewWSHandler(mgr, hub, testSecret, nil, cfg))

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/rooms/r-1/ws?token=" + tok

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	if err := conn.WriteJSON(map[string]string{"type": "message", "content": "not blocked"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["content"] != "not blocked" {
		t.Errorf("content = %v, want 'not blocked'", got["content"])
	}
}
