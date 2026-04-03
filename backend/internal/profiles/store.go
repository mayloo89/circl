package profiles

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type rowScanner interface {
	Scan(dest ...any) error
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) rowScanner
}

type pgxQuerier struct{ pool *pgxpool.Pool }

func (q *pgxQuerier) QueryRow(ctx context.Context, sql string, args ...any) rowScanner {
	return q.pool.QueryRow(ctx, sql, args...)
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
	       ) AS interests
	  FROM profiles p`

func scanProfile(row rowScanner) (*Profile, error) {
	var p Profile
	if err := row.Scan(
		&p.ID, &p.UserID, &p.Username, &p.DisplayName, &p.Bio, &p.AvatarURL,
		&p.DateOfBirth, &p.Gender, &p.LocationText,
		&p.Latitude, &p.Longitude, &p.Interests,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan profile: %w", err)
	}
	if p.Interests == nil {
		p.Interests = []string{}
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
	row := s.db.QueryRow(ctx,
		`INSERT INTO profiles (user_id, username, display_name, bio, avatar_url,
		                       date_of_birth, gender, location_text, latitude, longitude)
		 VALUES ($1, NULLIF($2, ''), $3, $4, NULLIF($5, ''),
		         $6, NULLIF($7, ''), NULLIF($8, ''), $9, $10)
		 ON CONFLICT (user_id) DO UPDATE
		    SET username      = COALESCE(NULLIF(EXCLUDED.username, ''), profiles.username),
		        display_name  = EXCLUDED.display_name,
		        bio           = EXCLUDED.bio,
		        avatar_url    = EXCLUDED.avatar_url,
		        date_of_birth = EXCLUDED.date_of_birth,
		        gender        = EXCLUDED.gender,
		        location_text = EXCLUDED.location_text,
		        latitude      = EXCLUDED.latitude,
		        longitude     = EXCLUDED.longitude,
		        updated_at    = now()
		 RETURNING id, user_id, COALESCE(username, ''), display_name, bio, COALESCE(avatar_url, ''),
		           date_of_birth, COALESCE(gender, ''), COALESCE(location_text, ''),
		           latitude, longitude`,
		userID, in.Username, in.DisplayName, in.Bio, in.AvatarURL,
		in.DateOfBirth, in.Gender, in.LocationText, in.Latitude, in.Longitude,
	)

	var p Profile
	if err := row.Scan(
		&p.ID, &p.UserID, &p.Username, &p.DisplayName, &p.Bio, &p.AvatarURL,
		&p.DateOfBirth, &p.Gender, &p.LocationText,
		&p.Latitude, &p.Longitude,
	); err != nil {
		if isUniqueViolation(err, "profiles_username_key") {
			return nil, ErrUsernameTaken
		}
		return nil, fmt.Errorf("upsert profile: %w", err)
	}

	p.Interests = []string{}
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

const browseSQLBody = `
WITH r AS (
    SELECT latitude AS lat, longitude AS lng
    FROM profiles WHERE user_id = $1
    LIMIT 1
)
SELECT
    p.id,
    p.user_id,
    COALESCE(p.username, '') AS username,
    p.display_name,
    COALESCE(p.avatar_url, '') AS avatar_url,
    p.date_of_birth,
    COALESCE(p.gender, '') AS gender,
    COALESCE(p.location_text, '') AS location_text,
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
      OR r.lat IS NULL OR r.lng IS NULL
      OR p.latitude IS NULL OR p.longitude IS NULL
      OR 6371.0 * 2.0 * ASIN(SQRT(
          POWER(SIN(RADIANS((p.latitude - r.lat) / 2.0)), 2.0) +
          COS(RADIANS(r.lat)) * COS(RADIANS(p.latitude)) *
          POWER(SIN(RADIANS((p.longitude - r.lng) / 2.0)), 2.0)
      )) <= prefs.max_distance_km
  )
  AND (
      cardinality($4::text[]) = 0
      OR EXISTS (
          SELECT 1 FROM profile_interests pi
          JOIN interests i ON i.id = pi.interest_id
          WHERE pi.user_id = p.user_id AND i.name = ANY($4::text[])
      )
  )
  AND NOT EXISTS (
      SELECT 1 FROM blocks b
      WHERE (b.blocker_id = $1 AND b.blocked_id = p.user_id)
         OR (b.blocker_id = p.user_id AND b.blocked_id = $1)
  )`

// Browse returns a paginated list of profiles for the browse/explore view.
func (s *pgStore) Browse(ctx context.Context, userID string, limit, offset int, sortByDistance bool, interests []string) ([]BrowseProfile, error) {
	if interests == nil {
		interests = []string{}
	}
	orderBy := "p.created_at DESC, p.id ASC"
	if sortByDistance {
		orderBy = "distance_km ASC NULLS LAST, p.created_at DESC, p.id ASC"
	}
	sql := browseSQLBody + "\nORDER BY " + orderBy + "\nLIMIT $2 OFFSET $3"
	rows, err := s.pool.Query(ctx, sql, userID, limit, offset, interests)
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
			&p.DistanceKm, &firstPhotoURL, &p.Interests,
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

// GetPreferences returns the discovery preferences for the given user.
// Returns an empty preferences object if none have been set yet.
func (s *pgStore) GetPreferences(ctx context.Context, userID string) (*ProfilePreferences, error) {
	row := s.db.QueryRow(ctx,
		`SELECT user_id, min_age, max_age, max_distance_km, gender_preference
		   FROM profile_preferences
		  WHERE user_id = $1`,
		userID,
	)
	var p ProfilePreferences
	if err := row.Scan(&p.UserID, &p.MinAge, &p.MaxAge, &p.MaxDistanceKm, &p.GenderPreference); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &ProfilePreferences{UserID: userID, GenderPreference: []string{}}, nil
		}
		return nil, fmt.Errorf("get preferences: %w", err)
	}
	if p.GenderPreference == nil {
		p.GenderPreference = []string{}
	}
	return &p, nil
}

// UpsertPreferences inserts or updates the discovery preferences for the given user.
func (s *pgStore) UpsertPreferences(ctx context.Context, userID string, prefs ProfilePreferences) (*ProfilePreferences, error) {
	genderPref := prefs.GenderPreference
	if genderPref == nil {
		genderPref = []string{}
	}

	row := s.db.QueryRow(ctx,
		`INSERT INTO profile_preferences (user_id, min_age, max_age, max_distance_km, gender_preference)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (user_id) DO UPDATE
		    SET min_age           = EXCLUDED.min_age,
		        max_age           = EXCLUDED.max_age,
		        max_distance_km   = EXCLUDED.max_distance_km,
		        gender_preference = EXCLUDED.gender_preference,
		        updated_at        = now()
		 RETURNING user_id, min_age, max_age, max_distance_km, gender_preference`,
		userID, prefs.MinAge, prefs.MaxAge, prefs.MaxDistanceKm, genderPref,
	)
	var p ProfilePreferences
	if err := row.Scan(&p.UserID, &p.MinAge, &p.MaxAge, &p.MaxDistanceKm, &p.GenderPreference); err != nil {
		return nil, fmt.Errorf("upsert preferences: %w", err)
	}
	if p.GenderPreference == nil {
		p.GenderPreference = []string{}
	}
	return &p, nil
}
