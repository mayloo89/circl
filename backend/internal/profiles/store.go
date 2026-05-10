package profiles

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type rowScanner interface {
	Scan(dest ...any) error
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) rowScanner
	Exec(ctx context.Context, sql string, args ...any) error
}

type pgxQuerier struct{ pool *pgxpool.Pool }

func (q *pgxQuerier) QueryRow(ctx context.Context, sql string, args ...any) rowScanner {
	return q.pool.QueryRow(ctx, sql, args...)
}

func (q *pgxQuerier) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := q.pool.Exec(ctx, sql, args...)
	return err
}

type pgStore struct {
	db   querier
	pool *pgxpool.Pool
}

// NewStore returns a Store backed by the given pgx pool.
func NewStore(pool *pgxpool.Pool) Store {
	return &pgStore{db: &pgxQuerier{pool: pool}, pool: pool}
}

const profileSelectSQL = `
	SELECT p.id, p.user_id, COALESCE(p.username, ''), p.display_name, p.bio, COALESCE(p.avatar_url, ''),
	       p.date_of_birth, COALESCE(p.gender, ''), COALESCE(p.location_text, ''),
	       p.latitude, p.longitude,
	       ARRAY(
	           SELECT i.name FROM profile_interests pi
	             JOIN interests i ON i.id = pi.interest_id
	            WHERE pi.user_id = p.user_id
	            ORDER BY i.name
	       ) AS interests,
	       p.onboarded_at,
	       p.looking_for_gender, p.looking_for_age_min, p.looking_for_age_max
	  FROM profiles p`

func scanProfile(row rowScanner) (*Profile, error) {
	var p Profile
	if err := row.Scan(
		&p.ID, &p.UserID, &p.Username, &p.DisplayName, &p.Bio, &p.AvatarURL,
		&p.DateOfBirth, &p.Gender, &p.LocationText,
		&p.Latitude, &p.Longitude, &p.Interests, &p.OnboardedAt,
		&p.LookingForGender, &p.LookingForAgeMin, &p.LookingForAgeMax,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan profile: %w", err)
	}
	if p.Interests == nil {
		p.Interests = []string{}
	}
	if p.LookingForGender == nil {
		p.LookingForGender = []string{}
	}
	return &p, nil
}

func isUniqueViolation(err error, constraintName string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraintName
}

// GetByUserID returns the profile for the given user ID.
func (s *pgStore) GetByUserID(ctx context.Context, userID string) (*Profile, error) {
	row := s.db.QueryRow(ctx, profileSelectSQL+` WHERE p.user_id = $1`, userID)
	return scanProfile(row)
}

// GetByUsername returns the profile for the given username.
func (s *pgStore) GetByUsername(ctx context.Context, username string) (*Profile, error) {
	row := s.db.QueryRow(ctx, profileSelectSQL+` WHERE p.username = $1`, username)
	return scanProfile(row)
}

