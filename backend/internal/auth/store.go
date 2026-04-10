package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// rowScanner is implemented by pgx.Row and allows mocking in tests.
type rowScanner interface {
	Scan(dest ...any) error
}

// rows is the minimal interface for multi-row query results.
type rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// querier is the minimal DB interface required by pgStore.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) rowScanner
	Query(ctx context.Context, sql string, args ...any) (rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// pgxQuerier adapts *pgxpool.Pool to the querier interface.
type pgxQuerier struct{ pool *pgxpool.Pool }

func (q *pgxQuerier) QueryRow(ctx context.Context, sql string, args ...any) rowScanner {
	return q.pool.QueryRow(ctx, sql, args...)
}

func (q *pgxQuerier) Query(ctx context.Context, sql string, args ...any) (rows, error) {
	return q.pool.Query(ctx, sql, args...)
}

func (q *pgxQuerier) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return q.pool.Exec(ctx, sql, args...)
}

// pgStore implements Store using a querier (backed by pgx in production).
type pgStore struct{ db querier }

// NewStore returns a Store backed by the given pgx pool.
func NewStore(pool *pgxpool.Pool) Store {
	return &pgStore{db: &pgxQuerier{pool: pool}}
}

func (s *pgStore) GetUserByEmail(ctx context.Context, email string) (*userRecord, error) {
	row := s.db.QueryRow(ctx,
		`SELECT id, email, password_hash, status, is_admin, email_verified_at, deleted_at
		   FROM users
		  WHERE email = $1
		  LIMIT 1`,
		email,
	)
	var u userRecord
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Status, &u.IsAdmin, &u.EmailVerifiedAt, &u.DeletedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("query user: %w", err)
	}
	return &u, nil
}

func (s *pgStore) GetUserByID(ctx context.Context, userID string) (*userRecord, error) {
	row := s.db.QueryRow(ctx,
		`SELECT id, email, password_hash, status, is_admin, email_verified_at, deleted_at
		   FROM users
		  WHERE id = $1
		  LIMIT 1`,
		userID,
	)
	var u userRecord
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Status, &u.IsAdmin, &u.EmailVerifiedAt, &u.DeletedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("query user by id: %w", err)
	}
	return &u, nil
}

func (s *pgStore) UpdatePassword(ctx context.Context, userID, newHash string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE users SET password_hash = $1 WHERE id = $2`,
		newHash, userID,
	)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}

// DeleteUser soft-deletes the account: sets status='deleted' and deleted_at=now().
func (s *pgStore) DeleteUser(ctx context.Context, userID string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE users SET status = 'deleted', deleted_at = now() WHERE id = $1`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

// ReactivateUser restores a soft-deleted account within the grace period.
func (s *pgStore) ReactivateUser(ctx context.Context, userID string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE users SET status = 'active', deleted_at = NULL WHERE id = $1`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("reactivate user: %w", err)
	}
	return nil
}

// GetExpiredDeletedUserIDs returns IDs of accounts that were soft-deleted before the cutoff.
func (s *pgStore) GetExpiredDeletedUserIDs(ctx context.Context, before time.Time) ([]string, error) {
	r, err := s.db.Query(ctx,
		`SELECT id FROM users WHERE status = 'deleted' AND deleted_at < $1`,
		before,
	)
	if err != nil {
		return nil, fmt.Errorf("get expired deleted users: %w", err)
	}
	defer r.Close()
	var ids []string
	for r.Next() {
		var id string
		if err := r.Scan(&id); err != nil {
			return nil, fmt.Errorf("get expired deleted users: scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, r.Err()
}

// GetUserUploadKeys returns the storage keys for all committed uploads owned by userID.
// Returns two parallel slices: the original keys and thumbnail keys (thumbnail may be empty).
func (s *pgStore) GetUserUploadKeys(ctx context.Context, userID string) (storageKeys, thumbnailKeys []string, err error) {
	r, err := s.db.Query(ctx,
		`SELECT storage_key, COALESCE(thumbnail_key, '')
		   FROM uploads
		  WHERE user_id = $1 AND status = 'committed'`,
		userID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("get user upload keys: %w", err)
	}
	defer r.Close()
	for r.Next() {
		var sk, tk string
		if err := r.Scan(&sk, &tk); err != nil {
			return nil, nil, fmt.Errorf("get user upload keys: scan: %w", err)
		}
		storageKeys = append(storageKeys, sk)
		thumbnailKeys = append(thumbnailKeys, tk)
	}
	return storageKeys, thumbnailKeys, r.Err()
}

// DeleteUserData removes all user-associated records except the users row itself
// (which is preserved for message FK integrity and anonymized separately).
// Deleted: uploads, profile_photos, profiles, contacts, push_subscriptions, room_members.
// Preserved: messages (shown as "deleted user"), reports (audit trail).
func (s *pgStore) DeleteUserData(ctx context.Context, userID string) error {
	stmts := []string{
		`DELETE FROM uploads           WHERE user_id      = $1`,
		`DELETE FROM profile_photos    WHERE user_id      = $1`,
		`DELETE FROM profiles          WHERE user_id      = $1`,
		`DELETE FROM contacts          WHERE requester_id = $1 OR addressee_id = $1`,
		`DELETE FROM push_subscriptions WHERE user_id     = $1`,
		`DELETE FROM room_members      WHERE user_id      = $1`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(ctx, stmt, userID); err != nil {
			return fmt.Errorf("delete user data (%s): %w", stmt[:40], err)
		}
	}
	return nil
}

// AnonymizeUser replaces PII on the users row with inert placeholders and
// marks status='purged'. The row is kept to preserve message sender_id FKs.
func (s *pgStore) AnonymizeUser(ctx context.Context, userID string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE users
		    SET email         = 'deleted-' || id || '@purged',
		        password_hash = '',
		        status        = 'purged'
		  WHERE id = $1`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("anonymize user: %w", err)
	}
	return nil
}

