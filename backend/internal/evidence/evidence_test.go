package evidence

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/mayloo89/circl/backend/internal/reports"
	"github.com/mayloo89/circl/backend/internal/storage"
)

// memStorage is an in-memory ObjectStorage.
type memStorage struct {
	objects   map[string][]byte
	getErr    map[string]error
	putErr    error
	deleteErr error
}

func newMemStorage(objects map[string]string) *memStorage {
	m := &memStorage{objects: map[string][]byte{}, getErr: map[string]error{}}
	for k, v := range objects {
		m.objects[k] = []byte(v)
	}
	return m
}

func (m *memStorage) GetObject(_ context.Context, key string) (io.ReadCloser, error) {
	if err := m.getErr[key]; err != nil {
		return nil, err
	}
	b, ok := m.objects[key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (m *memStorage) PutObject(_ context.Context, key, _ string, r io.Reader, _ int64) error {
	if m.putErr != nil {
		return m.putErr
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.objects[key] = b
	return nil
}

func (m *memStorage) Delete(_ context.Context, key string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.objects, key)
	return nil
}

// memStore is an in-memory Store.
type memStore struct {
	media, photos     []Source
	mediaErr          error
	photosErr         error
	recordErr         error
	files             []File
	due               []string
	dueErr            error
	markErr           error
	purged            []string
	gotPrefix, gotCut string
}

func (m *memStore) MessageMedia(context.Context, string) ([]Source, error) {
	return m.media, m.mediaErr
}

func (m *memStore) ProfilePhotos(_ context.Context, _, prefix string) ([]Source, error) {
	m.gotPrefix = prefix
	return m.photos, m.photosErr
}

func (m *memStore) Record(_ context.Context, f File) error {
	if m.recordErr != nil {
		return m.recordErr
	}
	m.files = append(m.files, f)
	return nil
}

func (m *memStore) List(_ context.Context, reportID string) ([]File, error) {
	var out []File
	for _, f := range m.files {
		if f.ReportID == reportID {
			out = append(out, f)
		}
	}
	return out, nil
}

func (m *memStore) Get(_ context.Context, reportID, fileID string) (*File, error) {
	for _, f := range m.files {
		if f.ReportID == reportID && f.ID == fileID {
			return &f, nil
		}
	}
	return nil, ErrNotFound
}

func (m *memStore) Delete(_ context.Context, fileID string) error {
	for i, f := range m.files {
		if f.ID == fileID {
			m.files = append(m.files[:i], m.files[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *memStore) DueForPurge(_ context.Context, cutoff time.Time, _ int) ([]string, error) {
	m.gotCut = cutoff.Format(time.DateOnly)
	return m.due, m.dueErr
}

func (m *memStore) MarkPurged(_ context.Context, reportID string) error {
	if m.markErr != nil {
		return m.markErr
	}
	m.purged = append(m.purged, reportID)
	return nil
}

func sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func messageReport(priority string) *reports.Report {
	return &reports.Report{ID: "rep-1", MessageID: new("m-1"), ReportedUserID: new("u-2"), Priority: priority}
}

func TestPreserve_CopiesMessageMediaUnderEvidencePrefix(t *testing.T) {
	store := &memStore{media: []Source{{UploadID: "up-1", Kind: KindMessageMedia, Key: "chat/u-2/a.jpg", ContentType: "image/jpeg", SizeBytes: 99}}}
	st := newMemStorage(map[string]string{"chat/u-2/a.jpg": "jpeg-bytes"})
	svc := NewService(Config{Store: store, Storage: st, URLPrefix: "https://cdn/"})

	if err := svc.Preserve(t.Context(), messageReport(reports.PriorityNormal)); err != nil {
		t.Fatalf("Preserve() error = %v", err)
	}
	if len(store.files) != 1 {
		t.Fatalf("recorded %d files, want 1", len(store.files))
	}
	f := store.files[0]
	if !strings.HasPrefix(f.StorageKey, "evidence/rep-1/") || !strings.HasSuffix(f.StorageKey, ".jpg") {
		t.Errorf("storage key = %q, want evidence/rep-1/<id>.jpg", f.StorageKey)
	}
	if !storage.IsRestrictedKey(f.StorageKey) {
		t.Errorf("copy %q is not under a never-served prefix", f.StorageKey)
	}
	if string(st.objects[f.StorageKey]) != "jpeg-bytes" {
		t.Errorf("copied bytes = %q", st.objects[f.StorageKey])
	}
	// The size is what was copied, not the upload row's declared 99.
	if f.SHA256 != sum("jpeg-bytes") || f.SizeBytes != int64(len("jpeg-bytes")) {
		t.Errorf("hash/size = %s/%d, want the copied bytes", f.SHA256, f.SizeBytes)
	}
	if f.SourceUploadID != "up-1" || f.Kind != KindMessageMedia || f.ContentType != "image/jpeg" {
		t.Errorf("file metadata = %+v", f)
	}
	if store.gotPrefix != "" {
		t.Error("a normal-priority report must not snapshot profile photos")
	}
}

func TestPreserve_CriticalReportsAlsoSnapshotProfilePhotos(t *testing.T) {
	store := &memStore{photos: []Source{{UploadID: "up-a", Kind: KindProfilePhoto, Key: "avatars/u-2/a.png", ContentType: "image/png"}}}
	st := newMemStorage(map[string]string{"avatars/u-2/a.png": "png"})
	svc := NewService(Config{Store: store, Storage: st, URLPrefix: "https://cdn/"})

	r := &reports.Report{ID: "rep-2", ReportedUserID: new("u-2"), Priority: reports.PriorityCritical}
	if err := svc.Preserve(t.Context(), r); err != nil {
		t.Fatalf("Preserve() error = %v", err)
	}
	if store.gotPrefix != "https://cdn/" {
		t.Errorf("profile photos looked up with prefix %q", store.gotPrefix)
	}
	if len(store.files) != 1 || store.files[0].Kind != KindProfilePhoto {
		t.Errorf("files = %+v, want one profile photo", store.files)
	}
}

func TestPreserve_SkipsPhotosWhenTheAccountIsGone(t *testing.T) {
	store := &memStore{}
	svc := NewService(Config{Store: store, Storage: newMemStorage(nil)})
	if err := svc.Preserve(t.Context(), &reports.Report{ID: "r", Priority: reports.PriorityCritical}); err != nil {
		t.Fatalf("Preserve() error = %v", err)
	}
	if store.gotPrefix != "" {
		t.Error("looked up photos for a report without a reported user")
	}
}

func TestPreserve_OneFailureDoesNotStopTheRest(t *testing.T) {
	store := &memStore{media: []Source{
		{UploadID: "gone", Key: "chat/gone.jpg"},
		{UploadID: "ok", Key: "chat/ok.jpg"},
	}}
	st := newMemStorage(map[string]string{"chat/ok.jpg": "ok"})
	svc := NewService(Config{Store: store, Storage: st})

	err := svc.Preserve(t.Context(), messageReport(reports.PriorityNormal))
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("error = %v, want the missing original reported", err)
	}
	if len(store.files) != 1 || store.files[0].SourceUploadID != "ok" {
		t.Errorf("files = %+v, want the surviving copy recorded", store.files)
	}
}

func TestPreserve_ReportsLookupErrors(t *testing.T) {
	boom := errors.New("db down")
	store := &memStore{mediaErr: boom, photosErr: boom}
	svc := NewService(Config{Store: store, Storage: newMemStorage(nil)})
	if err := svc.Preserve(t.Context(), messageReport(reports.PriorityCritical)); !errors.Is(err, boom) {
		t.Errorf("error = %v, want %v", err, boom)
	}
}

func TestPreserve_WriteFailureRecordsNothing(t *testing.T) {
	store := &memStore{media: []Source{{Key: "chat/a.jpg"}}}
	st := newMemStorage(map[string]string{"chat/a.jpg": "x"})
	st.putErr = errors.New("disk full")
	svc := NewService(Config{Store: store, Storage: st})

	if err := svc.Preserve(t.Context(), messageReport(reports.PriorityNormal)); !errors.Is(err, st.putErr) {
		t.Errorf("error = %v, want the write failure", err)
	}
	if len(store.files) != 0 {
		t.Errorf("recorded %d files after a failed write", len(store.files))
	}
}

func TestPreserve_RecordFailureRemovesTheOrphanCopy(t *testing.T) {
	store := &memStore{media: []Source{{Key: "chat/a.jpg"}}, recordErr: errors.New("insert failed")}
	st := newMemStorage(map[string]string{"chat/a.jpg": "x"})
	svc := NewService(Config{Store: store, Storage: st})

	if err := svc.Preserve(t.Context(), messageReport(reports.PriorityNormal)); !errors.Is(err, store.recordErr) {
		t.Errorf("error = %v, want the record failure", err)
	}
	for key := range st.objects {
		if storage.IsRestrictedKey(key) {
			t.Errorf("orphan copy %q left behind", key)
		}
	}
}

func TestDiscard_DeletesObjectsThenRows(t *testing.T) {
	store := &memStore{files: []File{
		{ID: "f1", ReportID: "rep-1", StorageKey: "evidence/rep-1/f1.jpg"},
		{ID: "f2", ReportID: "rep-1", StorageKey: "evidence/rep-1/f2.jpg"},
		{ID: "f3", ReportID: "other", StorageKey: "evidence/other/f3.jpg"},
	}}
	// f2's object is already gone; that must not block removing its row.
	st := newMemStorage(map[string]string{"evidence/rep-1/f1.jpg": "a", "evidence/other/f3.jpg": "c"})
	svc := NewService(Config{Store: store, Storage: st})

	if err := svc.Discard(t.Context(), "rep-1"); err != nil {
		t.Fatalf("Discard() error = %v", err)
	}
	if len(store.files) != 1 || store.files[0].ID != "f3" {
		t.Errorf("remaining rows = %+v, want only the other report's", store.files)
	}
	if _, ok := st.objects["evidence/rep-1/f1.jpg"]; ok {
		t.Error("object f1 not deleted")
	}
}

func TestDiscard_KeepsTheRowWhenTheObjectCannotBeDeleted(t *testing.T) {
	store := &memStore{files: []File{{ID: "f1", ReportID: "rep-1", StorageKey: "evidence/rep-1/f1.jpg"}}}
	st := newMemStorage(map[string]string{"evidence/rep-1/f1.jpg": "a"})
	st.deleteErr = errors.New("storage offline")
	svc := NewService(Config{Store: store, Storage: st})

	if err := svc.Discard(t.Context(), "rep-1"); !errors.Is(err, st.deleteErr) {
		t.Errorf("error = %v, want the delete failure", err)
	}
	if len(store.files) != 1 {
		t.Error("row removed although its object is still stored; the sweep could never retry it")
	}
}

func TestOpen_RefusesCSAMAndStreamsTheRest(t *testing.T) {
	store := &memStore{files: []File{
		{ID: "f1", ReportID: "r1", StorageKey: "evidence/r1/f1.jpg", ReportReason: reports.ReasonHarassment},
		{ID: "f2", ReportID: "r2", StorageKey: "evidence/r2/f2.jpg", ReportReason: reports.ReasonCSAM},
	}}
	st := newMemStorage(map[string]string{"evidence/r1/f1.jpg": "ok", "evidence/r2/f2.jpg": "never"})
	st.getErr["evidence/r2/f2.jpg"] = errors.New("must not be read")
	svc := NewService(Config{Store: store, Storage: st})

	f, rc, err := svc.Open(t.Context(), "r1", "f1")
	if err != nil || rc == nil {
		t.Fatalf("Open(viewable) = %v, %v", rc, err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(b) != "ok" || f.ID != "f1" {
		t.Errorf("streamed %q for %s", b, f.ID)
	}

	f, rc, err = svc.Open(t.Context(), "r2", "f2")
	if err != nil || rc != nil || f.Viewable() {
		t.Errorf("Open(csam) = %v, %v, viewable=%v; want metadata only", rc, err, f.Viewable())
	}

	if _, _, err := svc.Open(t.Context(), "r1", "f2"); !errors.Is(err, ErrNotFound) {
		t.Errorf("file from another report: error = %v, want ErrNotFound", err)
	}
}

func TestSweep_UsesTheRetentionCutoffAndMarksPurged(t *testing.T) {
	store := &memStore{
		due:   []string{"a", "b"},
		files: []File{{ID: "f", ReportID: "a", StorageKey: "evidence/a/f"}},
	}
	svc := NewService(Config{Store: store, Storage: newMemStorage(map[string]string{"evidence/a/f": "x"}), RetentionDays: 30})
	svc.now = func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) }

	res, err := svc.Sweep(t.Context(), 10)
	if err != nil {
		t.Fatalf("Sweep() error = %v", err)
	}
	if store.gotCut != "2026-03-01" {
		t.Errorf("cutoff = %s, want 30 days back", store.gotCut)
	}
	if res.Purged != 2 || len(store.purged) != 2 || len(store.files) != 0 {
		t.Errorf("result %+v, purged %v, files left %d", res, store.purged, len(store.files))
	}
}

func TestSweep_LeavesFailedReportsForTheNextRun(t *testing.T) {
	store := &memStore{
		due:   []string{"a"},
		files: []File{{ID: "f", ReportID: "a", StorageKey: "evidence/a/f"}},
	}
	st := newMemStorage(map[string]string{"evidence/a/f": "x"})
	st.deleteErr = errors.New("offline")
	svc := NewService(Config{Store: store, Storage: st})

	res, err := svc.Sweep(t.Context(), 10)
	if err == nil || res.Failed != 1 || res.Purged != 0 || len(store.purged) != 0 {
		t.Errorf("Sweep() = %+v, %v; want one failure and nothing marked purged", res, err)
	}

	store.markErr, st.deleteErr = errors.New("update failed"), nil
	if res, err := svc.Sweep(t.Context(), 10); err == nil || res.Failed != 1 {
		t.Errorf("mark failure: Sweep() = %+v, %v; want it counted", res, err)
	}

	store.dueErr = errors.New("query failed")
	if _, err := svc.Sweep(t.Context(), 10); !errors.Is(err, store.dueErr) {
		t.Errorf("error = %v, want the lookup failure", err)
	}
}

func TestNewService_DefaultsRetention(t *testing.T) {
	for _, days := range []int{0, -5} {
		if got := NewService(Config{RetentionDays: days}).retentionDays; got != DefaultRetentionDays {
			t.Errorf("RetentionDays %d gave %d, want %d", days, got, DefaultRetentionDays)
		}
	}
	if got := NewService(Config{RetentionDays: 90}).retentionDays; got != 90 {
		t.Errorf("RetentionDays 90 gave %d", got)
	}
}
