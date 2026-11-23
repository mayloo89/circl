package reports

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mayloo89/circl/backend/internal/testutil"
)

// evidenceFixture is a DM between reporter and sender holding one message, plus
// an outsider who is not in the room.
type evidenceFixture struct {
	pool                       *pgxpool.Pool
	store                      Store
	reporter, sender, outsider string
	roomID, messageID          string
}

func newEvidenceFixture(t *testing.T, prefix string) evidenceFixture {
	t.Helper()
	pool := testutil.OpenDB(t)
	f := evidenceFixture{
		pool:     pool,
		store:    NewStore(pool),
		reporter: testutil.CreateUser(t, pool, prefix+"_reporter@evidence.example.com"),
		sender:   testutil.CreateUser(t, pool, prefix+"_sender@evidence.example.com"),
		outsider: testutil.CreateUser(t, pool, prefix+"_outsider@evidence.example.com"),
	}
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO profiles (user_id, username, display_name) VALUES ($1, $2, 'Sender Name')
		 ON CONFLICT (user_id) DO UPDATE SET username = EXCLUDED.username, display_name = EXCLUDED.display_name`,
		f.sender, prefix+"_sender",
	); err != nil {
		t.Fatalf("seed sender profile: %v", err)
	}
	if err := pool.QueryRow(t.Context(), `INSERT INTO rooms (type) VALUES ('dm') RETURNING id`).Scan(&f.roomID); err != nil {
		t.Fatalf("create room: %v", err)
	}
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO room_members (room_id, user_id) VALUES ($1, $2), ($1, $3)`, f.roomID, f.reporter, f.sender,
	); err != nil {
		t.Fatalf("add members: %v", err)
	}
	if err := pool.QueryRow(t.Context(),
		`INSERT INTO messages (room_id, sender_id, content) VALUES ($1, $2, 'the reported words') RETURNING id`,
		f.roomID, f.sender,
	).Scan(&f.messageID); err != nil {
		t.Fatalf("create message: %v", err)
	}
	// Rooms and messages are cleaned before the users that testutil removes.
	t.Cleanup(func() {
		ctx := context.Background()
		pool.Exec(ctx, `DELETE FROM reports WHERE message_room_id = $1 OR reported_user_id = $2`, f.roomID, f.sender) //nolint:errcheck
		pool.Exec(ctx, `DELETE FROM rooms WHERE id = $1`, f.roomID)                                                   //nolint:errcheck
	})
	return f
}