func (s *pgStore) CreateUser(ctx context.Context, email, passwordHash string) (*userRecord, error) {
	row := s.db.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, provider, status)
		 VALUES ($1, $2, 'local', 'active')
		 RETURNING id, email, password_hash, status, is_admin, email_verified_at, deleted_at`,
		email, passwordHash,
	)
	var u userRecord
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Status, &u.IsAdmin, &u.EmailVerifiedAt, &u.DeletedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &u, nil
}

// --- Password resets ---

func (s *pgStore) CreatePasswordReset(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO password_resets (user_id, token_hash, expires_at)
		 VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("create password reset: %w", err)
	}
	return nil
}

func (s *pgStore) GetPasswordReset(ctx context.Context, tokenHash string) (*passwordResetRecord, error) {
	row := s.db.QueryRow(ctx,
		`SELECT id, user_id, token_hash, expires_at, used_at
		   FROM password_resets
		  WHERE token_hash = $1
		  LIMIT 1`,
		tokenHash,
	)
	var r passwordResetRecord
	if err := row.Scan(&r.ID, &r.UserID, &r.TokenHash, &r.ExpiresAt, &r.UsedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("reset token not found")
		}
		return nil, fmt.Errorf("query password reset: %w", err)
	}
	return &r, nil
}

func (s *pgStore) MarkPasswordResetUsed(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE password_resets SET used_at = now() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("mark reset used: %w", err)
	}
	return nil
}

// --- Email verifications ---

func (s *pgStore) CreateEmailVerification(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO email_verifications (user_id, token_hash, expires_at)
		 VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("create email verification: %w", err)
	}
	return nil
}

func (s *pgStore) GetEmailVerification(ctx context.Context, tokenHash string) (*emailVerificationRecord, error) {
	row := s.db.QueryRow(ctx,
		`SELECT id, user_id, token_hash, expires_at, verified_at
		   FROM email_verifications
		  WHERE token_hash = $1
		  LIMIT 1`,
		tokenHash,
	)
	var r emailVerificationRecord
	if err := row.Scan(&r.ID, &r.UserID, &r.TokenHash, &r.ExpiresAt, &r.VerifiedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("verification token not found")
		}
		return nil, fmt.Errorf("query email verification: %w", err)
	}
	return &r, nil
}

// MarkEmailVerified marks the verification record as used and sets email_verified_at
// on the user in a single CTE-based statement to avoid partial updates.
func (s *pgStore) MarkEmailVerified(ctx context.Context, userID, verificationID string) error {
	_, err := s.db.Exec(ctx,
		`WITH mark AS (
		     UPDATE email_verifications
		        SET verified_at = now()
		      WHERE id = $1 AND user_id = $2 AND verified_at IS NULL
		     RETURNING user_id
		 )
		 UPDATE users
		    SET email_verified_at = now()
		  WHERE id = (SELECT user_id FROM mark)`,
		verificationID, userID,
	)
	if err != nil {
		return fmt.Errorf("mark email verified: %w", err)
	}
	return nil
}
