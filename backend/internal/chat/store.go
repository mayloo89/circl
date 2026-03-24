package chat

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgStore struct {
	db        *pgxpool.Pool
	publicURL func(key string) string
}

// NewStore returns a Postgres-backed Store.
// publicURL converts a storage key to a publicly reachable URL; used to
// populate ThumbnailURL on messages returned by ListMessages.
func NewStore(db *pgxpool.Pool, publicURL func(key string) string) Store {
	return &pgStore{db: db, publicURL: publicURL}
}

// dmKey returns the canonical key for a DM room: the lexicographically smaller
// user ID comes first so that (A, B) and (B, A) produce the same key.
func dmKey(a, b string) string {
	if a < b {
		return a + ":" + b
	}
	return b + ":" + a
}

// GetOrCreateDM returns the existing DM room between userID and peerID, or
// creates one if it does not exist yet.  The operation is safe under concurrent
// requests: ON CONFLICT ensures only one row is ever inserted for each pair.
func (s *pgStore) GetOrCreateDM(ctx context.Context, userID, peerID string) (*Room, error) {
	key := dmKey(userID, peerID)

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("get or create dm: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var room Room
	err = tx.QueryRow(ctx, `
		SELECT id, type, COALESCE(name, ''), COALESCE(dm_key, ''), created_at, updated_at
		FROM rooms WHERE dm_key = $1`, key,
	).Scan(&room.ID, &room.Type, &room.Name, &room.DMKey, &room.CreatedAt, &room.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		// Room does not exist yet — create it.  DO UPDATE is a no-op that forces
		// RETURNING to always emit a row even when the conflict branch is taken.
		err = tx.QueryRow(ctx, `
			INSERT INTO rooms (type, dm_key) VALUES ('dm', $1)
			ON CONFLICT (dm_key) WHERE dm_key IS NOT NULL
			DO UPDATE SET dm_key = EXCLUDED.dm_key
			RETURNING id, type, COALESCE(name, ''), COALESCE(dm_key, ''), created_at, updated_at`, key,
		).Scan(&room.ID, &room.Type, &room.Name, &room.DMKey, &room.CreatedAt, &room.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("get or create dm: upsert room: %w", err)
		}

		if _, err = tx.Exec(ctx, `
			INSERT INTO room_members (room_id, user_id) VALUES ($1, $2), ($1, $3)
			ON CONFLICT DO NOTHING`, room.ID, userID, peerID,
		); err != nil {
			return nil, fmt.Errorf("get or create dm: add members: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("get or create dm: lookup: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("get or create dm: commit: %w", err)
	}
	return &room, nil
}

// CreateGroup creates a named group room and adds creatorID plus all memberIDs.
func (s *pgStore) CreateGroup(ctx context.Context, creatorID, name string, memberIDs []string) (*Room, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("create group: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var room Room
	if err = tx.QueryRow(ctx, `
		INSERT INTO rooms (type, name) VALUES ('group', $1)
		RETURNING id, type, COALESCE(name, ''), COALESCE(dm_key, ''), created_at, updated_at`, name,
	).Scan(&room.ID, &room.Type, &room.Name, &room.DMKey, &room.CreatedAt, &room.UpdatedAt); err != nil {
		return nil, fmt.Errorf("create group: insert room: %w", err)
	}

	// Deduplicate and ensure creator is always a member.
	seen := make(map[string]struct{}, len(memberIDs)+1)
	seen[creatorID] = struct{}{}
	members := []string{creatorID}
	for _, id := range memberIDs {
		if _, dup := seen[id]; !dup {
			seen[id] = struct{}{}
			members = append(members, id)
		}
	}

	for _, uid := range members {
		if _, err = tx.Exec(ctx,
			`INSERT INTO room_members (room_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			room.ID, uid,
		); err != nil {
			return nil, fmt.Errorf("create group: add member %s: %w", uid, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("create group: commit: %w", err)
	}
	return &room, nil
}

// ListMembers returns the user IDs of all members in a room.
func (s *pgStore) ListMembers(ctx context.Context, roomID string) ([]string, error) {
	rows, err := s.db.Query(ctx,
		`SELECT user_id::text FROM room_members WHERE room_id = $1`, roomID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("list members: scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// IsMember reports whether userID is a member of roomID.
func (s *pgStore) IsMember(ctx context.Context, roomID, userID string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM room_members WHERE room_id = $1 AND user_id = $2)`,
		roomID, userID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("is member: %w", err)
	}
	return exists, nil
}

// ListRooms returns all rooms the user belongs to, ordered by most-recently
// active first.  Each summary includes peer info (for DMs), the last message,
// and an unread count.
func (s *pgStore) ListRooms(ctx context.Context, userID string) ([]RoomSummary, error) {
	rows, err := s.db.Query(ctx, `
		SELECT
			r.id,
			r.type,
			COALESCE(r.name, '')                             AS name,
			r.created_at,
			COALESCE(peer.id::text, '')                                    AS peer_id,
			COALESCE(NULLIF(pp.display_name, ''), peer.email, '') AS peer_name,
			COALESCE(pp.avatar_url, '')                      AS peer_avatar_url,
			COALESCE(lm.content, '')                         AS last_content,
			COALESCE(lm.sender_id::text, '')                 AS last_sender_id,
			lm.created_at                                    AS last_at,
			(
				SELECT COUNT(*)::int
				FROM messages msg
				WHERE msg.room_id = r.id
				  AND msg.sender_id <> $1
				  AND (me.last_read_at IS NULL OR msg.created_at > me.last_read_at)
			) AS unread_count,
			peer_rm.last_read_at                             AS peer_last_read_at
		FROM rooms r
		JOIN room_members me
			ON me.room_id = r.id AND me.user_id = $1
		LEFT JOIN room_members peer_rm
			ON peer_rm.room_id = r.id AND peer_rm.user_id <> $1 AND r.type = 'dm'
		LEFT JOIN users peer
			ON peer.id = peer_rm.user_id
		LEFT JOIN profiles pp
			ON pp.user_id = peer.id
		LEFT JOIN LATERAL (
			SELECT content, sender_id, created_at
			FROM messages
			WHERE room_id = r.id
			ORDER BY created_at DESC
			LIMIT 1
		) lm ON true
		ORDER BY COALESCE(lm.created_at, r.created_at) DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list rooms: %w", err)
	}
	defer rows.Close()

	var summaries []RoomSummary
	for rows.Next() {
		var s RoomSummary
		var lastContent, lastSenderID string
		var lastAt *time.Time

		if err := rows.Scan(
			&s.ID, &s.Type, &s.Name, &s.CreatedAt,
			&s.PeerID, &s.PeerName, &s.PeerAvatarURL,
			&lastContent, &lastSenderID, &lastAt,
			&s.UnreadCount, &s.PeerLastReadAt,
		); err != nil {
			return nil, fmt.Errorf("list rooms: scan: %w", err)
		}

		if lastAt != nil {
			s.LastMessage = &MessageSummary{
				SenderID:  lastSenderID,
				Content:   lastContent,
				CreatedAt: *lastAt,
			}
		}
		summaries = append(summaries, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list rooms: rows: %w", err)
	}
	if summaries == nil {
		summaries = []RoomSummary{}
	}
	return summaries, nil
}

// SaveMessage persists a new message and returns the full record including
// sender display name for immediate broadcast.  If p.UploadID is set, the
// corresponding upload record is linked to this message in the same transaction.
func (s *pgStore) SaveMessage(ctx context.Context, p SaveMessageParams) (*Message, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("save message: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var msg Message
	err = tx.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO messages (room_id, sender_id, type, content, expires_at, view_once)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id, room_id, sender_id, type, content, expires_at, view_once, tombstone, created_at
		)
		SELECT
			i.id, i.room_id, i.sender_id,
			COALESCE(NULLIF(p.display_name, ''), u.email) AS sender_name,
			COALESCE(p.avatar_url, '') AS sender_avatar_url,
			i.type, i.content, i.expires_at, i.view_once, i.tombstone, i.created_at
		FROM inserted i
		JOIN users u ON u.id = i.sender_id
		LEFT JOIN profiles p ON p.user_id = i.sender_id`,
		p.RoomID, p.SenderID, p.Type, p.Content, p.ExpiresAt, p.ViewOnce,
	).Scan(
		&msg.ID, &msg.RoomID, &msg.SenderID, &msg.SenderName, &msg.SenderAvatarURL,
		&msg.Type, &msg.Content, &msg.ExpiresAt, &msg.ViewOnce, &msg.Tombstone, &msg.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("save message: %w", err)
	}

	if p.UploadID != "" {
		if _, err = tx.Exec(ctx, `
			UPDATE uploads SET message_id = $1
			WHERE id = $2 AND user_id = $3 AND status = 'committed'`,
			msg.ID, p.UploadID, p.SenderID,
		); err != nil {
			return nil, fmt.Errorf("save message: link upload: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("save message: commit: %w", err)
	}
	return &msg, nil
}

// ListMessages returns up to limit messages in roomID older than before
// (or all messages when before is nil), newest first.
// Expired TTL messages are excluded.
func (s *pgStore) ListMessages(ctx context.Context, roomID string, before *time.Time, limit int) ([]Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows, err := s.db.Query(ctx, `
		SELECT
			m.id, m.room_id, m.sender_id,
			COALESCE(NULLIF(p.display_name, ''), u.email) AS sender_name,
			COALESCE(p.avatar_url, '') AS sender_avatar_url,
			m.type, m.content, m.expires_at, m.view_once, m.tombstone, m.created_at,
			COALESCE(up.thumbnail_key, '') AS thumbnail_key
		FROM messages m
		JOIN users u ON u.id = m.sender_id
		LEFT JOIN profiles p ON p.user_id = m.sender_id
		LEFT JOIN uploads up ON up.message_id = m.id
		WHERE m.room_id = $1
		  AND ($2::timestamptz IS NULL OR m.created_at < $2)
		  AND (m.expires_at IS NULL OR m.expires_at > NOW() OR m.tombstone)
		ORDER BY m.created_at DESC
		LIMIT $3`,
		roomID, before, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		var thumbnailKey string
		if err := rows.Scan(
			&m.ID, &m.RoomID, &m.SenderID, &m.SenderName, &m.SenderAvatarURL,
			&m.Type, &m.Content, &m.ExpiresAt, &m.ViewOnce, &m.Tombstone, &m.CreatedAt,
			&thumbnailKey,
		); err != nil {
			return nil, fmt.Errorf("list messages: scan: %w", err)
		}
		if thumbnailKey != "" && s.publicURL != nil {
			m.ThumbnailURL = s.publicURL(thumbnailKey)
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list messages: rows: %w", err)
	}
	if msgs == nil {
		msgs = []Message{}
	}
	return msgs, nil
}

// MarkRead updates the user's last_read_at for the given room to now and
// returns the timestamp that was written so the caller can broadcast it.
func (s *pgStore) MarkRead(ctx context.Context, roomID, userID string) (time.Time, error) {
	var readAt time.Time
	err := s.db.QueryRow(ctx,
		`UPDATE room_members SET last_read_at = NOW()
		 WHERE room_id = $1 AND user_id = $2
		 RETURNING last_read_at`,
		roomID, userID,
	).Scan(&readAt)
	if err != nil {
		return time.Time{}, fmt.Errorf("mark read: %w", err)
	}
	return readAt, nil
}

// ViewOnceMessage atomically records that viewerID has viewed the message.
// For DMs (the only supported scope), once all non-sender members have viewed
// the message, it is deleted and the storage keys for any linked uploads are
// returned so the caller can remove them from object storage.
func (s *pgStore) ViewOnceMessage(ctx context.Context, messageID, roomID, viewerID string) (*Message, []string, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("view once: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var msg Message
	err = tx.QueryRow(ctx, `
		SELECT m.id, m.room_id, m.sender_id,
			COALESCE(NULLIF(p.display_name, ''), u.email),
			COALESCE(p.avatar_url, ''),
			m.type, m.content, m.expires_at, m.view_once, m.created_at
		FROM messages m
		JOIN users u ON u.id = m.sender_id
		LEFT JOIN profiles p ON p.user_id = m.sender_id
		WHERE m.id = $1 AND m.room_id = $2`,
		messageID, roomID,
	).Scan(
		&msg.ID, &msg.RoomID, &msg.SenderID, &msg.SenderName, &msg.SenderAvatarURL,
		&msg.Type, &msg.Content, &msg.ExpiresAt, &msg.ViewOnce, &msg.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("view once: get message: %w", err)
	}
	if !msg.ViewOnce {
		return nil, nil, ErrForbidden
	}
	if msg.SenderID == viewerID {
		return nil, nil, ErrForbidden
	}

	if _, err = tx.Exec(ctx, `
		INSERT INTO message_views (message_id, user_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, messageID, viewerID,
	); err != nil {
		return nil, nil, fmt.Errorf("view once: record view: %w", err)
	}

	// Check whether all non-sender members have now viewed the message.
	var shouldDelete bool
	if err = tx.QueryRow(ctx, `
		SELECT NOT EXISTS (
			SELECT 1 FROM room_members rm
			WHERE rm.room_id = $1
			  AND rm.user_id != $2
			  AND NOT EXISTS (
				SELECT 1 FROM message_views mv
				WHERE mv.message_id = $3 AND mv.user_id = rm.user_id
			  )
		)`, msg.RoomID, msg.SenderID, messageID,
	).Scan(&shouldDelete); err != nil {
		return nil, nil, fmt.Errorf("view once: check views: %w", err)
	}

	var keys []string
	if shouldDelete {
		keys, err = collectUploadKeys(ctx, tx, messageID)
		if err != nil {
			return nil, nil, err
		}
		if _, err = tx.Exec(ctx,
			`UPDATE messages SET content = '', tombstone = true WHERE id = $1`,
			messageID,
		); err != nil {
			return nil, nil, fmt.Errorf("view once: tombstone: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("view once: commit: %w", err)
	}
	return &msg, keys, nil
}

// DeleteMessage deletes a message and its linked uploads from the database.
// Returns the room ID and any storage keys to remove from object storage.
func (s *pgStore) DeleteMessage(ctx context.Context, messageID string) (string, []string, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("delete message: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var roomID string
	if err = tx.QueryRow(ctx,
		`SELECT room_id FROM messages WHERE id = $1`, messageID,
	).Scan(&roomID); errors.Is(err, pgx.ErrNoRows) {
		return "", nil, ErrNotFound
	} else if err != nil {
		return "", nil, fmt.Errorf("delete message: get room: %w", err)
	}

	keys, err := collectUploadKeys(ctx, tx, messageID)
	if err != nil {
		return "", nil, err
	}

	if _, err = tx.Exec(ctx, `DELETE FROM messages WHERE id = $1`, messageID); err != nil {
		return "", nil, fmt.Errorf("delete message: delete: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", nil, fmt.Errorf("delete message: commit: %w", err)
	}
	return roomID, keys, nil
}

// TombstoneMessage converts an expired TTL message into a persistent tombstone:
// it erases the content, sets tombstone=true and expires_at=NULL so the record
// is retained in history but no longer appears as active.
// Returns the room ID and any storage keys to remove from object storage.
func (s *pgStore) TombstoneMessage(ctx context.Context, messageID string) (string, []string, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("tombstone message: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var roomID string
	if err = tx.QueryRow(ctx,
		`SELECT room_id FROM messages WHERE id = $1`, messageID,
	).Scan(&roomID); errors.Is(err, pgx.ErrNoRows) {
		return "", nil, ErrNotFound
	} else if err != nil {
		return "", nil, fmt.Errorf("tombstone message: get room: %w", err)
	}

	keys, err := collectUploadKeys(ctx, tx, messageID)
	if err != nil {
		return "", nil, err
	}

	if _, err = tx.Exec(ctx,
		`UPDATE messages SET content = '', expires_at = NULL, tombstone = true WHERE id = $1`,
		messageID,
	); err != nil {
		return "", nil, fmt.Errorf("tombstone message: update: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", nil, fmt.Errorf("tombstone message: commit: %w", err)
	}
	return roomID, keys, nil
}

// GetDisplayName returns the display_name for the given user from their profile.
// Returns an empty string when no profile row exists.
func (s *pgStore) GetDisplayName(ctx context.Context, userID string) (string, error) {
	var name string
	err := s.db.QueryRow(ctx,
		`SELECT COALESCE(display_name, '') FROM profiles WHERE user_id = $1`,
		userID,
	).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get display name: %w", err)
	}
	return name, nil
}

// ListExpiredMessages returns the IDs of messages whose TTL has elapsed.
func (s *pgStore) ListExpiredMessages(ctx context.Context) ([]string, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id FROM messages WHERE expires_at IS NOT NULL AND expires_at < NOW()`)
	if err != nil {
		return nil, fmt.Errorf("list expired messages: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("list expired messages: scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// txQuerier is the subset of pgx.Tx used by collectUploadKeys.
type txQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// collectUploadKeys queries uploads linked to messageID and returns all storage
// keys (primary + thumbnail) that should be removed from object storage.
func collectUploadKeys(ctx context.Context, tx txQuerier, messageID string) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT storage_key, thumbnail_key
		FROM uploads
		WHERE message_id = $1 AND status = 'committed'`, messageID)
	if err != nil {
		return nil, fmt.Errorf("collect upload keys: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var sk string
		var tk *string
		if err := rows.Scan(&sk, &tk); err != nil {
			return nil, fmt.Errorf("collect upload keys: scan: %w", err)
		}
		keys = append(keys, sk)
		if tk != nil {
			keys = append(keys, *tk)
		}
	}
	return keys, rows.Err()
}
