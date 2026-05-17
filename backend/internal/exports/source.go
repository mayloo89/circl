package exports

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pgSource collects the per-user export bundle directly from Postgres.
// One method per logical resource so a future contributor can add or change a
// table without rewriting the orchestration layer.
type pgSource struct {
	pool *pgxpool.Pool
}

// NewSource returns a Postgres-backed Source.
func NewSource(pool *pgxpool.Pool) Source {
	return &pgSource{pool: pool}
}

// BuildBundle runs every per-resource query and assembles the JSON snapshot
// plus the media manifest. Queries are issued sequentially because (a) a
// single user's export is small enough that we don't need concurrency and
// (b) sequential SQL is easier to reason about than goroutine fan-out for
// what is fundamentally a read-only audit.
func (s *pgSource) BuildBundle(ctx context.Context, userID string) (*Bundle, error) {
	account, err := s.account(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("account: %w", err)
	}
	profile, avatarKey, err := s.profile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("profile: %w", err)
	}
	photos, photoKeys, err := s.profilePhotos(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("profile photos: %w", err)
	}
	prefs, err := s.preferences(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("preferences: %w", err)
	}
	contacts, err := s.contacts(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("contacts: %w", err)
	}
	blocks, err := s.blocks(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("blocks: %w", err)
	}
	rooms, err := s.rooms(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("rooms: %w", err)
	}
	msgs, err := s.messages(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("messages: %w", err)
	}
	reports, err := s.reports(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("reports: %w", err)
	}
	att, err := s.ageAttestations(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("age attestations: %w", err)
	}
	uploads, uploadMedia, err := s.uploads(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("uploads: %w", err)
	}

	media := make([]MediaItem, 0, 1+len(photoKeys)+len(uploadMedia))
	if avatarKey != "" {
		media = append(media, MediaItem{
			StorageKey:  avatarKey,
			ContentType: contentTypeForKey(avatarKey),
			ArchivePath: "media/avatar" + path.Ext(avatarKey),
		})
	}
	for _, k := range photoKeys {
		media = append(media, MediaItem{
			StorageKey:  k.key,
			ContentType: contentTypeForKey(k.key),
			ArchivePath: "media/photos/" + k.id + path.Ext(k.key),
		})
	}
	media = append(media, uploadMedia...)

	return &Bundle{
		Snapshot: Snapshot{
			SchemaVersion:   SchemaVersion,
			GeneratedAt:     time.Now().UTC(),
			Account:         *account,
			Profile:         profile,
			ProfilePhotos:   photos,
			Preferences:     prefs,
			Contacts:        contacts,
			Blocks:          blocks,
			Rooms:           rooms,
			Messages:        msgs,
			ReportsFiled:    reports,
			AgeAttestations: att,
			Uploads:         uploads,
		},
		Media: media,
	}, nil
}

func (s *pgSource) account(ctx context.Context, userID string) (*Account, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, email, status, role, created_at, email_verified_at,
		       terms_accepted_at, privacy_accepted_at, accepted_policy_version
		  FROM users
		 WHERE id = $1`, userID)
	var a Account
	if err := row.Scan(&a.ID, &a.Email, &a.Status, &a.Role, &a.CreatedAt, &a.EmailVerifiedAt,
		&a.TermsAcceptedAt, &a.PrivacyAcceptedAt, &a.AcceptedPolicyVersion); err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *pgSource) profile(ctx context.Context, userID string) (*Profile, string, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT COALESCE(p.username, ''), p.display_name, p.bio, p.date_of_birth,
		       COALESCE(p.gender, ''), COALESCE(p.location_text, ''),
		       p.latitude, p.longitude,
		       COALESCE(p.avatar_url, ''),
		       COALESCE(ARRAY(
		         SELECT i.name FROM profile_interests pi
		         JOIN interests i ON i.id = pi.interest_id
		         WHERE pi.user_id = p.user_id
		         ORDER BY i.name
		       ), '{}'::text[]),
		       COALESCE(p.looking_for_gender, '{}'::text[]),
		       p.looking_for_age_min, p.looking_for_age_max,
		       p.onboarded_at
		  FROM profiles p
		 WHERE p.user_id = $1`, userID)
	var p Profile
	if err := row.Scan(&p.Username, &p.DisplayName, &p.Bio, &p.DateOfBirth,
		&p.Gender, &p.Location, &p.Latitude, &p.Longitude, &p.AvatarURL,
		&p.Interests, &p.LookingForGender, &p.LookingForAgeMin, &p.LookingForAgeMax,
		&p.OnboardedAt); err != nil {
		if isNoRows(err) {
			return nil, "", nil
		}
		return nil, "", err
	}
	avatarKey := storageKeyFromURL(p.AvatarURL)
	return &p, avatarKey, nil
}

