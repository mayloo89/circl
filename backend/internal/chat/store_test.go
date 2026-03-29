package chat

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// openTestDB returns a live pool for integration tests.
// The test is skipped when TEST_DATABASE_URL is not set.
func openTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// createTestUser inserts a minimal user record and returns its UUID.
func createTestUser(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(t.Context(),
		`INSERT INTO users (email, password_hash, status) VALUES ($1, 'x', 'active')
		 ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email RETURNING id`, email,
	).Scan(&id); err != nil {
		t.Fatalf("createTestUser(%q): %v", email, err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) //nolint:errcheck
	})
	return id
}

func TestIntegration_ChatFlow(t *testing.T) {
	pool := openTestDB(t)
	store := NewStore(pool, func(key string) string { return "https://example.com/" + key })
	ctx := t.Context()

	u1 := createTestUser(t, pool, "chat_test_u1@example.com")
	u2 := createTestUser(t, pool, "chat_test_u2@example.com")

	// --- GetOrCreateDM ---

	room, err := store.GetOrCreateDM(ctx, u1, u2)
	if err != nil {
		t.Fatalf("GetOrCreateDM: %v", err)
	}
	if room.Type != RoomTypeDM {
		t.Errorf("room.Type = %q, want %q", room.Type, RoomTypeDM)
	}

	// Calling again must return the same room.
	room2, err := store.GetOrCreateDM(ctx, u2, u1)
	if err != nil {
		t.Fatalf("GetOrCreateDM (second call): %v", err)
	}
	if room2.ID != room.ID {
		t.Errorf("second call returned room %q, want %q", room2.ID, room.ID)
	}

	// --- IsMember ---

	ok, err := store.IsMember(ctx, room.ID, u1)
	if err != nil {
		t.Fatalf("IsMember: %v", err)
	}
	if !ok {
		t.Error("u1 should be a member of the DM room")
	}

	ok, err = store.IsMember(ctx, room.ID, "00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("IsMember non-member: %v", err)
	}
	if ok {
		t.Error("non-existent user should not be a member")
	}

	// --- SaveMessage ---

	msg, err := store.SaveMessage(ctx, SaveMessageParams{
		RoomID: room.ID, SenderID: u1, Type: MessageTypeText, Content: "hello world",
	})
	if err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}
	if msg.Content != "hello world" {
		t.Errorf("Content = %q, want %q", msg.Content, "hello world")
	}
	if msg.SenderID != u1 {
		t.Errorf("SenderID = %q, want %q", msg.SenderID, u1)
	}
	if msg.Type != MessageTypeText {
		t.Errorf("Type = %q, want %q", msg.Type, MessageTypeText)
	}

	// --- ListMessages ---

	msgs, err := store.ListMessages(ctx, room.ID, nil, 50)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(msgs) == 0 {
		t.Fatal("expected at least 1 message, got 0")
	}
	if msgs[0].ID != msg.ID {
		t.Errorf("first message ID = %q, want %q", msgs[0].ID, msg.ID)
	}

	// Pagination: request messages before the saved one — should be empty.
	before := msg.CreatedAt.Add(-time.Second)
	older, err := store.ListMessages(ctx, room.ID, &before, 50)
	if err != nil {
		t.Fatalf("ListMessages before: %v", err)
	}
	if len(older) != 0 {
		t.Errorf("expected 0 messages before first, got %d", len(older))
	}

	// --- ListRooms ---

	rooms, err := store.ListRooms(ctx, u1)
	if err != nil {
		t.Fatalf("ListRooms: %v", err)
	}
	found := false
	for _, r := range rooms {
		if r.ID == room.ID {
			found = true
			if r.LastMessage == nil {
				t.Error("LastMessage should not be nil after sending a message")
			}
		}
	}
	if !found {
		t.Errorf("room %q not found in ListRooms for u1", room.ID)
	}

	// --- ListMembers ---

	memberIDs, err := store.ListMembers(ctx, room.ID)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	found1, found2 := false, false
	for _, id := range memberIDs {
		if id == u1 {
			found1 = true
		}
		if id == u2 {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Errorf("ListMembers = %v, want both %q and %q", memberIDs, u1, u2)
	}

	// --- MarkRead ---

	readAt, err := store.MarkRead(ctx, room.ID, u1)
	if err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	if readAt.IsZero() {
		t.Error("MarkRead returned zero time")
	}

	// After marking read, unread_count for u1 should drop to 0.
	rooms, err = store.ListRooms(ctx, u1)
	if err != nil {
		t.Fatalf("ListRooms after MarkRead: %v", err)
	}
	for _, r := range rooms {
		if r.ID == room.ID && r.UnreadCount != 0 {
			t.Errorf("UnreadCount = %d, want 0 after MarkRead", r.UnreadCount)
		}
	}
}

