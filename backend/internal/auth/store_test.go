package auth

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// mockQuerier is a test double for the querier interface.
type mockQuerier struct {
	row      rowScanner
	execErr  error
	queryErr error
	queryRows [][]any // rows returned by Query, each element is one row's values
}

func (m *mockQuerier) QueryRow(_ context.Context, _ string, _ ...any) rowScanner {
	return m.row
}

func (m *mockQuerier) Query(_ context.Context, _ string, _ ...any) (rows, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	return &mockRows{data: m.queryRows}, nil
}

func (m *mockQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, m.execErr
}

// mockRows is a test double for the rows interface.
type mockRows struct {
	data [][]any
	pos  int
}

func (r *mockRows) Next() bool   { r.pos++; return r.pos <= len(r.data) }
func (r *mockRows) Err() error   { return nil }
func (r *mockRows) Close()       {}
func (r *mockRows) Scan(dest ...any) error {
	row := r.data[r.pos-1]
	for i, d := range dest {
		if i >= len(row) {
			break
		}
		switch v := d.(type) {
		case *string:
			*v = row[i].(string)
		}
	}
	return nil
}

// mockRow is a test double for rowScanner.
type mockRow struct {
	scanFn func(dest ...any) error
}

func (r *mockRow) Scan(dest ...any) error { return r.scanFn(dest...) }

// --- GetUserByEmail ---

func TestPgStore_GetUserByEmail_Success(t *testing.T) {
	store := &pgStore{db: &mockQuerier{
		row: &mockRow{scanFn: func(dest ...any) error {
			*dest[0].(*string) = "uuid-1"
			*dest[1].(*string) = "user@example.com"
			*dest[2].(*string) = "$2a$10$hash"
			*dest[3].(*string) = "active"
			*dest[4].(*string) = "user"
			// dest[5] is *time.Time (email_verified_at), leave as nil
			// dest[6] is *time.Time (deleted_at), leave as nil
			return nil
		}},
	}}

	record, err := store.GetUserByEmail(t.Context(), "user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record.ID != "uuid-1" {
		t.Errorf("ID = %q, want %q", record.ID, "uuid-1")
	}
}