// IsUsernameAvailable returns true if no profile has the given username.
func (s *pgStore) IsUsernameAvailable(ctx context.Context, username string) (bool, error) {
	var exists bool
	row := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM profiles WHERE username = $1)`,
		username,
	)
	if err := row.Scan(&exists); err != nil {
		return false, fmt.Errorf("check username: %w", err)
	}
	return !exists, nil
}

// Upsert inserts or updates the profile for the given user ID.
// Interests are managed separately via SyncInterests.
func (s *pgStore) Upsert(ctx context.Context, userID string, in ProfileInput) (*Profile, error) {
	genders := in.LookingForGender
	if genders == nil {
		genders = []string{}
	}
	row := s.db.QueryRow(ctx,
		`INSERT INTO profiles (user_id, username, display_name, bio, avatar_url,
		                       date_of_birth, gender, location_text, latitude, longitude,
		                       onboarded_at,
		                       looking_for_gender, looking_for_age_min, looking_for_age_max)
		 VALUES ($1, NULLIF($2, ''), $3, $4, NULLIF($5, ''),
		         $6, NULLIF($7, ''), NULLIF($8, ''), $9, $10,
		         $11,
		         $12, $13, $14)
		 ON CONFLICT (user_id) DO UPDATE
		    SET username             = COALESCE(NULLIF(EXCLUDED.username, ''), profiles.username),
		        display_name         = COALESCE(NULLIF(EXCLUDED.display_name, ''), profiles.display_name),
		        bio                  = EXCLUDED.bio,
		        avatar_url           = EXCLUDED.avatar_url,
		        date_of_birth        = EXCLUDED.date_of_birth,
		        gender               = EXCLUDED.gender,
		        location_text        = EXCLUDED.location_text,
		        latitude             = EXCLUDED.latitude,
		        longitude            = EXCLUDED.longitude,
		        onboarded_at         = COALESCE(profiles.onboarded_at, EXCLUDED.onboarded_at),
		        looking_for_gender   = EXCLUDED.looking_for_gender,
		        looking_for_age_min  = EXCLUDED.looking_for_age_min,
		        looking_for_age_max  = EXCLUDED.looking_for_age_max,
		        updated_at           = now()
		 RETURNING id, user_id, COALESCE(username, ''), display_name, bio, COALESCE(avatar_url, ''),
		           date_of_birth, COALESCE(gender, ''), COALESCE(location_text, ''),
		           latitude, longitude, onboarded_at,
		           looking_for_gender, looking_for_age_min, looking_for_age_max`,
		userID, in.Username, in.DisplayName, in.Bio, in.AvatarURL,
		in.DateOfBirth, in.Gender, in.LocationText, in.Latitude, in.Longitude,
		in.OnboardedAt,
		genders, in.LookingForAgeMin, in.LookingForAgeMax,
	)

	var p Profile
	if err := row.Scan(
		&p.ID, &p.UserID, &p.Username, &p.DisplayName, &p.Bio, &p.AvatarURL,
		&p.DateOfBirth, &p.Gender, &p.LocationText,
		&p.Latitude, &p.Longitude, &p.OnboardedAt,
		&p.LookingForGender, &p.LookingForAgeMin, &p.LookingForAgeMax,
	); err != nil {
		if isUniqueViolation(err, "profiles_username_key") {
			return nil, ErrUsernameTaken
		}
		return nil, fmt.Errorf("upsert profile: %w", err)
	}

	p.Interests = []string{}
	if p.LookingForGender == nil {
		p.LookingForGender = []string{}
	}
	return &p, nil
}

// SyncInterests replaces all interests for the given user atomically.
func (s *pgStore) SyncInterests(ctx context.Context, userID string, names []string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err = tx.Exec(ctx, `DELETE FROM profile_interests WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("clear interests: %w", err)
	}

	for _, name := range names {
		if _, err = tx.Exec(ctx,
			`INSERT INTO interests (name) VALUES ($1) ON CONFLICT (name) DO NOTHING`,
			name,
		); err != nil {
			return fmt.Errorf("upsert interest %q: %w", name, err)
		}
		if _, err = tx.Exec(ctx,
			`INSERT INTO profile_interests (user_id, interest_id)
			 SELECT $1, id FROM interests WHERE name = $2
			 ON CONFLICT DO NOTHING`,
			userID, name,
		); err != nil {
			return fmt.Errorf("link interest %q: %w", name, err)
		}
	}

	return tx.Commit(ctx)
}

// SearchInterests returns interest suggestions matching the given prefix, ordered by usage count.
func (s *pgStore) SearchInterests(ctx context.Context, query string, limit int) ([]InterestSuggestion, error) {
	var rows pgx.Rows
	var err error
	if query == "" {
		rows, err = s.pool.Query(ctx,
			`SELECT i.name, COUNT(pi.user_id) AS usage_count
			   FROM interests i
			   LEFT JOIN profile_interests pi ON pi.interest_id = i.id
			  GROUP BY i.id
			  ORDER BY usage_count DESC, i.name
			  LIMIT $1`,
			limit,
		)
	} else {
		rows, err = s.pool.Query(ctx,
			`SELECT i.name, COUNT(pi.user_id) AS usage_count
			   FROM interests i
			   LEFT JOIN profile_interests pi ON pi.interest_id = i.id
			  WHERE i.name ILIKE $1 || '%'
			  GROUP BY i.id
			  ORDER BY usage_count DESC, i.name
			  LIMIT $2`,
			query, limit,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("search interests: %w", err)
	}
	defer rows.Close()

	var suggestions []InterestSuggestion
	for rows.Next() {
		var s InterestSuggestion
		if err := rows.Scan(&s.Name, &s.Count); err != nil {
			return nil, fmt.Errorf("scan interest: %w", err)
		}
		suggestions = append(suggestions, s)
	}
	if suggestions == nil {
		suggestions = []InterestSuggestion{}
	}
	return suggestions, rows.Err()
}

// UpdateAvatar updates only the avatar_url for the given user.
func (s *pgStore) UpdateAvatar(ctx context.Context, userID, avatarURL string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE profiles SET avatar_url = NULLIF($2, ''), updated_at = now() WHERE user_id = $1`,
		userID, avatarURL,
	)
	if err != nil {
		return fmt.Errorf("update avatar: %w", err)
	}
	return nil
}

// GetPhotosByUserID returns all showcase photos for a user ordered by position.
func (s *pgStore) GetPhotosByUserID(ctx context.Context, userID string) ([]ProfilePhoto, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, url FROM profile_photos WHERE user_id = $1 ORDER BY position, created_at`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get profile photos: %w", err)
	}
	defer rows.Close()

	var photos []ProfilePhoto
	for rows.Next() {
		var p ProfilePhoto
		if err := rows.Scan(&p.ID, &p.URL); err != nil {
			return nil, fmt.Errorf("scan profile photo: %w", err)
		}
		photos = append(photos, p)
	}
	if photos == nil {
		photos = []ProfilePhoto{}
	}
	return photos, rows.Err()
}

