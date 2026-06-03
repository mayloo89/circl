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

// TestIntegration_RetentionEligibility exercises ListRetentionEligibleMessages
// against a real DB. It guards the core retention rules: DM and group messages
// age out after ~3 months, while channel rows are never eligible (channels are
// broadcast-only and not in the retention map) and self-destruct / tombstoned
// rows are left to the EphemeralCleaner.
func TestIntegration_RetentionEligibility(t *testing.T) {
	pool := openTestDB(t)
	store := NewStore(pool, func(key string) string { return "https://example.com/" + key })
	ctx := t.Context()

	u1 := createTestUser(t, pool, "ret_test_u1@example.com")
	u2 := createTestUser(t, pool, "ret_test_u2@example.com")

	dm, err := store.GetOrCreateDM(ctx, u1, u2)
	if err != nil {
		t.Fatalf("GetOrCreateDM: %v", err)
	}
	channel, err := store.CreateChannel(ctx, u1, "retention-test-channel", "")
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	group, err := store.CreateGroup(ctx, u1, "retention-test-group", []string{u2})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	// Clean up the channel/group and all messages we create (runs before the
	// user cleanup registered by createTestUser, since t.Cleanup is LIFO).
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM rooms WHERE id = ANY($1)`, []string{channel.ID, group.ID}) //nolint:errcheck
	})

	save := func(roomID, content string, expiresAt *time.Time) string {
		t.Helper()
		m, err := store.SaveMessage(ctx, SaveMessageParams{
			RoomID: roomID, SenderID: u1, Type: MessageTypeText, Content: content, ExpiresAt: expiresAt,
		})
		if err != nil {
			t.Fatalf("SaveMessage(%q): %v", content, err)
		}
		return m.ID
	}
	backdate := func(id string, age time.Duration) {
		t.Helper()
		if _, err := pool.Exec(ctx,
			`UPDATE messages SET created_at = NOW() - make_interval(secs => $2) WHERE id = $1`,
			id, age.Seconds(),
		); err != nil {
			t.Fatalf("backdate %q: %v", id, err)
		}
	}

	future := time.Now().Add(time.Hour)
	oldDM := save(dm.ID, "old dm message", nil)
	recentDM := save(dm.ID, "recent dm message", nil)
	oldGroup := save(group.ID, "old group message", nil)
	recentGroup := save(group.ID, "recent group message", nil)
	oldChannel := save(channel.ID, "old channel message", nil)
	oldSelfDestruct := save(dm.ID, "old self-destruct dm", &future)
	oldTombstoned := save(dm.ID, "old tombstoned dm", nil)

	// Age the relevant messages past the retention window (~3 months).
	old := 100 * 24 * time.Hour
	backdate(oldDM, old)
	backdate(oldGroup, old)
	backdate(oldChannel, old)
	backdate(oldSelfDestruct, old)
	backdate(oldTombstoned, old)
	if _, err := pool.Exec(ctx, `UPDATE messages SET tombstone = true WHERE id = $1`, oldTombstoned); err != nil {
		t.Fatalf("tombstone: %v", err)
	}

	ids, err := store.ListRetentionEligibleMessages(ctx)
	if err != nil {
		t.Fatalf("ListRetentionEligibleMessages: %v", err)
	}
	got := make(map[string]bool, len(ids))
	for _, id := range ids {
		got[id] = true
	}

	if !got[oldDM] {
		t.Error("old DM message should be retention-eligible")
	}
	if got[recentDM] {
		t.Error("recent DM message must NOT be retention-eligible")
	}
	if !got[oldGroup] {
		t.Error("old group message should be retention-eligible")
	}
	if got[recentGroup] {
		t.Error("recent group message must NOT be retention-eligible")
	}
	if got[oldChannel] {
		t.Error("channel message must NEVER be retention-eligible (channels are broadcast-only, not in the retention map)")
	}
	if got[oldSelfDestruct] {
		t.Error("self-destruct (expires_at) message must be left to the EphemeralCleaner, not retention")
	}
	if got[oldTombstoned] {
		t.Error("tombstoned message must NOT be retention-eligible")
	}
}

func TestIntegration_PublicRooms(t *testing.T) {
	pool := openTestDB(t)
	store := NewStore(pool, func(key string) string { return "https://example.com/" + key })
	ctx := t.Context()

	admin := createTestUser(t, pool, "pubroom_admin@example.com")

	// --- CreatePublicRoom ---

	room, err := store.CreatePublicRoom(ctx, admin, "Test Lounge", "A friendly place")
	if err != nil {
		t.Fatalf("CreatePublicRoom: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM rooms WHERE id = $1`, room.ID) //nolint:errcheck
	})
	if room.Type != RoomTypePublic {
		t.Errorf("room.Type = %q, want %q", room.Type, RoomTypePublic)
	}
	if room.Name != "Test Lounge" {
		t.Errorf("room.Name = %q, want %q", room.Name, "Test Lounge")
	}
	if room.Visibility != RoomVisibilityPublic {
		t.Errorf("room.Visibility = %q, want %q", room.Visibility, RoomVisibilityPublic)
	}

	// --- ListPublicRooms ---

	rooms, err := store.ListPublicRooms(ctx)
	if err != nil {
		t.Fatalf("ListPublicRooms: %v", err)
	}
	found := slices.ContainsFunc(rooms, func(r PublicRoomSummary) bool {
		return r.ID == room.ID
	})
	if !found {
		t.Errorf("ListPublicRooms: created room %s not found in listing", room.ID)
	}

	// --- Guest can save a message in the public room ---

	msg, err := store.SaveMessage(ctx, SaveMessageParams{
		RoomID:        room.ID,
		SenderID:      "guest:abc-123",
		Type:          MessageTypeText,
		Content:       "hello from guest",
		SenderIsGuest: true,
		SenderName:    "GuestUser1",
	})
	if err != nil {
		t.Fatalf("SaveMessage (guest): %v", err)
	}
	if msg.SenderName != "GuestUser1" {
		t.Errorf("msg.SenderName = %q, want %q", msg.SenderName, "GuestUser1")
	}
	if msg.SenderAvatarURL != "" {
		t.Errorf("msg.SenderAvatarURL = %q, want empty", msg.SenderAvatarURL)
	}

	// --- IsMember returns true for public rooms ---

	isMember, err := store.IsMember(ctx, room.ID, "any-random-user")
	if err != nil {
		t.Fatalf("IsMember (public room): %v", err)
	}
	if !isMember {
		t.Error("IsMember should return true for public rooms regardless of user")
	}

	// --- NicknameTaken: a registered user's username should be protected ---

	createTestProfile(t, pool, admin, "adminnick", "AdminNick")
	taken, err := store.NicknameTaken(ctx, "AdminNick")
	if err != nil {
		t.Fatalf("NicknameTaken: %v", err)
	}
	if !taken {
		t.Error("NicknameTaken should return true for a registered display_name")
	}
	taken, err = store.NicknameTaken(ctx, "adminnick")
	if err != nil {
		t.Fatalf("NicknameTaken (username): %v", err)
	}
	if !taken {
		t.Error("NicknameTaken should return true for a registered username (case-insensitive)")
	}
	taken, err = store.NicknameTaken(ctx, "TotallyUnknown123")
	if err != nil {
		t.Fatalf("NicknameTaken (unknown): %v", err)
	}
	if taken {
		t.Error("NicknameTaken should return false for an unknown nickname")
	}

	// --- 24h retention: public room messages are eligible ---

	save := func(roomID, content string, expiresAt *time.Time) string {
		t.Helper()
		m, err := store.SaveMessage(ctx, SaveMessageParams{
			RoomID: roomID, SenderID: admin, Type: MessageTypeText, Content: content, ExpiresAt: expiresAt,
		})
		if err != nil {
			t.Fatalf("SaveMessage(%q): %v", content, err)
		}
		return m.ID
	}
	backdate := func(id string, age time.Duration) {
		t.Helper()
		if _, err := pool.Exec(ctx,
			`UPDATE messages SET created_at = NOW() - make_interval(secs => $2) WHERE id = $1`,
			id, age.Seconds(),
		); err != nil {
			t.Fatalf("backdate %q: %v", id, err)
		}
	}

	oldMsgID := save(room.ID, "old public msg", nil)
	backdate(oldMsgID, 25*time.Hour)

	retentionIDs, err := store.ListRetentionEligibleMessages(ctx)
	if err != nil {
		t.Fatalf("ListRetentionEligibleMessages: %v", err)
	}
	retentionSet := make(map[string]bool, len(retentionIDs))
	for _, id := range retentionIDs {
		retentionSet[id] = true
	}
	if !retentionSet[oldMsgID] {
		t.Error("public room message older than 24h should be retention-eligible")
	}
}

func createTestProfile(t *testing.T, pool *pgxpool.Pool, userID, username, displayName string) {
	t.Helper()
	_, err := pool.Exec(t.Context(),
		`UPDATE profiles SET username = $2, display_name = $3 WHERE user_id = $1`,
		userID, username, displayName)
	if err != nil {
		t.Fatalf("createTestProfile: %v", err)
	}
}
