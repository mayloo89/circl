package chat_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mayloo89/circl/backend/internal/chat"
)

// mockStore implements chat.Store for service-layer tests.
type mockStore struct {
	room            *chat.Room
	rooms           []chat.RoomSummary
	msg             *chat.Message
	msgs            []chat.Message
	members         []string
	isMember        bool
	roomErr         error
	roomsErr        error
	msgErr          error
	msgsErr         error
	memberErr       error
	membersErr      error
	markReadTime    time.Time
	markReadErr     error
	viewOnceMsg     *chat.Message
	viewOnceKeys    []string
	viewOnceErr     error
	deleteRoomID    string
	deleteKeys      []string
	deleteErr       error
	expiredIDs      []string
	expiredErr      error
	displayName     string
	displayNameErr  error
}

func (m *mockStore) GetDisplayName(_ context.Context, _ string) (string, error) {
	return m.displayName, m.displayNameErr
}
func (m *mockStore) GetOrCreateDM(_ context.Context, _, _ string) (*chat.Room, error) {
	return m.room, m.roomErr
}
func (m *mockStore) CreateGroup(_ context.Context, _, _ string, _ []string) (*chat.Room, error) {
	return m.room, m.roomErr
}
func (m *mockStore) IsMember(_ context.Context, _, _ string) (bool, error) {
	return m.isMember, m.memberErr
}
func (m *mockStore) ListMembers(_ context.Context, _ string) ([]string, error) {
	return m.members, m.membersErr
}
func (m *mockStore) ListRooms(_ context.Context, _ string) ([]chat.RoomSummary, error) {
	return m.rooms, m.roomsErr
}
func (m *mockStore) SaveMessage(_ context.Context, _ chat.SaveMessageParams) (*chat.Message, error) {
	return m.msg, m.msgErr
}
func (m *mockStore) ListMessages(_ context.Context, _ string, _ *time.Time, _ int) ([]chat.Message, error) {
	return m.msgs, m.msgsErr
}
func (m *mockStore) MarkRead(_ context.Context, _, _ string) (time.Time, error) {
	return m.markReadTime, m.markReadErr
}
func (m *mockStore) ViewOnceMessage(_ context.Context, _, _, _ string) (*chat.Message, []string, error) {
	return m.viewOnceMsg, m.viewOnceKeys, m.viewOnceErr
}
func (m *mockStore) DeleteMessage(_ context.Context, _ string) (string, []string, error) {
	return m.deleteRoomID, m.deleteKeys, m.deleteErr
}
func (m *mockStore) TombstoneMessage(_ context.Context, _ string) (string, []string, error) {
	return m.deleteRoomID, m.deleteKeys, m.deleteErr
}
func (m *mockStore) ListExpiredMessages(_ context.Context) ([]string, error) {
	return m.expiredIDs, m.expiredErr
}

func TestService_GetOrCreateDM_Success(t *testing.T) {
	want := &chat.Room{ID: "r-1", Type: chat.RoomTypeDM}
	svc := chat.NewService(&mockStore{room: want})

	got, err := svc.GetOrCreateDM(t.Context(), "u-1", "u-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "r-1" {
		t.Errorf("ID = %q, want r-1", got.ID)
	}
}