func TestIntegration_GetDisplayName(t *testing.T) {
	pool := openTestDB(t)
	store := NewStore(pool, func(key string) string { return "https://example.com/" + key })
	ctx := t.Context()

	uid := createTestUser(t, pool, "chat_dn_test@example.com")

	// No profile row yet — should return empty string, no error.
	name, err := store.GetDisplayName(ctx, uid)
	if err != nil {
		t.Fatalf("GetDisplayName (no profile): %v", err)
	}
	if name != "" {
		t.Errorf("name = %q, want empty", name)
	}

	// Insert a profile row.
	if _, err := pool.Exec(ctx,
		`INSERT INTO profiles (user_id, display_name, bio) VALUES ($1, 'Test User', '')
		 ON CONFLICT (user_id) DO UPDATE SET display_name = EXCLUDED.display_name`,
		uid,
	); err != nil {
		t.Fatalf("insert profile: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM profiles WHERE user_id = $1`, uid) //nolint:errcheck
	})

	name, err = store.GetDisplayName(ctx, uid)
	if err != nil {
		t.Fatalf("GetDisplayName (with profile): %v", err)
	}
	if name != "Test User" {
		t.Errorf("name = %q, want %q", name, "Test User")
	}
}

func TestIntegration_CreateGroup(t *testing.T) {
	pool := openTestDB(t)
	store := NewStore(pool, func(key string) string { return "https://example.com/" + key })
	ctx := t.Context()

	u1 := createTestUser(t, pool, "chat_group_u1@example.com")
	u2 := createTestUser(t, pool, "chat_group_u2@example.com")
	u3 := createTestUser(t, pool, "chat_group_u3@example.com")

	room, err := store.CreateGroup(ctx, u1, "Test Group", []string{u2, u3})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if room.Type != RoomTypeGroup {
		t.Errorf("Type = %q, want %q", room.Type, RoomTypeGroup)
	}
	if room.Name != "Test Group" {
		t.Errorf("Name = %q, want %q", room.Name, "Test Group")
	}

	// All three should be members.
	for _, uid := range []string{u1, u2, u3} {
		ok, err := store.IsMember(ctx, room.ID, uid)
		if err != nil {
			t.Fatalf("IsMember(%q): %v", uid, err)
		}
		if !ok {
			t.Errorf("user %q should be a member of the group", uid)
		}
	}
}

func TestIntegration_ViewOnceMessage(t *testing.T) {
	pool := openTestDB(t)
	store := NewStore(pool, func(key string) string { return "https://example.com/" + key })
	ctx := t.Context()

	u1 := createTestUser(t, pool, "chat_vo_u1@example.com")
	u2 := createTestUser(t, pool, "chat_vo_u2@example.com")

	room, err := store.GetOrCreateDM(ctx, u1, u2)
	if err != nil {
		t.Fatalf("GetOrCreateDM: %v", err)
	}

	msg, err := store.SaveMessage(ctx, SaveMessageParams{
		RoomID: room.ID, SenderID: u1, Type: MessageTypeText,
		Content: "view once secret", ViewOnce: true,
	})
	if err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}

	// Sender viewing their own view-once message must be rejected.
	_, _, err = store.ViewOnceMessage(ctx, msg.ID, room.ID, u1)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("sender view: err = %v, want ErrForbidden", err)
	}

	// Trying to view a non-view-once message returns ErrForbidden.
	plain, err := store.SaveMessage(ctx, SaveMessageParams{
		RoomID: room.ID, SenderID: u1, Type: MessageTypeText, Content: "normal",
	})
	if err != nil {
		t.Fatalf("SaveMessage plain: %v", err)
	}
	_, _, err = store.ViewOnceMessage(ctx, plain.ID, room.ID, u2)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("non-view-once: err = %v, want ErrForbidden", err)
	}

	// Viewing an unknown message returns ErrNotFound.
	_, _, err = store.ViewOnceMessage(ctx, "00000000-0000-0000-0000-000000000000", room.ID, u2)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown message: err = %v, want ErrNotFound", err)
	}

	// Legitimate view by u2 — only non-sender member, so the message is tombstoned.
	viewed, keys, err := store.ViewOnceMessage(ctx, msg.ID, room.ID, u2)
	if err != nil {
		t.Fatalf("ViewOnceMessage: %v", err)
	}
	if viewed.ID != msg.ID {
		t.Errorf("ID = %q, want %q", viewed.ID, msg.ID)
	}
	if len(keys) != 0 {
		t.Errorf("keys = %v, want empty (no uploads linked)", keys)
	}
}