// CountPhotos returns the number of showcase photos for a user.
func (s *pgStore) CountPhotos(ctx context.Context, userID string) (int, error) {
	var count int
	row := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM profile_photos WHERE user_id = $1`,
		userID,
	)
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("count profile photos: %w", err)
	}
	return count, nil
}

// AddPhoto inserts a new showcase photo and returns it.
func (s *pgStore) AddPhoto(ctx context.Context, userID, url string) (*ProfilePhoto, error) {
	row := s.db.QueryRow(ctx,
		`INSERT INTO profile_photos (user_id, url, position)
		 VALUES ($1, $2, (SELECT COALESCE(MAX(position) + 1, 0) FROM profile_photos WHERE user_id = $1))
		 RETURNING id, url`,
		userID, url,
	)
	var p ProfilePhoto
	if err := row.Scan(&p.ID, &p.URL); err != nil {
		return nil, fmt.Errorf("add profile photo: %w", err)
	}
	return &p, nil
}

// DeletePhoto removes a photo, verifying it belongs to the given user.
func (s *pgStore) DeletePhoto(ctx context.Context, photoID, userID string) error {
	result, err := s.pool.Exec(ctx,
		`DELETE FROM profile_photos WHERE id = $1 AND user_id = $2`,
		photoID, userID,
	)
	if err != nil {
		return fmt.Errorf("delete profile photo: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrPhotoNotFound
	}
	return nil
}

// browseCTEBase is the first part of the browse query, a CTE that materialises
// all candidate profiles with their computed distance. It uses two fixed params:
//   $1 = userID (the requester)
//   $2 = interests filter (text[])
//
// Callers append an optional WHERE (cursor), ORDER BY, and LIMIT clause.
const browseCTEBase = `
WITH r AS (
    SELECT latitude AS lat, longitude AS lng
    FROM profiles WHERE user_id = $1
    LIMIT 1
),
candidates AS (
    SELECT
        p.id,
        p.user_id,
        COALESCE(p.username, '') AS username,
        p.display_name,
        COALESCE(p.avatar_url, '') AS avatar_url,
        p.date_of_birth,
        COALESCE(p.gender, '') AS gender,
        COALESCE(p.location_text, '') AS location_text,
        p.created_at,
        CASE
            WHEN r.lat IS NOT NULL AND r.lng IS NOT NULL
                 AND p.latitude IS NOT NULL AND p.longitude IS NOT NULL
            THEN 6371.0 * 2.0 * ASIN(SQRT(
                POWER(SIN(RADIANS((p.latitude - r.lat) / 2.0)), 2.0) +
                COS(RADIANS(r.lat)) * COS(RADIANS(p.latitude)) *
                POWER(SIN(RADIANS((p.longitude - r.lng) / 2.0)), 2.0)
            ))
            ELSE NULL
        END AS distance_km,
        (SELECT url FROM profile_photos ph
         WHERE ph.user_id = p.user_id
         ORDER BY ph.position, ph.created_at
         LIMIT 1) AS first_photo_url,
        ARRAY(
            SELECT i.name FROM profile_interests pi
            JOIN interests i ON i.id = pi.interest_id
            WHERE pi.user_id = p.user_id
            ORDER BY i.name
        ) AS interests
    FROM profiles p
    LEFT JOIN LATERAL (SELECT lat, lng FROM r LIMIT 1) r ON true
    LEFT JOIN profile_preferences prefs ON prefs.user_id = $1
    WHERE p.user_id <> $1
      AND p.username IS NOT NULL
      AND p.date_of_birth IS NOT NULL
      AND NOT EXISTS (
          SELECT 1 FROM contacts c
          WHERE (c.requester_id = $1 AND c.addressee_id = p.user_id)
             OR (c.requester_id = p.user_id AND c.addressee_id = $1)
      )
      AND (prefs.min_age IS NULL
           OR DATE_PART('year', AGE(CURRENT_DATE, p.date_of_birth::date)) >= prefs.min_age)
      AND (prefs.max_age IS NULL
           OR DATE_PART('year', AGE(CURRENT_DATE, p.date_of_birth::date)) <= prefs.max_age)
      AND (prefs.gender_preference IS NULL
           OR array_length(prefs.gender_preference, 1) IS NULL
           OR COALESCE(p.gender, '') = ANY(prefs.gender_preference))
      AND (
          prefs.max_distance_km IS NULL
          OR (
              r.lat IS NOT NULL AND r.lng IS NOT NULL
              AND p.latitude IS NOT NULL AND p.longitude IS NOT NULL
              AND 6371.0 * 2.0 * ASIN(SQRT(
                  POWER(SIN(RADIANS((p.latitude - r.lat) / 2.0)), 2.0) +
                  COS(RADIANS(r.lat)) * COS(RADIANS(p.latitude)) *
                  POWER(SIN(RADIANS((p.longitude - r.lng) / 2.0)), 2.0)
              )) <= prefs.max_distance_km
          )
      )
      AND (
          cardinality($2::text[]) = 0
          OR EXISTS (
              SELECT 1 FROM profile_interests pi
              JOIN interests i ON i.id = pi.interest_id
              WHERE pi.user_id = p.user_id AND i.name = ANY($2::text[])
          )
      )
      AND NOT EXISTS (
          SELECT 1 FROM blocks b
          WHERE (b.blocker_id = $1 AND b.blocked_id = p.user_id)
             OR (b.blocker_id = p.user_id AND b.blocked_id = $1)
      )
      AND EXISTS (
          SELECT 1 FROM users u WHERE u.id = p.user_id AND u.status = 'active'
      )
      -- "Hide profiles without a photo" filter — only applies when the
      -- viewer has the toggle on. IS NOT TRUE handles NULL safely (no
      -- preferences row → treat as off → no filter).
      AND (
          prefs.require_photo IS NOT TRUE
          OR (p.avatar_url IS NOT NULL AND p.avatar_url <> '')
      )
)
SELECT id, user_id, username, display_name, avatar_url, date_of_birth,
       gender, location_text, created_at, distance_km, first_photo_url, interests
FROM candidates`

// Browse returns a cursor-paginated list of profiles for the browse/explore view.
// cursor is an opaque token returned by a previous call ("" for the first page).
// limit should be limit+1 (the caller trims and encodes the next cursor).
func (s *pgStore) Browse(ctx context.Context, userID string, limit int, cursor string, sortByDistance bool, interests []string) ([]BrowseProfile, error) {
	if interests == nil {
		interests = []string{}
	}

	args := []any{userID, interests} // $1, $2
	nextArg := 3

	var cursorClause string
	if cursor != "" {
		cur, err := DecodeBrowseCursor(cursor)
		if err != nil {
			return nil, fmt.Errorf("browse profiles: invalid cursor: %w", err)
		}
		if sortByDistance {
			// ORDER BY distance_km ASC NULLS LAST, created_at DESC, id ASC
			if cur.DistanceKm != nil {
				// Rows after (dk, ca, id): dk greater, OR dk equal + (ca smaller, OR ca equal + id greater), OR dk IS NULL (NULLS LAST)
				args = append(args, *cur.DistanceKm, cur.CreatedAt, cur.ID)
				p := [3]int{nextArg, nextArg + 1, nextArg + 2}
				cursorClause = "\nWHERE distance_km > $" + strconv.Itoa(p[0]) +
					" OR distance_km IS NULL" +
					" OR (distance_km = $" + strconv.Itoa(p[0]) +
					" AND (created_at < $" + strconv.Itoa(p[1]) +
					" OR (created_at = $" + strconv.Itoa(p[1]) +
					" AND id > $" + strconv.Itoa(p[2]) + ")))"
				nextArg += 3
			} else {
				// cursor's dk is NULL → among NULLS-LAST tail; continue by (ca DESC, id ASC)
				args = append(args, cur.CreatedAt, cur.ID)
				p := [2]int{nextArg, nextArg + 1}
				cursorClause = "\nWHERE distance_km IS NULL AND (created_at < $" + strconv.Itoa(p[0]) +
					" OR (created_at = $" + strconv.Itoa(p[0]) +
					" AND id > $" + strconv.Itoa(p[1]) + "))"
				nextArg += 2
			}
		} else {
			// ORDER BY created_at DESC, id ASC
			args = append(args, cur.CreatedAt, cur.ID)
			p := [2]int{nextArg, nextArg + 1}
			cursorClause = "\nWHERE created_at < $" + strconv.Itoa(p[0]) +
				" OR (created_at = $" + strconv.Itoa(p[0]) +
				" AND id > $" + strconv.Itoa(p[1]) + ")"
			nextArg += 2
		}
	}

	var orderBy string
	if sortByDistance {
		orderBy = "distance_km ASC NULLS LAST, created_at DESC, id ASC"
	} else {
		orderBy = "created_at DESC, id ASC"
	}

	args = append(args, limit)
	limitClause := "\nORDER BY " + orderBy + "\nLIMIT $" + strconv.Itoa(nextArg)

	sql := browseCTEBase + cursorClause + limitClause

	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("browse profiles: %w", err)
	}
	defer rows.Close()

	var profiles []BrowseProfile
	for rows.Next() {
		var p BrowseProfile
		var firstPhotoURL *string
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.Username, &p.DisplayName, &p.AvatarURL,
			&p.DateOfBirth, &p.Gender, &p.LocationText,
			&p.CreatedAt, &p.DistanceKm, &firstPhotoURL, &p.Interests,
		); err != nil {
			return nil, fmt.Errorf("scan browse profile: %w", err)
		}
		if firstPhotoURL != nil {
			p.FirstPhotoURL = *firstPhotoURL
		}
		if p.Interests == nil {
			p.Interests = []string{}
		}
		profiles = append(profiles, p)
	}
	if profiles == nil {
		profiles = []BrowseProfile{}
	}
	return profiles, rows.Err()
}

// GetPreferences returns the discovery, privacy, and notification preferences
// for the given user. Returns a defaults object if no row has been written
// yet — defaults match the SQL column defaults (privacy toggles off,
// notification toggles on).
func (s *pgStore) GetPreferences(ctx context.Context, userID string) (*ProfilePreferences, error) {
	row := s.db.QueryRow(ctx,
		`SELECT user_id, min_age, max_age, max_distance_km, gender_preference, locale,
		        require_photo,
		        hide_distance_from_non_contacts, hide_presence, hide_read_receipts, hide_typing_indicator,
		        notify_chat_messages, notify_contact_requests, notify_channel_mentions, notify_system
		   FROM profile_preferences
		  WHERE user_id = $1`,
		userID,
	)
	var p ProfilePreferences
	if err := row.Scan(
		&p.UserID, &p.MinAge, &p.MaxAge, &p.MaxDistanceKm, &p.GenderPreference, &p.Locale,
		&p.RequirePhoto,
		&p.HideDistanceFromNonContacts, &p.HidePresence, &p.HideReadReceipts, &p.HideTypingIndicator,
		&p.NotifyChatMessages, &p.NotifyContactRequests, &p.NotifyChannelMentions, &p.NotifySystem,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return defaultPreferences(userID), nil
		}
		return nil, fmt.Errorf("get preferences: %w", err)
	}
	if p.GenderPreference == nil {
		p.GenderPreference = []string{}
	}
	return &p, nil
}

// defaultPreferences returns the same defaults the SQL layer would produce
// for a fresh row: privacy toggles off, notification toggles on, locale "es".
func defaultPreferences(userID string) *ProfilePreferences {
	return &ProfilePreferences{
		UserID:                userID,
		GenderPreference:      []string{},
		Locale:                "es",
		NotifyChatMessages:    true,
		NotifyContactRequests: true,
		NotifyChannelMentions: true,
		NotifySystem:          true,
	}
}

// GetPrivacyFlagsByIDs returns the privacy toggles for each requested user
// keyed by user_id. Users with no preferences row are absent from the map;
// callers should treat absence as the zero PrivacyFlags (all false).
func (s *pgStore) GetPrivacyFlagsByIDs(ctx context.Context, userIDs []string) (map[string]PrivacyFlags, error) {
	if len(userIDs) == 0 {
		return map[string]PrivacyFlags{}, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT user_id::text, hide_distance_from_non_contacts, hide_presence,
		        hide_read_receipts, hide_typing_indicator
		   FROM profile_preferences
		  WHERE user_id = ANY($1::uuid[])`,
		userIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("get privacy flags: %w", err)
	}
	defer rows.Close()

	out := make(map[string]PrivacyFlags, len(userIDs))
	for rows.Next() {
		var (
			id    string
			flags PrivacyFlags
		)
		if err := rows.Scan(&id, &flags.HideDistanceFromNonContacts, &flags.HidePresence,
			&flags.HideReadReceipts, &flags.HideTypingIndicator); err != nil {
			return nil, fmt.Errorf("get privacy flags: scan: %w", err)
		}
		out[id] = flags
	}
	return out, rows.Err()
}