func (f evidenceFixture) listed(t *testing.T, reportID string) ReportWithUserInfo {
	t.Helper()
	all, err := f.store.List(t.Context(), ListFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, r := range all {
		if r.ID == reportID {
			return r
		}
	}
	t.Fatalf("report %s not listed", reportID)
	return ReportWithUserInfo{}
}

func TestReportEvidence_Integration(t *testing.T) {
	t.Run("message report derives the sender and copies the message", func(t *testing.T) {
		f := newEvidenceFixture(t, "copy")
		r, err := f.store.CreateForMessage(t.Context(), f.reporter, f.messageID, ReasonHarassment, PriorityNormal, "")
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if gone, err := f.store.ReportedAccountGone(t.Context(), r.ID); err != nil || gone {
			t.Errorf("ReportedAccountGone() = %v, %v; want false for a live account", gone, err)
		}
		if _, err := f.store.ReportedAccountGone(t.Context(), "00000000-0000-0000-0000-000000000000"); !errors.Is(err, ErrNotFound) {
			t.Errorf("missing report: error = %v, want ErrNotFound", err)
		}
		if r.ReportedUserID == nil || *r.ReportedUserID != f.sender {
			t.Errorf("reported user = %v, want the sender %s", r.ReportedUserID, f.sender)
		}
		got := f.listed(t, r.ID)
		if got.Message == nil || got.Message.Content == nil || *got.Message.Content != "the reported words" || got.Message.RoomID != f.roomID {
			t.Errorf("message snapshot = %+v", got.Message)
		}
	})

	t.Run("message copy survives the original being deleted", func(t *testing.T) {
		f := newEvidenceFixture(t, "unsent")
		r, err := f.store.CreateForMessage(t.Context(), f.reporter, f.messageID, ReasonHarassment, PriorityNormal, "")
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := f.pool.Exec(t.Context(), `DELETE FROM messages WHERE id = $1`, f.messageID); err != nil {
			t.Fatalf("delete message: %v", err)
		}
		if got := f.listed(t, r.ID); got.Message == nil || got.Message.Content == nil || *got.Message.Content != "the reported words" {
			t.Errorf("message snapshot after deletion = %+v", got.Message)
		}
	})

	t.Run("identity snapshot survives account purge", func(t *testing.T) {
		f := newEvidenceFixture(t, "purged")
		r, err := f.store.CreateForMessage(t.Context(), f.reporter, f.messageID, ReasonHarassment, PriorityNormal, "")
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		// What the purge worker does: drop the profile, scrub the users row.
		if _, err := f.pool.Exec(t.Context(), `DELETE FROM profiles WHERE user_id = $1`, f.sender); err != nil {
			t.Fatalf("delete profile: %v", err)
		}
		if _, err := f.pool.Exec(t.Context(),
			`UPDATE users SET email = 'deleted-' || id || '@purged', status = 'purged' WHERE id = $1`, f.sender,
		); err != nil {
			t.Fatalf("anonymize: %v", err)
		}
		if gone, err := f.store.ReportedAccountGone(t.Context(), r.ID); err != nil || !gone {
			t.Errorf("ReportedAccountGone() = %v, %v; want true for a purged account", gone, err)
		}
		got := f.listed(t, r.ID)
		if !got.ReportedAccountGone || got.ReportedUsername != "purged_sender" || got.ReportedName != "Sender Name" ||
			got.ReportedEmail != "purged_sender@evidence.example.com" {
			t.Errorf("purged account listing = %+v, want the snapshot identity", got)
		}
	})

	t.Run("report survives a hard delete of the reported account", func(t *testing.T) {
		f := newEvidenceFixture(t, "hard")
		r, err := f.store.CreateForMessage(t.Context(), f.reporter, f.messageID, ReasonHarassment, PriorityNormal, "")
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := f.pool.Exec(t.Context(), `DELETE FROM messages WHERE sender_id = $1`, f.sender); err != nil {
			t.Fatalf("delete messages: %v", err)
		}
		if _, err := f.pool.Exec(t.Context(), `DELETE FROM users WHERE id = $1`, f.sender); err != nil {
			t.Fatalf("hard delete: %v", err)
		}
		if gone, err := f.store.ReportedAccountGone(t.Context(), r.ID); err != nil || !gone {
			t.Errorf("ReportedAccountGone() = %v, %v; want true after a hard delete", gone, err)
		}
		got := f.listed(t, r.ID)
		if got.ReportedUserID != nil || !got.ReportedAccountGone || got.ReportedUsername != "hard_sender" {
			t.Errorf("listing after hard delete = %+v", got)
		}
	})

	t.Run("dismissal purges the message copy, review keeps it", func(t *testing.T) {
		f := newEvidenceFixture(t, "dismiss")
		r, err := f.store.CreateForMessage(t.Context(), f.reporter, f.messageID, ReasonSpam, PriorityNormal, "")
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := f.store.UpdateStatus(t.Context(), r.ID, StatusReviewed, f.reporter); err != nil {
			t.Fatalf("review: %v", err)
		}
		if got := f.listed(t, r.ID); got.Message.Content == nil || got.Message.PurgedAt != nil {
			t.Errorf("reviewed report lost its evidence: %+v", got.Message)
		}
		if _, err := f.store.UpdateStatus(t.Context(), r.ID, StatusDismissed, f.reporter); err != nil {
			t.Fatalf("dismiss: %v", err)
		}
		if got := f.listed(t, r.ID); got.Message.Content != nil || got.Message.PurgedAt == nil {
			t.Errorf("dismissed report kept its evidence: %+v", got.Message)
		}
	})

	t.Run("rejects reports the reporter is not entitled to file", func(t *testing.T) {
		f := newEvidenceFixture(t, "deny")
		cases := []struct {
			name     string
			reporter string
			message  string
			want     error
		}{
			{"outsider", f.outsider, f.messageID, ErrMessageNotFound},
			{"own message", f.sender, f.messageID, ErrSelfReport},
			{"malformed id", f.reporter, "not-a-uuid", ErrMessageNotFound},
		}
		for _, tc := range cases {
			if _, err := f.store.CreateForMessage(t.Context(), tc.reporter, tc.message, ReasonSpam, PriorityNormal, ""); !errors.Is(err, tc.want) {
				t.Errorf("%s: error = %v, want %v", tc.name, err, tc.want)
			}
		}
		if _, err := f.pool.Exec(t.Context(), `UPDATE messages SET tombstone = true, content = '' WHERE id = $1`, f.messageID); err != nil {
			t.Fatalf("tombstone: %v", err)
		}
		if _, err := f.store.CreateForMessage(t.Context(), f.reporter, f.messageID, ReasonSpam, PriorityNormal, ""); !errors.Is(err, ErrMessageNotFound) {
			t.Errorf("tombstoned message: error = %v, want ErrMessageNotFound", err)
		}
	})

	t.Run("one report per reporter per message", func(t *testing.T) {
		f := newEvidenceFixture(t, "dup")
		if _, err := f.store.CreateForMessage(t.Context(), f.reporter, f.messageID, ReasonSpam, PriorityNormal, ""); err != nil {
			t.Fatalf("first report: %v", err)
		}
		if _, err := f.store.CreateForMessage(t.Context(), f.reporter, f.messageID, ReasonHarassment, PriorityNormal, ""); !errors.Is(err, ErrDuplicateReport) {
			t.Errorf("second report error = %v, want ErrDuplicateReport", err)
		}
	})

	t.Run("user report snapshots identity", func(t *testing.T) {
		f := newEvidenceFixture(t, "user")
		r, err := f.store.Create(t.Context(), f.reporter, f.sender, ReasonFakeProfile, PriorityNormal, "")
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := f.pool.Exec(t.Context(), `DELETE FROM profiles WHERE user_id = $1`, f.sender); err != nil {
			t.Fatalf("delete profile: %v", err)
		}
		if got := f.listed(t, r.ID); got.ReportedUsername != "user_sender" || got.ReportedName != "Sender Name" || got.Message != nil {
			t.Errorf("listing = %+v, want the snapshot identity and no message", got)
		}
		if _, err := f.store.Create(t.Context(), f.reporter, "00000000-0000-0000-0000-000000000000", ReasonSpam, PriorityNormal, ""); !errors.Is(err, ErrUserNotFound) {
			t.Errorf("unknown user error = %v, want ErrUserNotFound", err)
		}
	})
}