func TestIntegration_DeleteMessage(t *testing.T) {
	pool := openTestDB(t)
	store := NewStore(pool, func(key string) string { return "https://example.com/" + key })
	ctx := t.Context()

	u1 := createTestUser(t, pool, "chat_del_u1@example.com")
	u2 := createTestUser(t, pool, "chat_del_u2@example.com")

	room, err := store.GetOrCreateDM(ctx, u1, u2)
	if err != nil {
		t.Fatalf("GetOrCreateDM: %v", err)
	}

	msg, err := store.SaveMessage(ctx, SaveMessageParams{
		RoomID: room.ID, SenderID: u1, Type: MessageTypeText, Content: "to be deleted",
	})
	if err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}

	roomID, keys, err := store.DeleteMessage(ctx, msg.ID)
	if err != nil {
		t.Fatalf("DeleteMessage: %v", err)
	}
	if roomID != room.ID {
		t.Errorf("roomID = %q, want %q", roomID, room.ID)
	}
	if len(keys) != 0 {
		t.Errorf("keys = %v, want empty", keys)
	}

	// Second delete must return ErrNotFound.
	_, _, err = store.DeleteMessage(ctx, msg.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: err = %v, want ErrNotFound", err)
	}
}

func TestIntegration_TombstoneAndExpiry(t *testing.T) {
	pool := openTestDB(t)
	store := NewStore(pool, func(key string) string { return "https://example.com/" + key })
	ctx := t.Context()

	u1 := createTestUser(t, pool, "chat_ttl_u1@example.com")
	u2 := createTestUser(t, pool, "chat_ttl_u2@example.com")

	room, err := store.GetOrCreateDM(ctx, u1, u2)
	if err != nil {
		t.Fatalf("GetOrCreateDM: %v", err)
	}

	past := time.Now().Add(-time.Minute)
	msg, err := store.SaveMessage(ctx, SaveMessageParams{
		RoomID: room.ID, SenderID: u1, Type: MessageTypeText,
		Content: "ephemeral", ExpiresAt: &past,
	})
	if err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM messages WHERE id = $1`, msg.ID) //nolint:errcheck
	})

	// Message should appear in ListExpiredMessages.
	ids, err := store.ListExpiredMessages(ctx)
	if err != nil {
		t.Fatalf("ListExpiredMessages: %v", err)
	}
	if !slices.Contains(ids, msg.ID) {
		t.Errorf("expired message %q not in ListExpiredMessages", msg.ID)
	}

	// Tombstone the message.
	roomID, keys, err := store.TombstoneMessage(ctx, msg.ID)
	if err != nil {
		t.Fatalf("TombstoneMessage: %v", err)
	}
	if roomID != room.ID {
		t.Errorf("roomID = %q, want %q", roomID, room.ID)
	}
	if len(keys) != 0 {
		t.Errorf("keys = %v, want empty", keys)
	}

	// Tombstoned message should appear in ListMessages with tombstone=true.
	msgs, err := store.ListMessages(ctx, room.ID, nil, 50)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	tombstoneFound := false
	for _, m := range msgs {
		if m.ID == msg.ID {
			tombstoneFound = true
			if !m.Tombstone {
				t.Error("expected tombstone=true")
			}
			if m.Content != "" {
				t.Errorf("tombstoned content = %q, want empty", m.Content)
			}
		}
	}
	if !tombstoneFound {
		t.Errorf("tombstoned message %q not found in ListMessages", msg.ID)
	}

	// TombstoneMessage on unknown ID returns ErrNotFound.
	_, _, err = store.TombstoneMessage(ctx, "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown tombstone: err = %v, want ErrNotFound", err)
	}
}

// TestListMessages_DefaultLimit verifies the limit is capped when <= 0.
func TestIntegration_ListMessages_DefaultLimit(t *testing.T) {
	pool := openTestDB(t)
	store := NewStore(pool, func(key string) string { return "https://example.com/" + key })
	ctx := t.Context()

	u1 := createTestUser(t, pool, "chat_limit_u1@example.com")
	u2 := createTestUser(t, pool, "chat_limit_u2@example.com")

	room, err := store.GetOrCreateDM(ctx, u1, u2)
	if err != nil {
		t.Fatalf("GetOrCreateDM: %v", err)
	}

	// Limit 0 should not error (store defaults to 50).
	msgs, err := store.ListMessages(ctx, room.ID, nil, 0)
	if err != nil {
		t.Fatalf("ListMessages(limit=0): %v", err)
	}
	_ = msgs // result can be empty, just verifying no error
}