// AcceptedContactIDs returns the user IDs of the given user's accepted
// contacts. Used to gate per-viewer visibility in browse and presence.
func (s *pgStore) AcceptedContactIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT CASE
			WHEN requester_id = $1 THEN addressee_id::text
			ELSE requester_id::text
		END AS contact_user_id
		FROM contacts
		WHERE (requester_id = $1 OR addressee_id = $1)
		  AND status = 'accepted'`, userID)
	if err != nil {
		return nil, fmt.Errorf("accepted contact ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("accepted contact ids: scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// UpsertPreferences applies a partial update to the user's preferences row,
// inserting one with column defaults if it does not yet exist. Each field
// in update is independently dispatched: fields the caller did not specify
// (Optional[T].Set == false, or nil pointer) are omitted from both the
// INSERT column list and the ON CONFLICT SET clause, so existing values
// are preserved on update and the SQL DEFAULT is applied on insert.
func (s *pgStore) UpsertPreferences(ctx context.Context, userID string, update PreferencesUpdate) (*ProfilePreferences, error) {
	cols := []string{"user_id"}
	placeholders := []string{"$1"}
	sets := []string{"updated_at = now()"}
	args := []any{userID}

	addField := func(col string, value any) {
		idx := len(args) + 1
		cols = append(cols, col)
		placeholders = append(placeholders, fmt.Sprintf("$%d", idx))
		sets = append(sets, fmt.Sprintf("%s = $%d", col, idx))
		args = append(args, value)
	}

	if update.MinAge.Set {
		addField("min_age", update.MinAge.Value)
	}
	if update.MaxAge.Set {
		addField("max_age", update.MaxAge.Value)
	}
	if update.MaxDistanceKm.Set {
		addField("max_distance_km", update.MaxDistanceKm.Value)
	}
	if update.GenderPreference != nil {
		addField("gender_preference", *update.GenderPreference)
	}
	if update.Locale != nil {
		addField("locale", *update.Locale)
	}
	if update.RequirePhoto != nil {
		addField("require_photo", *update.RequirePhoto)
	}
	if update.HideDistanceFromNonContacts != nil {
		addField("hide_distance_from_non_contacts", *update.HideDistanceFromNonContacts)
	}
	if update.HidePresence != nil {
		addField("hide_presence", *update.HidePresence)
	}
	if update.HideReadReceipts != nil {
		addField("hide_read_receipts", *update.HideReadReceipts)
	}
	if update.HideTypingIndicator != nil {
		addField("hide_typing_indicator", *update.HideTypingIndicator)
	}
	if update.NotifyChatMessages != nil {
		addField("notify_chat_messages", *update.NotifyChatMessages)
	}
	if update.NotifyContactRequests != nil {
		addField("notify_contact_requests", *update.NotifyContactRequests)
	}
	if update.NotifyChannelMentions != nil {
		addField("notify_channel_mentions", *update.NotifyChannelMentions)
	}
	if update.NotifySystem != nil {
		addField("notify_system", *update.NotifySystem)
	}

	// On a no-op update (no fields supplied) the INSERT degenerates to an
	// INSERT (user_id) with a plain DO NOTHING clause; the row is created
	// with all column defaults if it was missing, otherwise nothing changes.
	conflictClause := "DO UPDATE SET " + strings.Join(sets, ", ")
	if len(cols) == 1 {
		conflictClause = "DO NOTHING"
	}

	sql := fmt.Sprintf(`
		INSERT INTO profile_preferences (%s)
		VALUES (%s)
		ON CONFLICT (user_id) %s`,
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "),
		conflictClause,
	)

	if err := s.db.Exec(ctx, sql, args...); err != nil {
		return nil, fmt.Errorf("upsert preferences: %w", err)
	}
	return s.GetPreferences(ctx, userID)
}
