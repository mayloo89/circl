package evidence

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mayloo89/circl/backend/internal/reports"
	"github.com/mayloo89/circl/backend/internal/storage"
	"github.com/mayloo89/circl/backend/internal/testutil"
)

const testURLPrefix = "http://x/uploads/files/"

type fixture struct {
	pool              *pgxpool.Pool
	disk              *storage.LocalStorage
	dir               string
	reporter, sender  string
	roomID, messageID string
}

func newFixture(t *testing.T, prefix string) fixture {
	t.Helper()
	pool := testutil.OpenDB(t)
	dir := t.TempDir()
	f := fixture{
		pool:     pool,
		dir:      dir,
		disk:     storage.NewLocalStorage(dir, "http://x/uploads/files"),
		reporter: testutil.CreateUser(t, pool, prefix+"_reporter@evidence-files.example.com"),
		sender:   testutil.CreateUser(t, pool, prefix+"_sender@evidence-files.example.com"),
	}
	ctx := t.Context()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(pool.QueryRow(ctx, `INSERT INTO rooms (type) VALUES ('dm') RETURNING id`).Scan(&f.roomID))
	_, err := pool.Exec(ctx, `INSERT INTO room_members (room_id, user_id) VALUES ($1, $2), ($1, $3)`, f.roomID, f.reporter, f.sender)
	must(err)
	must(pool.QueryRow(ctx, `INSERT INTO messages (room_id, sender_id, content) VALUES ($1, $2, 'hi') RETURNING id`,
		f.roomID, f.sender).Scan(&f.messageID))
	t.Cleanup(func() {
		ctx := context.Background()
		pool.Exec(ctx, `DELETE FROM reports WHERE reported_user_id = $1`, f.sender) //nolint:errcheck
		pool.Exec(ctx, `DELETE FROM uploads WHERE user_id = $1`, f.sender)          //nolint:errcheck
		pool.Exec(ctx, `DELETE FROM rooms WHERE id = $1`, f.roomID)                 //nolint:errcheck
	})
	return f
}