type photoKey struct {
	id  string
	key string
}

func (s *pgSource) profilePhotos(ctx context.Context, userID string) ([]ProfilePhoto, []photoKey, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, url, position, created_at
		  FROM profile_photos
		 WHERE user_id = $1
		 ORDER BY position, created_at`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	out := []ProfilePhoto{}
	var keys []photoKey
	for rows.Next() {
		var p ProfilePhoto
		if err := rows.Scan(&p.ID, &p.URL, &p.Position, &p.CreatedAt); err != nil {
			return nil, nil, err
		}
		out = append(out, p)
		if k := storageKeyFromURL(p.URL); k != "" {
			keys = append(keys, photoKey{id: p.ID, key: k})
		}
	}
	return out, keys, rows.Err()
}

func (s *pgSource) preferences(ctx context.Context, userID string) (*Preferences, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT min_age, max_age, max_distance_km, gender_preference,
		       require_photo, hide_distance_from_non_contacts, hide_presence,
		       hide_read_receipts, hide_typing_indicator,
		       notify_chat_messages, notify_contact_requests,
		       notify_channel_mentions, notify_system, locale
		  FROM profile_preferences
		 WHERE user_id = $1`, userID)
	var p Preferences
	if err := row.Scan(&p.MinAge, &p.MaxAge, &p.MaxDistanceKm, &p.GenderPreference,
		&p.RequirePhoto, &p.HideDistanceFromNonContacts, &p.HidePresence,
		&p.HideReadReceipts, &p.HideTypingIndicator,
		&p.NotifyChatMessages, &p.NotifyContactRequests,
		&p.NotifyChannelMentions, &p.NotifySystem, &p.Locale); err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (s *pgSource) contacts(ctx context.Context, userID string) ([]Contact, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT CASE WHEN c.requester_id = $1 THEN c.addressee_id ELSE c.requester_id END,
		       COALESCE(p.username, ''),
		       c.status, c.created_at
		  FROM contacts c
		  LEFT JOIN profiles p ON p.user_id = CASE WHEN c.requester_id = $1 THEN c.addressee_id ELSE c.requester_id END
		 WHERE (c.requester_id = $1 OR c.addressee_id = $1)
		   AND c.status = 'accepted'
		 ORDER BY c.created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Contact{}
	for rows.Next() {
		var c Contact
		if err := rows.Scan(&c.UserID, &c.Username, &c.Status, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *pgSource) blocks(ctx context.Context, userID string) ([]Block, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.addressee_id, COALESCE(p.username, ''), c.created_at
		  FROM contacts c
		  LEFT JOIN profiles p ON p.user_id = c.addressee_id
		 WHERE c.requester_id = $1 AND c.status = 'blocked'
		 ORDER BY c.created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Block{}
	for rows.Next() {
		var b Block
		if err := rows.Scan(&b.BlockedUserID, &b.BlockedUsername, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *pgSource) rooms(ctx context.Context, userID string) ([]Room, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.type, COALESCE(r.name, ''), r.created_at
		  FROM room_members rm
		  JOIN rooms r ON r.id = rm.room_id
		 WHERE rm.user_id = $1
		 ORDER BY r.created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Room{}
	for rows.Next() {
		var r Room
		if err := rows.Scan(&r.ID, &r.Kind, &r.Name, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *pgSource) messages(ctx context.Context, userID string) ([]Message, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, room_id, content, type, created_at
		  FROM messages
		 WHERE sender_id = $1
		 ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.RoomID, &m.Body, &m.ContentType, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *pgSource) reports(ctx context.Context, userID string) ([]ReportFiled, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, reported_user_id, reason, priority, COALESCE(description, ''), status, created_at
		  FROM reports
		 WHERE reporter_id = $1
		 ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReportFiled{}
	for rows.Next() {
		var r ReportFiled
		if err := rows.Scan(&r.ID, &r.ReportedUserID, &r.Reason, &r.Priority, &r.Description, &r.Status, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *pgSource) ageAttestations(ctx context.Context, userID string) ([]AgeAttestation, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT attested_age, host(ip)::text, user_agent, date_of_birth, policy_version, created_at
		  FROM age_verification_audit
		 WHERE user_id = $1
		 ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AgeAttestation{}
	for rows.Next() {
		var a AgeAttestation
		var ip *string
		if err := rows.Scan(&a.AttestedAge, &ip, &a.UserAgent, &a.DateOfBirth, &a.PolicyVersion, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.IP = ip
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *pgSource) uploads(ctx context.Context, userID string) ([]Upload, []MediaItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, storage_key, content_type, size_bytes, status, category, created_at
		  FROM uploads
		 WHERE user_id = $1 AND status = 'committed'
		 ORDER BY created_at`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	out := []Upload{}
	var media []MediaItem
	for rows.Next() {
		var u Upload
		var category string
		if err := rows.Scan(&u.ID, &u.StorageKey, &u.ContentType, &u.SizeBytes, &u.Status, &category, &u.CreatedAt); err != nil {
			return nil, nil, err
		}
		out = append(out, u)
		// Avatar uploads are already pulled in via the profile.avatar_url
		// path; chat-attachment uploads are the user's own media we still
		// need to include.
		if category == "chat-attachment" {
			media = append(media, MediaItem{
				StorageKey:  u.StorageKey,
				ContentType: u.ContentType,
				ArchivePath: "media/uploads/" + u.ID + path.Ext(u.StorageKey),
			})
		}
	}
	return out, media, rows.Err()
}

func isNoRows(err error) bool { return err != nil && err.Error() == pgx.ErrNoRows.Error() }

// storageKeyFromURL strips the storage provider's public URL prefix from a
// stored URL and returns the bare key. URLs we don't recognise are returned
// as-is — the builder will try to fetch them and will skip on error.
func storageKeyFromURL(u string) string {
	if u == "" {
		return ""
	}
	// Match the last segment after `/uploads/files/` (LocalStorage) or after
	// the bucket name (S3). Both providers serve files under a path that
	// embeds the bare key, so taking the path after the last bucket-style
	// segment is good enough for an export manifest.
	if _, after, ok := strings.Cut(u, "/uploads/files/"); ok {
		return after
	}
	// Fallback for S3-style URLs `https://host/bucket/key/...`: take the path
	// component after the host. Worst case we feed the full URL to GetObject;
	// the builder logs and skips on failure.
	if _, rest, ok := strings.Cut(u, "://"); ok {
		if _, afterHost, ok := strings.Cut(rest, "/"); ok {
			if _, afterBucket, ok := strings.Cut(afterHost, "/"); ok {
				return afterBucket
			}
		}
	}
	return u
}

func contentTypeForKey(key string) string {
	switch strings.ToLower(path.Ext(key)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".mp4":
		return "video/mp4"
	default:
		return "application/octet-stream"
	}
}
