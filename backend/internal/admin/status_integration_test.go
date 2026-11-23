package admin

import (
	"errors"
	"testing"

	"github.com/mayloo89/circl/backend/internal/testutil"
)

func TestSetUserStatus_NeverRevivesPurgedAccount_Integration(t *testing.T) {
	pool := testutil.OpenDB(t)
	store := NewStore(pool)
	userID := testutil.CreateUser(t, pool, "status_guard@admin.example.com")

	if err := store.SetUserStatus(t.Context(), userID, "suspended"); err != nil {
		t.Fatalf("suspend live account: %v", err)
	}
	if _, err := pool.Exec(t.Context(), `UPDATE users SET status = 'purged' WHERE id = $1`, userID); err != nil {
		t.Fatalf("purge: %v", err)
	}
	for _, status := range []string{"active", "suspended", "banned"} {
		if err := store.SetUserStatus(t.Context(), userID, status); !errors.Is(err, ErrUserPurged) {
			t.Errorf("SetUserStatus(%q) on purged account: error = %v, want ErrUserPurged", status, err)
		}
	}
	var got string
	if err := pool.QueryRow(t.Context(), `SELECT status FROM users WHERE id = $1`, userID).Scan(&got); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if got != "purged" {
		t.Errorf("status = %q, want purged", got)
	}
	if err := store.SetUserStatus(t.Context(), "00000000-0000-0000-0000-000000000000", "banned"); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("missing account: error = %v, want ErrUserNotFound", err)
	}
}
