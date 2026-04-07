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

// querier is the minimal DB interface required by pgStore.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) rowScanner
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// pgxQuerier adapts *pgxpool.Pool to the querier interface.
type pgxQuerier struct{ pool *pgxpool.Pool }

func (q *pgxQuerier) QueryRow(ctx context.Context, sql string, args ...any) rowScanner {
	return q.pool.QueryRow(ctx, sql, args...)
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

// PurgeExpiredDeletedUsers anonymizes accounts deleted before the given cutoff.
// Returns the number of rows affected.
func (s *pgStore) PurgeExpiredDeletedUsers(ctx context.Context, before time.Time) (int64, error) {
	tag, err := s.db.Exec(ctx,
		`UPDATE users
		    SET email        = 'deleted-' || id || '@purged',
		        password_hash = '',
		        status        = 'purged',
		        deleted_at    = deleted_at  -- preserve for audit
		  WHERE status = 'deleted'
		    AND deleted_at < $1`,
		before,
	)
	if err != nil {
		return 0, fmt.Errorf("purge deleted users: %w", err)
	}
	return tag.RowsAffected(), nil
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