// upload stores an object on disk and its uploads row, returning the row ID.
func (f fixture) upload(t *testing.T, key, category, status string, messageID *string, body string) string {
	t.Helper()
	p := filepath.Join(f.dir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := f.pool.QueryRow(t.Context(), `
		INSERT INTO uploads (user_id, storage_key, filename, content_type, size_bytes, category, status, message_id)
		VALUES ($1, $2, 'f.jpg', 'image/jpeg', $3, $4, $5, $6) RETURNING id`,
		f.sender, key, len(body), category, status, messageID).Scan(&id); err != nil {
		t.Fatalf("insert upload %s: %v", key, err)
	}
	return id
}

func (f fixture) report(t *testing.T, reason string, messageID *string) *reports.Report {
	t.Helper()
	r := &reports.Report{ReportedUserID: &f.sender, MessageID: messageID, Reason: reason, Priority: reports.PriorityFor(reason)}
	if err := f.pool.QueryRow(t.Context(), `
		INSERT INTO reports (reporter_id, reported_user_id, reason, priority, message_id, message_content)
		VALUES ($1, $2, $3, $4, $5, CASE WHEN $5::uuid IS NULL THEN NULL ELSE 'hi' END) RETURNING id`,
		f.reporter, f.sender, reason, r.Priority, messageID).Scan(&r.ID); err != nil {
		t.Fatalf("insert report: %v", err)
	}
	return r
}

func TestEvidence_Integration(t *testing.T) {
	f := newFixture(t, "evfiles")
	ctx := t.Context()
	svc := NewService(Config{Store: NewStore(f.pool), Storage: f.disk, URLPrefix: testURLPrefix, RetentionDays: 30})

	// Message media: one committed attachment, one quarantined (read from its
	// quarantine copy), one still pending (ignored).
	f.upload(t, "chat/"+f.sender+"/ok.jpg", "chat-attachment", "committed", &f.messageID, "ok-bytes")
	qid := f.upload(t, "quarantine/chat/"+f.sender+"/bad.jpg", "chat-attachment", "failed", &f.messageID, "bad-bytes")
	if _, err := f.pool.Exec(ctx, `UPDATE uploads SET storage_key = $2, moderation_status = 'quarantined', moderation_quarantine_key = $3 WHERE id = $1`,
		qid, "chat/"+f.sender+"/bad.jpg", "quarantine/chat/"+f.sender+"/bad.jpg"); err != nil {
		t.Fatalf("quarantine upload: %v", err)
	}
	f.upload(t, "chat/"+f.sender+"/pending.jpg", "chat-attachment", "pending", &f.messageID, "pending")

	// Profile: the shown avatar and one gallery photo count; a replaced avatar does not.
	f.upload(t, "avatars/"+f.sender+"/now.jpg", "avatar", "committed", nil, "avatar")
	f.upload(t, "avatars/"+f.sender+"/old.jpg", "avatar", "committed", nil, "old-avatar")
	f.upload(t, "gallery/"+f.sender+"/g1.jpg", "gallery", "committed", nil, "gallery")
	if _, err := f.pool.Exec(ctx, `INSERT INTO profiles (user_id, username, display_name, avatar_url) VALUES ($1, 'evfiles_sender', 'Sender', $2)
		ON CONFLICT (user_id) DO UPDATE SET avatar_url = EXCLUDED.avatar_url`, f.sender, testURLPrefix+"avatars/"+f.sender+"/now.jpg"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO profile_photos (user_id, url) VALUES ($1, $2)`, f.sender, testURLPrefix+"gallery/"+f.sender+"/g1.jpg"); err != nil {
		t.Fatal(err)
	}

	t.Run("normal message report copies only the message's media", func(t *testing.T) {
		r := f.report(t, reports.ReasonHarassment, &f.messageID)
		if err := svc.Preserve(ctx, r); err != nil {
			t.Fatalf("Preserve() error = %v", err)
		}
		files, err := svc.List(ctx, r.ID)
		if err != nil || len(files) != 2 {
			t.Fatalf("List() = %d files, %v; want 2", len(files), err)
		}
		got := map[string]bool{}
		for _, file := range files {
			if file.Kind != KindMessageMedia || !storage.IsRestrictedKey(file.StorageKey) {
				t.Errorf("file %+v", file)
			}
			_, rc, err := svc.Open(ctx, r.ID, file.ID)
			if err != nil {
				t.Fatal(err)
			}
			b, _ := io.ReadAll(rc)
			_ = rc.Close()
			got[string(b)] = true
		}
		if !got["ok-bytes"] || !got["bad-bytes"] {
			t.Errorf("copied contents = %v, want the committed and the quarantined attachment", got)
		}

		// Dismissed reports are swept at once, whatever the cutoff.
		if _, err := f.pool.Exec(ctx, `UPDATE reports SET status = 'dismissed', reviewed_at = NOW() WHERE id = $1`, r.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Sweep(ctx, 100); err != nil {
			t.Fatalf("Sweep() error = %v", err)
		}
		if files, _ := svc.List(ctx, r.ID); len(files) != 0 {
			t.Errorf("%d files left after sweeping a dismissed report", len(files))
		}
		for _, file := range files {
			if _, err := os.Stat(f.disk.FilePath(file.StorageKey)); !os.IsNotExist(err) {
				t.Errorf("object %s still on disk", file.StorageKey)
			}
		}
		var content *string
		var purgedAt *time.Time
		if err := f.pool.QueryRow(ctx, `SELECT message_content, evidence_purged_at FROM reports WHERE id = $1`, r.ID).Scan(&content, &purgedAt); err != nil {
			t.Fatal(err)
		}
		if content != nil || purgedAt == nil {
			t.Errorf("message_content = %v, evidence_purged_at = %v; want wiped and stamped", content, purgedAt)
		}
	})

	t.Run("critical profile report snapshots shown photos and is never viewable", func(t *testing.T) {
		r := f.report(t, reports.ReasonCSAM, nil)
		if err := svc.Preserve(ctx, r); err != nil {
			t.Fatalf("Preserve() error = %v", err)
		}
		files, err := svc.List(ctx, r.ID)
		if err != nil || len(files) != 2 {
			t.Fatalf("List() = %d files, %v; want avatar + gallery", len(files), err)
		}
		for _, file := range files {
			if file.Kind != KindProfilePhoto || file.SHA256 == "" {
				t.Errorf("file %+v", file)
			}
			got, rc, err := svc.Open(ctx, r.ID, file.ID)
			if err != nil || rc != nil || got.Viewable() {
				t.Errorf("Open(csam) streamed bytes: rc=%v err=%v", rc, err)
			}
		}
		if _, _, err := svc.Open(ctx, r.ID, "00000000-0000-0000-0000-000000000000"); err != ErrNotFound {
			t.Errorf("unknown file: %v, want ErrNotFound", err)
		}

		// Reviewed reports keep evidence until the retention period passes.
		if _, err := f.pool.Exec(ctx, `UPDATE reports SET status = 'reviewed', reviewed_at = NOW() - INTERVAL '10 days' WHERE id = $1`, r.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Sweep(ctx, 100); err != nil {
			t.Fatal(err)
		}
		if files, _ := svc.List(ctx, r.ID); len(files) != 2 {
			t.Fatalf("evidence swept %d days before retention ended", 20)
		}
		if _, err := f.pool.Exec(ctx, `UPDATE reports SET reviewed_at = NOW() - INTERVAL '31 days' WHERE id = $1`, r.ID); err != nil {
			t.Fatal(err)
		}
		if res, err := svc.Sweep(ctx, 100); err != nil || res.Purged < 1 {
			t.Fatalf("Sweep() = %+v, %v", res, err)
		}
		if files, _ := svc.List(ctx, r.ID); len(files) != 0 {
			t.Errorf("%d files left after retention", len(files))
		}
	})

	t.Run("deleting a report cascades to its rows", func(t *testing.T) {
		r := f.report(t, reports.ReasonNCII, nil)
		if err := svc.Preserve(ctx, r); err != nil {
			t.Fatal(err)
		}
		if files, _ := svc.List(ctx, r.ID); len(files) == 0 {
			t.Fatal("no evidence preserved to cascade")
		}
		if _, err := f.pool.Exec(ctx, `DELETE FROM reports WHERE id = $1`, r.ID); err != nil {
			t.Fatal(err)
		}
		var n int
		if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM report_evidence_files WHERE report_id = $1`, r.ID).Scan(&n); err != nil || n != 0 {
			t.Errorf("rows left = %d, %v", n, err)
		}
	})
}