func TestPgStore_GetUserByEmail_NotFound(t *testing.T) {
	store := &pgStore{db: &mockQuerier{
		row: &mockRow{scanFn: func(_ ...any) error { return pgx.ErrNoRows }},
	}}

	_, err := store.GetUserByEmail(t.Context(), "nobody@example.com")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPgStore_GetUserByEmail_QueryError(t *testing.T) {
	store := &pgStore{db: &mockQuerier{
		row: &mockRow{scanFn: func(_ ...any) error { return errors.New("connection reset") }},
	}}

	_, err := store.GetUserByEmail(t.Context(), "user@example.com")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- CreateUser ---

func TestPgStore_CreateUser_Success(t *testing.T) {
	store := &pgStore{db: &mockQuerier{
		row: &mockRow{scanFn: func(dest ...any) error {
			*dest[0].(*string) = "new-uuid"
			*dest[1].(*string) = "new@example.com"
			*dest[2].(*string) = "$2a$10$hash"
			*dest[3].(*string) = "active"
			*dest[4].(*string) = "user"
			// dest[5] is *time.Time (email_verified_at), leave as nil
			// dest[6] is *time.Time (deleted_at), leave as nil
			return nil
		}},
	}}

	record, err := store.CreateUser(t.Context(), CreateUserInput{Email: "new@example.com", PasswordHash: "$2a$10$hash", AcceptedTermsAt: time.Now(), AcceptedPrivacyAt: time.Now(), AcceptedPolicyVersion: CurrentPolicyVersion})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record.ID != "new-uuid" {
		t.Errorf("ID = %q, want %q", record.ID, "new-uuid")
	}
}

func TestPgStore_CreateUser_EmailTaken(t *testing.T) {
	store := &pgStore{db: &mockQuerier{
		row: &mockRow{scanFn: func(_ ...any) error {
			return &pgconn.PgError{Code: "23505"}
		}},
	}}

	_, err := store.CreateUser(t.Context(), CreateUserInput{Email: "taken@example.com", PasswordHash: "hash", AcceptedTermsAt: time.Now(), AcceptedPrivacyAt: time.Now(), AcceptedPolicyVersion: CurrentPolicyVersion})
	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("got %v, want ErrEmailTaken", err)
	}
}

func TestPgStore_CreateUser_QueryError(t *testing.T) {
	store := &pgStore{db: &mockQuerier{
		row: &mockRow{scanFn: func(_ ...any) error { return errors.New("db error") }},
	}}

	_, err := store.CreateUser(t.Context(), CreateUserInput{Email: "user@example.com", PasswordHash: "hash", AcceptedTermsAt: time.Now(), AcceptedPrivacyAt: time.Now(), AcceptedPolicyVersion: CurrentPolicyVersion})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- GetUserByID ---

func TestPgStore_GetUserByID_Success(t *testing.T) {
	store := &pgStore{db: &mockQuerier{
		row: &mockRow{scanFn: func(dest ...any) error {
			*dest[0].(*string) = "uuid-1"
			*dest[1].(*string) = "user@example.com"
			*dest[2].(*string) = "$2a$10$hash"
			*dest[3].(*string) = "active"
			*dest[4].(*string) = "user"
			// dest[5] is *time.Time (email_verified_at), leave as nil
			// dest[6] is *time.Time (deleted_at), leave as nil
			return nil
		}},
	}}

	record, err := store.GetUserByID(t.Context(), "uuid-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record.ID != "uuid-1" {
		t.Errorf("ID = %q, want %q", record.ID, "uuid-1")
	}
}

func TestPgStore_GetUserByID_NotFound(t *testing.T) {
	store := &pgStore{db: &mockQuerier{
		row: &mockRow{scanFn: func(_ ...any) error { return pgx.ErrNoRows }},
	}}

	_, err := store.GetUserByID(t.Context(), "missing-uuid")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- UpdatePassword ---

func TestPgStore_UpdatePassword_Success(t *testing.T) {
	store := &pgStore{db: &mockQuerier{
		row: &mockRow{scanFn: func(_ ...any) error { return nil }},
	}}

	if err := store.UpdatePassword(t.Context(), "uuid-1", "$2a$10$newhash"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPgStore_UpdatePassword_ExecError(t *testing.T) {
	store := &pgStore{db: &mockQuerier{execErr: errors.New("db error")}}

	if err := store.UpdatePassword(t.Context(), "uuid-1", "hash"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- DeleteUser ---

func TestPgStore_DeleteUser_Success(t *testing.T) {
	store := &pgStore{db: &mockQuerier{
		row: &mockRow{scanFn: func(_ ...any) error { return nil }},
	}}

	if err := store.DeleteUser(t.Context(), "uuid-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPgStore_DeleteUser_ExecError(t *testing.T) {
	store := &pgStore{db: &mockQuerier{execErr: errors.New("db error")}}

	if err := store.DeleteUser(t.Context(), "uuid-1"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- ReactivateUser ---

func TestPgStore_ReactivateUser_Success(t *testing.T) {
	store := &pgStore{db: &mockQuerier{}}
	if err := store.ReactivateUser(t.Context(), "uuid-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPgStore_ReactivateUser_ExecError(t *testing.T) {
	store := &pgStore{db: &mockQuerier{execErr: errors.New("db error")}}
	if err := store.ReactivateUser(t.Context(), "uuid-1"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- GetExpiredDeletedUserIDs ---

func TestPgStore_GetExpiredDeletedUserIDs_Empty(t *testing.T) {
	store := &pgStore{db: &mockQuerier{queryRows: nil}}
	ids, err := store.GetExpiredDeletedUserIDs(t.Context(), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("ids = %v, want empty", ids)
	}
}

func TestPgStore_GetExpiredDeletedUserIDs_SomeRows(t *testing.T) {
	store := &pgStore{db: &mockQuerier{
		queryRows: [][]any{{"uid-1"}, {"uid-2"}},
	}}
	ids, err := store.GetExpiredDeletedUserIDs(t.Context(), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 || ids[0] != "uid-1" || ids[1] != "uid-2" {
		t.Errorf("ids = %v, want [uid-1 uid-2]", ids)
	}
}

func TestPgStore_GetExpiredDeletedUserIDs_QueryError(t *testing.T) {
	store := &pgStore{db: &mockQuerier{queryErr: errors.New("db error")}}
	if _, err := store.GetExpiredDeletedUserIDs(t.Context(), time.Now()); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- GetUserUploadKeys ---

func TestPgStore_GetUserUploadKeys_Empty(t *testing.T) {
	store := &pgStore{db: &mockQuerier{queryRows: nil}}
	sk, tk, err := store.GetUserUploadKeys(t.Context(), "uid-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sk) != 0 || len(tk) != 0 {
		t.Errorf("expected empty slices, got sk=%v tk=%v", sk, tk)
	}
}

func TestPgStore_GetUserUploadKeys_SomeRows(t *testing.T) {
	store := &pgStore{db: &mockQuerier{
		queryRows: [][]any{
			{"uploads/img1.jpg", "uploads/img1_thumb.jpg"},
			{"uploads/img2.jpg", ""},
		},
	}}
	sk, tk, err := store.GetUserUploadKeys(t.Context(), "uid-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sk) != 2 || sk[0] != "uploads/img1.jpg" || sk[1] != "uploads/img2.jpg" {
		t.Errorf("storageKeys = %v, want [uploads/img1.jpg uploads/img2.jpg]", sk)
	}
	if len(tk) != 2 || tk[0] != "uploads/img1_thumb.jpg" || tk[1] != "" {
		t.Errorf("thumbnailKeys = %v, want [uploads/img1_thumb.jpg ]", tk)
	}
}

func TestPgStore_GetUserUploadKeys_QueryError(t *testing.T) {
	store := &pgStore{db: &mockQuerier{queryErr: errors.New("db error")}}
	if _, _, err := store.GetUserUploadKeys(t.Context(), "uid-1"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- DeleteUserData ---

func TestPgStore_DeleteUserData_Success(t *testing.T) {
	store := &pgStore{db: &mockQuerier{}}
	if err := store.DeleteUserData(t.Context(), "uid-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPgStore_DeleteUserData_ExecError(t *testing.T) {
	store := &pgStore{db: &mockQuerier{execErr: errors.New("db error")}}
	if err := store.DeleteUserData(t.Context(), "uid-1"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- AnonymizeUser ---

func TestPgStore_AnonymizeUser_Success(t *testing.T) {
	store := &pgStore{db: &mockQuerier{}}
	if err := store.AnonymizeUser(t.Context(), "uid-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPgStore_AnonymizeUser_ExecError(t *testing.T) {
	store := &pgStore{db: &mockQuerier{execErr: errors.New("db error")}}
	if err := store.AnonymizeUser(t.Context(), "uid-1"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- CreatePasswordReset ---

func TestPgStore_CreatePasswordReset_Success(t *testing.T) {
	store := &pgStore{db: &mockQuerier{}}
	if err := store.CreatePasswordReset(t.Context(), "uid", "hash", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPgStore_CreatePasswordReset_ExecError(t *testing.T) {
	store := &pgStore{db: &mockQuerier{execErr: errors.New("db error")}}
	if err := store.CreatePasswordReset(t.Context(), "uid", "hash", time.Now().Add(time.Hour)); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- GetPasswordReset ---

func TestPgStore_GetPasswordReset_Success(t *testing.T) {
	exp := time.Now().Add(time.Hour)
	store := &pgStore{db: &mockQuerier{
		row: &mockRow{scanFn: func(dest ...any) error {
			*dest[0].(*string) = "pr-id"
			*dest[1].(*string) = "uid"
			*dest[2].(*string) = "h"
			*dest[3].(*time.Time) = exp
			// dest[4] is *time.Time (used_at), leave as nil
			return nil
		}},
	}}
	r, err := store.GetPasswordReset(t.Context(), "h")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ID != "pr-id" {
		t.Errorf("ID = %q, want %q", r.ID, "pr-id")
	}
}

func TestPgStore_GetPasswordReset_NotFound(t *testing.T) {
	store := &pgStore{db: &mockQuerier{
		row: &mockRow{scanFn: func(_ ...any) error { return pgx.ErrNoRows }},
	}}
	if _, err := store.GetPasswordReset(t.Context(), "nope"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- MarkPasswordResetUsed ---

func TestPgStore_MarkPasswordResetUsed_Success(t *testing.T) {
	store := &pgStore{db: &mockQuerier{}}
	if err := store.MarkPasswordResetUsed(t.Context(), "pr-id"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPgStore_MarkPasswordResetUsed_ExecError(t *testing.T) {
	store := &pgStore{db: &mockQuerier{execErr: errors.New("db error")}}
	if err := store.MarkPasswordResetUsed(t.Context(), "pr-id"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- CreateEmailVerification ---

func TestPgStore_CreateEmailVerification_Success(t *testing.T) {
	store := &pgStore{db: &mockQuerier{}}
	if err := store.CreateEmailVerification(t.Context(), "uid", "hash", time.Now().Add(24*time.Hour)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPgStore_CreateEmailVerification_ExecError(t *testing.T) {
	store := &pgStore{db: &mockQuerier{execErr: errors.New("db error")}}
	if err := store.CreateEmailVerification(t.Context(), "uid", "hash", time.Now().Add(time.Hour)); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- GetEmailVerification ---

func TestPgStore_GetEmailVerification_Success(t *testing.T) {
	exp := time.Now().Add(24 * time.Hour)
	store := &pgStore{db: &mockQuerier{
		row: &mockRow{scanFn: func(dest ...any) error {
			*dest[0].(*string) = "ev-id"
			*dest[1].(*string) = "uid"
			*dest[2].(*string) = "h"
			*dest[3].(*time.Time) = exp
			// dest[4] is *time.Time (verified_at), leave as nil
			return nil
		}},
	}}
	r, err := store.GetEmailVerification(t.Context(), "h")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ID != "ev-id" {
		t.Errorf("ID = %q, want %q", r.ID, "ev-id")
	}
}

func TestPgStore_GetEmailVerification_NotFound(t *testing.T) {
	store := &pgStore{db: &mockQuerier{
		row: &mockRow{scanFn: func(_ ...any) error { return pgx.ErrNoRows }},
	}}
	if _, err := store.GetEmailVerification(t.Context(), "nope"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- MarkEmailVerified ---

func TestPgStore_MarkEmailVerified_Success(t *testing.T) {
	store := &pgStore{db: &mockQuerier{}}
	if err := store.MarkEmailVerified(t.Context(), "uid", "ev-id"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPgStore_MarkEmailVerified_ExecError(t *testing.T) {
	store := &pgStore{db: &mockQuerier{execErr: errors.New("db error")}}
	if err := store.MarkEmailVerified(t.Context(), "uid", "ev-id"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- NewStore ---

func TestNewStore(t *testing.T) {
	store := NewStore((*pgxpool.Pool)(nil))
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

// --- Integration ---

func TestStore_Integration(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("connect to db: %v", err)
	}
	defer pool.Close()

	store := NewStore(pool)
	svc := NewService(store, &noopMailer{}, "")

	const email = "store_integration@example.com"
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE email = $1`, email)
	})

	t.Run("register new user", func(t *testing.T) {
		user, err := svc.Register(t.Context(), RegistrationInput{Email: email, Password: "Secure1pass", AcceptedPolicyVersion: CurrentPolicyVersion})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Email != email {
			t.Errorf("email = %q, want %q", user.Email, email)
		}
	})

	t.Run("login before verification returns ErrEmailNotVerified", func(t *testing.T) {
		_, err := svc.Login(t.Context(), email, "Secure1pass")
		if !errors.Is(err, ErrEmailNotVerified) {
			t.Errorf("got %v, want ErrEmailNotVerified", err)
		}
	})

	t.Run("login after manual verification succeeds", func(t *testing.T) {
		// Simulate email verification by setting email_verified_at directly.
		_, err := pool.Exec(context.Background(),
			`UPDATE users SET email_verified_at = now() WHERE email = $1`, email)
		if err != nil {
			t.Fatalf("manual verify: %v", err)
		}
		user, err := svc.Login(t.Context(), email, "Secure1pass")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Email != email {
			t.Errorf("email = %q, want %q", user.Email, email)
		}
	})

	t.Run("register duplicate email returns ErrEmailTaken", func(t *testing.T) {
		_, err := svc.Register(t.Context(), RegistrationInput{Email: email, Password: "Other1pass", AcceptedPolicyVersion: CurrentPolicyVersion})
		if !errors.Is(err, ErrEmailTaken) {
			t.Errorf("got %v, want ErrEmailTaken", err)
		}
	})
}
