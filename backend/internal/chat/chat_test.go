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
	room             *chat.Room
	rooms            []chat.RoomSummary
	channels         []chat.ChannelSummary
	msg              *chat.Message
	msgs             []chat.Message
	members          []string
	memberProfiles   []chat.MemberProfile
	isMember         bool
	roomErr          error
	roomsErr         error
	channelsErr      error
	msgErr           error
	msgsErr          error
	memberErr        error
	membersErr       error
	memberProfileErr error
	groupMemberErr   error
	updateGroupErr   error
	markReadTime     time.Time
	markReadErr      error
	viewOnceMsg      *chat.Message
	viewOnceKeys     []string
	viewOnceErr      error
	deleteRoomID     string
	deleteKeys       []string
	deleteErr        error
	expiredIDs       []string
	expiredErr       error
	retentionIDs     []string
	retentionErr     error
	displayName      string
	displayNameErr   error
	savedParams      *chat.SaveMessageParams
}

func (m *mockStore) GetDisplayName(_ context.Context, _ string) (string, error) {
	return m.displayName, m.displayNameErr
}
func (m *mockStore) GetAvatarURL(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (m *mockStore) GetUsername(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (m *mockStore) GetDMPeerID(_ context.Context, _, _ string) (string, error) {
	return "peer-1", nil
}
func (m *mockStore) GetOrCreateDM(_ context.Context, _, _ string) (*chat.Room, error) {
	return m.room, m.roomErr
}
func (m *mockStore) CreateGroup(_ context.Context, _, _ string, _ []string) (*chat.Room, error) {
	return m.room, m.roomErr
}
func (m *mockStore) GetRoom(_ context.Context, _ string) (*chat.Room, error) {
	return m.room, m.roomErr
}
func (m *mockStore) CreateChannel(_ context.Context, _, _, _ string) (*chat.Room, error) {
	return m.room, m.roomErr
}
func (m *mockStore) ListChannels(_ context.Context) ([]chat.ChannelSummary, error) {
	return m.channels, m.channelsErr
}
func (m *mockStore) IsMember(_ context.Context, _, _ string) (bool, error) {
	return m.isMember, m.memberErr
}
func (m *mockStore) ListMembers(_ context.Context, _ string) ([]string, error) {
	return m.members, m.membersErr
}
func (m *mockStore) ListMemberProfiles(_ context.Context, _ string) ([]chat.MemberProfile, error) {
	return m.memberProfiles, m.memberProfileErr
}
func (m *mockStore) AddGroupMember(_ context.Context, _, _, _ string) error {
	return m.groupMemberErr
}
func (m *mockStore) RemoveGroupMember(_ context.Context, _, _, _ string) error {
	return m.groupMemberErr
}
func (m *mockStore) UpdateGroupName(_ context.Context, _, _, _ string) error {
	return m.updateGroupErr
}
func (m *mockStore) ListRooms(_ context.Context, _ string) ([]chat.RoomSummary, error) {
	return m.rooms, m.roomsErr
}
func (m *mockStore) SaveMessage(_ context.Context, p chat.SaveMessageParams) (*chat.Message, error) {
	m.savedParams = &p
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

func (m *mockStore) ListRetentionEligibleMessages(_ context.Context) ([]string, error) {
	return m.retentionIDs, m.retentionErr
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

func TestService_ListRooms_LastMessageType(t *testing.T) {
	now := time.Now()
	want := []chat.RoomSummary{{
		ID: "r-1",
		LastMessage: &chat.MessageSummary{
			SenderID:  "u-2",
			Type:      chat.MessageTypeImage,
			Content:   "https://example.com/img.jpg",
			CreatedAt: now,
		},
	}}
	svc := chat.NewService(&mockStore{rooms: want})

	got, err := svc.ListRooms(t.Context(), "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0].LastMessage == nil {
		t.Fatal("LastMessage is nil")
	}
	if got[0].LastMessage.Type != chat.MessageTypeImage {
		t.Errorf("Type = %q, want %q", got[0].LastMessage.Type, chat.MessageTypeImage)
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

func TestService_GetRoom_Success(t *testing.T) {
	want := &chat.Room{ID: "r-1", Type: chat.RoomTypeGroup, Name: "squad"}
	svc := chat.NewService(&mockStore{room: want})

	got, err := svc.GetRoom(t.Context(), "r-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "r-1" {
		t.Errorf("ID = %q, want r-1", got.ID)
	}
}

func TestService_GetRoom_Error(t *testing.T) {
	svc := chat.NewService(&mockStore{roomErr: errors.New("db fail")})
	_, err := svc.GetRoom(t.Context(), "r-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestService_ListMemberProfiles_Success(t *testing.T) {
	want := []chat.MemberProfile{
		{UserID: "u-1", DisplayName: "Alice", IsAdmin: true},
		{UserID: "u-2", DisplayName: "Bob"},
	}
	svc := chat.NewService(&mockStore{memberProfiles: want})

	got, err := svc.ListMemberProfiles(t.Context(), "r-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
	if !got[0].IsAdmin {
		t.Error("first member should be admin")
	}
}

func TestService_ListMemberProfiles_Error(t *testing.T) {
	svc := chat.NewService(&mockStore{memberProfileErr: errors.New("db fail")})
	_, err := svc.ListMemberProfiles(t.Context(), "r-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestService_AddGroupMember_Success(t *testing.T) {
	svc := chat.NewService(&mockStore{})
	if err := svc.AddGroupMember(t.Context(), "r-1", "u-1", "u-2"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestService_AddGroupMember_Error(t *testing.T) {
	svc := chat.NewService(&mockStore{groupMemberErr: errors.New("db fail")})
	if err := svc.AddGroupMember(t.Context(), "r-1", "u-1", "u-2"); err == nil {
		t.Error("expected error, got nil")
	}
}

func TestService_RemoveGroupMember_Success(t *testing.T) {
	svc := chat.NewService(&mockStore{})
	if err := svc.RemoveGroupMember(t.Context(), "r-1", "u-1", "u-2"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestService_RemoveGroupMember_Error(t *testing.T) {
	svc := chat.NewService(&mockStore{groupMemberErr: errors.New("db fail")})
	if err := svc.RemoveGroupMember(t.Context(), "r-1", "u-1", "u-2"); err == nil {
		t.Error("expected error, got nil")
	}
}

func TestService_UpdateGroupName_Success(t *testing.T) {
	svc := chat.NewService(&mockStore{})
	if err := svc.UpdateGroupName(t.Context(), "r-1", "u-1", "new name"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestService_UpdateGroupName_Error(t *testing.T) {
	svc := chat.NewService(&mockStore{updateGroupErr: errors.New("db fail")})
	if err := svc.UpdateGroupName(t.Context(), "r-1", "u-1", "new name"); err == nil {
		t.Error("expected error, got nil")
	}
}

func TestService_CreateChannel_Success(t *testing.T) {
	want := &chat.Room{ID: "c-1", Type: chat.RoomTypeChannel, Name: "general", Description: "a place to chat"}
	svc := chat.NewService(&mockStore{room: want})

	got, err := svc.CreateChannel(t.Context(), "u-1", "general", "a place to chat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Type != chat.RoomTypeChannel {
		t.Errorf("type = %q, want channel", got.Type)
	}
	if got.Description != "a place to chat" {
		t.Errorf("description = %q, want 'a place to chat'", got.Description)
	}
}

func TestService_CreateChannel_Error(t *testing.T) {
	svc := chat.NewService(&mockStore{roomErr: errors.New("db fail")})
	_, err := svc.CreateChannel(t.Context(), "u-1", "general", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestService_ListChannels_Success(t *testing.T) {
	want := []chat.ChannelSummary{
		{ID: "c-1", Name: "general"},
		{ID: "c-2", Name: "random"},
	}
	svc := chat.NewService(&mockStore{channels: want})

	got, err := svc.ListChannels(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestService_ListChannels_Error(t *testing.T) {
	svc := chat.NewService(&mockStore{channelsErr: errors.New("db fail")})
	_, err := svc.ListChannels(t.Context())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestService_MaybeRedact_DMNonAccepted(t *testing.T) {
	store := &mockStore{
		room:    &chat.Room{ID: "r-1", Type: chat.RoomTypeDM},
		members: []string{"u-1", "u-2"},
		msg:     &chat.Message{ID: "m-1"},
	}
	svc := chat.NewService(store)
	svc.AreContacts = func(_ context.Context, _, _ string) (bool, error) { return false, nil }
	svc.IsExemptSender = func(_ context.Context, _ string) bool { return false }

	_, err := svc.SaveMessage(t.Context(), chat.SaveMessageParams{
		RoomID:   "r-1",
		SenderID: "u-1",
		Type:     chat.MessageTypeText,
		Content:  "call me at +1-555-0123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.savedParams == nil {
		t.Fatal("savedParams is nil")
	}
	if !store.savedParams.Redacted {
		t.Error("Redacted = false, want true")
	}
	if store.savedParams.Content == "call me at +1-555-0123" {
		t.Error("Content should have been redacted")
	}
}

func TestService_MaybeRedact_DMAccepted(t *testing.T) {
	store := &mockStore{
		room:    &chat.Room{ID: "r-1", Type: chat.RoomTypeDM},
		members: []string{"u-1", "u-2"},
		msg:     &chat.Message{ID: "m-1"},
	}
	svc := chat.NewService(store)
	svc.AreContacts = func(_ context.Context, _, _ string) (bool, error) { return true, nil }
	svc.IsExemptSender = func(_ context.Context, _ string) bool { return false }

	_, err := svc.SaveMessage(t.Context(), chat.SaveMessageParams{
		RoomID:   "r-1",
		SenderID: "u-1",
		Type:     chat.MessageTypeText,
		Content:  "call me at +1-555-0123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.savedParams.Redacted {
		t.Error("Redacted = true, want false (accepted contacts)")
	}
}

func TestService_MaybeRedact_ExemptSender(t *testing.T) {
	store := &mockStore{
		room:    &chat.Room{ID: "r-1", Type: chat.RoomTypeDM},
		members: []string{"u-1", "u-2"},
		msg:     &chat.Message{ID: "m-1"},
	}
	svc := chat.NewService(store)
	svc.AreContacts = func(_ context.Context, _, _ string) (bool, error) { return false, nil }
	svc.IsExemptSender = func(_ context.Context, _ string) bool { return true }

	_, err := svc.SaveMessage(t.Context(), chat.SaveMessageParams{
		RoomID:   "r-1",
		SenderID: "u-1",
		Type:     chat.MessageTypeText,
		Content:  "call me at +1-555-0123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.savedParams.Redacted {
		t.Error("Redacted = true, want false (exempt sender)")
	}
}

func TestService_MaybeRedact_GroupRoom(t *testing.T) {
	store := &mockStore{
		room:    &chat.Room{ID: "r-1", Type: chat.RoomTypeGroup},
		members: []string{"u-1", "u-2", "u-3"},
		msg:     &chat.Message{ID: "m-1"},
	}
	svc := chat.NewService(store)
	svc.AreContacts = func(_ context.Context, _, _ string) (bool, error) { return false, nil }
	svc.IsExemptSender = func(_ context.Context, _ string) bool { return false }

	_, err := svc.SaveMessage(t.Context(), chat.SaveMessageParams{
		RoomID:   "r-1",
		SenderID: "u-1",
		Type:     chat.MessageTypeText,
		Content:  "call me at +1-555-0123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.savedParams.Redacted {
		t.Error("Redacted = true, want false (group room)")
	}
}

func TestService_MaybeRedact_NilFuncs(t *testing.T) {
	store := &mockStore{
		room:    &chat.Room{ID: "r-1", Type: chat.RoomTypeDM},
		members: []string{"u-1", "u-2"},
		msg:     &chat.Message{ID: "m-1"},
	}
	svc := chat.NewService(store)

	_, err := svc.SaveMessage(t.Context(), chat.SaveMessageParams{
		RoomID:   "r-1",
		SenderID: "u-1",
		Type:     chat.MessageTypeText,
		Content:  "call me at +1-555-0123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.savedParams.Redacted {
		t.Error("Redacted = true, want false (nil funcs)")
	}
}

func TestService_MaybeRedact_NonTextType(t *testing.T) {
	store := &mockStore{
		room:    &chat.Room{ID: "r-1", Type: chat.RoomTypeDM},
		members: []string{"u-1", "u-2"},
		msg:     &chat.Message{ID: "m-1"},
	}
	svc := chat.NewService(store)
	svc.AreContacts = func(_ context.Context, _, _ string) (bool, error) { return false, nil }
	svc.IsExemptSender = func(_ context.Context, _ string) bool { return false }

	_, err := svc.SaveMessage(t.Context(), chat.SaveMessageParams{
		RoomID:   "r-1",
		SenderID: "u-1",
		Type:     chat.MessageTypeImage,
		Content:  "https://example.com/img.png",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.savedParams.Redacted {
		t.Error("Redacted = true, want false (non-text type)")
	}
}

func TestService_MaybeRedact_NoContactInfo(t *testing.T) {
	store := &mockStore{
		room:    &chat.Room{ID: "r-1", Type: chat.RoomTypeDM},
		members: []string{"u-1", "u-2"},
		msg:     &chat.Message{ID: "m-1"},
	}
	svc := chat.NewService(store)
	svc.AreContacts = func(_ context.Context, _, _ string) (bool, error) { return false, nil }
	svc.IsExemptSender = func(_ context.Context, _ string) bool { return false }

	_, err := svc.SaveMessage(t.Context(), chat.SaveMessageParams{
		RoomID:   "r-1",
		SenderID: "u-1",
		Type:     chat.MessageTypeText,
		Content:  "hey, how are you?",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.savedParams.Redacted {
		t.Error("Redacted = true, want false (no contact info)")
	}
}

func TestService_GetDMPeerID_Success(t *testing.T) {
	store := &mockStore{}
	svc := chat.NewService(store)
	peerID, err := svc.GetDMPeerID(t.Context(), "r-1", "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if peerID != "peer-1" {
		t.Errorf("peerID = %q, want peer-1", peerID)
	}
}

func TestService_ListRetentionEligibleMessages_Success(t *testing.T) {
	svc := chat.NewService(&mockStore{retentionIDs: []string{"m-1", "m-2"}})

	ids, err := svc.ListRetentionEligibleMessages(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("len = %d, want 2", len(ids))
	}
}

func TestService_ListRetentionEligibleMessages_Error(t *testing.T) {
	svc := chat.NewService(&mockStore{retentionErr: errors.New("db error")})
	_, err := svc.ListRetentionEligibleMessages(t.Context())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRetentionDurations_Defined(t *testing.T) {
	if _, ok := chat.RetentionDurations[chat.RoomTypeDM]; !ok {
		t.Error("RetentionDurations missing DM entry")
	}
	// Groups are relationship spaces like DMs — they share the retention window.
	if _, ok := chat.RetentionDurations[chat.RoomTypeGroup]; !ok {
		t.Error("RetentionDurations missing group entry")
	}
	// Channels must NOT have a retention window — they are broadcast-only and
	// never persist messages. The 24h public-room policy stays inert until
	// public rooms exist.
	if _, ok := chat.RetentionDurations[chat.RoomTypeChannel]; ok {
		t.Error("RetentionDurations must not include channels (broadcast-only, never persisted)")
	}
}
