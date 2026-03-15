package presence

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const (
	// onlineTTL is how long a presence key lives in Redis without a heartbeat.
	onlineTTL = 35 * time.Second

	// keyPrefix is the Redis key namespace for online presence.
	keyPrefix = "presence:"
)

// Info holds the presence state for a single user.
type Info struct {
	UserID     string     `json:"user_id"`
	Online     bool       `json:"online"`
	LastSeenAt *time.Time `json:"last_seen_at"`
}

// Store handles Redis and Postgres operations for presence.
type Store struct {
	rdb *redis.Client
	db  *pgxpool.Pool
}

// NewStore returns a ready-to-use Store.
func NewStore(rdb *redis.Client, db *pgxpool.Pool) *Store {
	return &Store{rdb: rdb, db: db}
}

// Heartbeat marks userID as online and updates last_seen_at in the DB.
// It returns true if the user just came online (key did not exist before).
func (s *Store) Heartbeat(ctx context.Context, userID string) (justOnline bool, err error) {
	key := keyPrefix + userID

	// SET key 1 EX 35 NX — only sets if not exists.
	result, err := s.rdb.SetArgs(ctx, key, 1, redis.SetArgs{TTL: onlineTTL, Mode: "NX"}).Result()
	if err != nil && err != redis.Nil {
		return false, fmt.Errorf("presence heartbeat: redis set nx: %w", err)
	}

	if result == "OK" {
		// Key was new — user just came online.
		justOnline = true
	} else {
		// Key already existed — renew the TTL.
		if err := s.rdb.Expire(ctx, key, onlineTTL).Err(); err != nil {
			return false, fmt.Errorf("presence heartbeat: redis expire: %w", err)
		}
	}

	// Update last_seen_at in the DB on every heartbeat.
	if _, err := s.db.Exec(ctx,
		`UPDATE users SET last_seen_at = NOW() WHERE id = $1`, userID,
	); err != nil {
		return false, fmt.Errorf("presence heartbeat: update last_seen_at: %w", err)
	}

	return justOnline, nil
}

// Offline immediately removes the presence key for userID, marking them offline.
func (s *Store) Offline(ctx context.Context, userID string) error {
	if err := s.rdb.Del(ctx, keyPrefix+userID).Err(); err != nil {
		return fmt.Errorf("presence offline: redis del: %w", err)
	}
	return nil
}

// GetPresence returns presence info for the given user IDs.
// Online status is read from Redis; last_seen_at from Postgres.
func (s *Store) GetPresence(ctx context.Context, userIDs []string) ([]Info, error) {
	if len(userIDs) == 0 {
		return []Info{}, nil
	}

	// Pipeline EXISTS checks for all users.
	pipe := s.rdb.Pipeline()
	cmds := make([]*redis.IntCmd, len(userIDs))
	for i, id := range userIDs {
		cmds[i] = pipe.Exists(ctx, keyPrefix+id)
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("get presence: redis pipeline: %w", err)
	}

	online := make(map[string]bool, len(userIDs))
	for i, cmd := range cmds {
		online[userIDs[i]] = cmd.Val() > 0
	}

	// Fetch last_seen_at from Postgres for all users in one query.
	rows, err := s.db.Query(ctx,
		`SELECT id::text, last_seen_at FROM users WHERE id = ANY($1::uuid[])`,
		userIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("get presence: query: %w", err)
	}
	defer rows.Close()

	lastSeen := make(map[string]*time.Time, len(userIDs))
	for rows.Next() {
		var id string
		var t *time.Time
		if err := rows.Scan(&id, &t); err != nil {
			return nil, fmt.Errorf("get presence: scan: %w", err)
		}
		lastSeen[id] = t
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get presence: rows: %w", err)
	}

	result := make([]Info, len(userIDs))
	for i, id := range userIDs {
		result[i] = Info{
			UserID:     id,
			Online:     online[id],
			LastSeenAt: lastSeen[id],
		}
	}
	return result, nil
}

// ContactIDs returns the user IDs of all accepted contacts for a given user.
// Used to fan-out presence_online SSE events.
func (s *Store) ContactIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.db.Query(ctx, `
		SELECT CASE
			WHEN requester_id = $1 THEN addressee_id::text
			ELSE requester_id::text
		END AS contact_user_id
		FROM contacts
		WHERE (requester_id = $1 OR addressee_id = $1)
		  AND status = 'accepted'`, userID)
	if err != nil {
		return nil, fmt.Errorf("contact ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("contact ids: scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
