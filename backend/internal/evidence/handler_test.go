package evidence

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

type fakeReader struct {
	files   []File
	listErr error
	file    *File
	body    string
	openErr error
	gotID   string
}

func (f *fakeReader) List(_ context.Context, reportID string) ([]File, error) {
	f.gotID = reportID
	return f.files, f.listErr
}

func (f *fakeReader) Open(_ context.Context, reportID, _ string) (*File, io.ReadCloser, error) {
	f.gotID = reportID
	if f.openErr != nil {
		return nil, nil, f.openErr
	}
	if !f.file.Viewable() {
		return f.file, nil, nil
	}
	return f.file, io.NopCloser(strings.NewReader(f.body)), nil
}

// serveEvidence mounts the handler the way the reports router does.
func serveEvidence(reader Reader, target string) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	r.Mount("/reports/{id}/evidence", NewAdminHandler(reader))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestAdminHandler_ListsMetadataWithoutStorageKeys(t *testing.T) {
	reader := &fakeReader{files: []File{{ID: "f1", ReportID: "r1", Kind: KindMessageMedia, StorageKey: "evidence/r1/f1.jpg", SHA256: "abc"}}}
	rec := serveEvidence(reader, "/reports/r1/evidence")
	if rec.Code != http.StatusOK || reader.gotID != "r1" {
		t.Fatalf("status = %d, report = %q", rec.Code, reader.gotID)
	}
	if strings.Contains(rec.Body.String(), "evidence/r1") {
		t.Errorf("response leaks the storage key: %s", rec.Body.String())
	}
	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || len(got) != 1 || got[0]["sha256"] != "abc" {
		t.Errorf("body = %s (%v)", rec.Body.String(), err)
	}
}

func TestAdminHandler_ListError(t *testing.T) {
	if rec := serveEvidence(&fakeReader{listErr: errors.New("db")}, "/reports/r1/evidence"); rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestAdminHandler_StreamsViewableFilesSafely(t *testing.T) {
	reader := &fakeReader{file: &File{ID: "f1", ContentType: "image/jpeg", SizeBytes: 5, ReportReason: "harassment"}, body: "bytes"}
	rec := serveEvidence(reader, "/reports/r1/evidence/f1")
	if rec.Code != http.StatusOK || rec.Body.String() != "bytes" {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	for header, want := range map[string]string{
		"Content-Type":           "image/jpeg",
		"Content-Length":         "5",
		"Cache-Control":          "no-store",
		"X-Content-Type-Options": "nosniff",
		"Content-Disposition":    `attachment; filename="f1"`,
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestAdminHandler_RefusesCSAM(t *testing.T) {
	reader := &fakeReader{file: &File{ID: "f1", ReportReason: "csam"}, body: "never"}
	rec := serveEvidence(reader, "/reports/r1/evidence/f1")
	if rec.Code != http.StatusForbidden || strings.Contains(rec.Body.String(), "never") {
		t.Fatalf("status = %d, body = %q; want 403 without bytes", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "evidence_restricted") {
		t.Errorf("body = %s, want evidence_restricted", rec.Body.String())
	}
}

func TestAdminHandler_OpenErrors(t *testing.T) {
	if rec := serveEvidence(&fakeReader{openErr: ErrNotFound}, "/reports/r1/evidence/f9"); rec.Code != http.StatusNotFound {
		t.Errorf("missing file: status = %d, want 404", rec.Code)
	}
	if rec := serveEvidence(&fakeReader{openErr: errors.New("storage")}, "/reports/r1/evidence/f1"); rec.Code != http.StatusInternalServerError {
		t.Errorf("storage failure: status = %d, want 500", rec.Code)
	}
}