func TestService_GetOrCreateDM_Error(t *testing.T) {
	svc := chat.NewService(&mockStore{roomErr: errors.New("db error")})

	_, err := svc.GetOrCreateDM(t.Context(), "u-1", "u-2")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestService_CreateGroup_Success(t *testing.T) {
	want := &chat.Room{ID: "r-2", Type: chat.RoomTypeGroup, Name: "team"}
	svc := chat.NewService(&mockStore{room: want})

	got, err := svc.CreateGroup(t.Context(), "u-1", "team", []string{"u-2", "u-3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "team" {
		t.Errorf("Name = %q, want team", got.Name)
	}
}

func TestService_CreateGroup_Error(t *testing.T) {
	svc := chat.NewService(&mockStore{roomErr: errors.New("db error")})
	_, err := svc.CreateGroup(t.Context(), "u-1", "team", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestService_IsMember_True(t *testing.T) {
	svc := chat.NewService(&mockStore{isMember: true})

	ok, err := svc.IsMember(t.Context(), "r-1", "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected true, got false")
	}
}

func TestService_IsMember_False(t *testing.T) {
	svc := chat.NewService(&mockStore{isMember: false})
	ok, err := svc.IsMember(t.Context(), "r-1", "u-99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected false, got true")
	}
}

func TestService_IsMember_Error(t *testing.T) {
	svc := chat.NewService(&mockStore{memberErr: errors.New("db error")})
	_, err := svc.IsMember(t.Context(), "r-1", "u-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestService_ListRooms_Success(t *testing.T) {
	want := []chat.RoomSummary{{ID: "r-1"}, {ID: "r-2"}}
	svc := chat.NewService(&mockStore{rooms: want})

	got, err := svc.ListRooms(t.Context(), "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestService_ListRooms_Error(t *testing.T) {
	svc := chat.NewService(&mockStore{roomsErr: errors.New("db error")})
	_, err := svc.ListRooms(t.Context(), "u-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestService_SaveMessage_Success(t *testing.T) {
	now := time.Now()
	want := &chat.Message{ID: "m-1", RoomID: "r-1", SenderID: "u-1", Content: "hello", CreatedAt: now}
	svc := chat.NewService(&mockStore{msg: want})

	got, err := svc.SaveMessage(t.Context(), chat.SaveMessageParams{
		RoomID: "r-1", SenderID: "u-1", Type: chat.MessageTypeText, Content: "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "m-1" {
		t.Errorf("ID = %q, want m-1", got.ID)
	}
}

func TestService_SaveMessage_Error(t *testing.T) {
	svc := chat.NewService(&mockStore{msgErr: errors.New("db error")})
	_, err := svc.SaveMessage(t.Context(), chat.SaveMessageParams{
		RoomID: "r-1", SenderID: "u-1", Type: chat.MessageTypeText, Content: "hello",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestService_ListMessages_Success(t *testing.T) {
	want := []chat.Message{{ID: "m-1"}, {ID: "m-2"}}
	svc := chat.NewService(&mockStore{msgs: want})

	got, err := svc.ListMessages(t.Context(), "r-1", nil, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestService_ListMessages_Error(t *testing.T) {
	svc := chat.NewService(&mockStore{msgsErr: errors.New("db error")})
	_, err := svc.ListMessages(t.Context(), "r-1", nil, 50)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestService_ListMembers_Success(t *testing.T) {
	want := []string{"u-1", "u-2"}
	svc := chat.NewService(&mockStore{members: want})

	got, err := svc.ListMembers(t.Context(), "r-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestService_ListMembers_Error(t *testing.T) {
	svc := chat.NewService(&mockStore{membersErr: errors.New("db error")})
	_, err := svc.ListMembers(t.Context(), "r-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestService_MarkRead_Success(t *testing.T) {
	now := time.Now()
	svc := chat.NewService(&mockStore{markReadTime: now})
	readAt, err := svc.MarkRead(t.Context(), "r-1", "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !readAt.Equal(now) {
		t.Errorf("readAt = %v, want %v", readAt, now)
	}
}

func TestService_MarkRead_Error(t *testing.T) {
	svc := chat.NewService(&mockStore{markReadErr: errors.New("db error")})
	_, err := svc.MarkRead(t.Context(), "r-1", "u-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestService_ViewOnceMessage_Success(t *testing.T) {
	msg := &chat.Message{ID: "m-1", ViewOnce: true, Content: "secret"}
	svc := chat.NewService(&mockStore{viewOnceMsg: msg, viewOnceKeys: []string{"uploads/key.jpg"}})

	got, keys, err := svc.ViewOnceMessage(t.Context(), "m-1", "r-1", "u-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "m-1" {
		t.Errorf("ID = %q, want m-1", got.ID)
	}
	if len(keys) != 1 {
		t.Errorf("keys len = %d, want 1", len(keys))
	}
}

func TestService_ViewOnceMessage_Error(t *testing.T) {
	svc := chat.NewService(&mockStore{viewOnceErr: errors.New("db error")})
	_, _, err := svc.ViewOnceMessage(t.Context(), "m-1", "r-1", "u-2")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestService_DeleteMessage_Success(t *testing.T) {
	svc := chat.NewService(&mockStore{deleteRoomID: "r-1", deleteKeys: []string{"uploads/key.jpg"}})

	roomID, keys, err := svc.DeleteMessage(t.Context(), "m-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if roomID != "r-1" {
		t.Errorf("roomID = %q, want r-1", roomID)
	}
	if len(keys) != 1 {
		t.Errorf("keys len = %d, want 1", len(keys))
	}
}

func TestService_DeleteMessage_Error(t *testing.T) {
	svc := chat.NewService(&mockStore{deleteErr: errors.New("db error")})
	_, _, err := svc.DeleteMessage(t.Context(), "m-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestService_ListExpiredMessages_Success(t *testing.T) {
	svc := chat.NewService(&mockStore{expiredIDs: []string{"m-1", "m-2"}})

	ids, err := svc.ListExpiredMessages(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("len = %d, want 2", len(ids))
	}
}

func TestService_ListExpiredMessages_Error(t *testing.T) {
	svc := chat.NewService(&mockStore{expiredErr: errors.New("db error")})
	_, err := svc.ListExpiredMessages(t.Context())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseTTL_Valid(t *testing.T) {
	cases := []struct {
		label   string
		minutes int
	}{
		{chat.TTL15Min, 15},
		{chat.TTL30Min, 30},
		{chat.TTL1Hour, 60},
		{chat.TTL6Hours, 6 * 60},
		{chat.TTL12Hours, 12 * 60},
		{chat.TTL24Hour, 24 * 60},
	}
	for _, tc := range cases {
		d, err := chat.ParseTTL(tc.label)
		if err != nil {
			t.Errorf("ParseTTL(%q): unexpected error: %v", tc.label, err)
		}
		if d.Minutes() != float64(tc.minutes) {
			t.Errorf("ParseTTL(%q) = %v, want %dm", tc.label, d, tc.minutes)
		}
	}
}

func TestParseTTL_Invalid(t *testing.T) {
	_, err := chat.ParseTTL("7d")
	if err == nil {
		t.Error("expected error for invalid TTL, got nil")
	}
}

func TestService_GetDisplayName_Found(t *testing.T) {
	svc := chat.NewService(&mockStore{displayName: "Alice"})
	name, err := svc.GetDisplayName(t.Context(), "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Alice" {
		t.Errorf("name = %q, want Alice", name)
	}
}

func TestService_GetDisplayName_Error(t *testing.T) {
	svc := chat.NewService(&mockStore{displayNameErr: errors.New("db fail")})
	_, err := svc.GetDisplayName(t.Context(), "u-1")
	if err == nil {
		t.Error("expected error, got nil")
	}
}
