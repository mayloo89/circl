package exports_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/exports"
)

type mockStore struct {
	create        *exports.Request
	createErr     error
	latest        *exports.Request
	latestErr     error
	byID          *exports.Request
	byIDErr       error
	byHash        *exports.Request
	byHashErr     error
	processingErr error
	readyErr      error
	failedErr     error
	downloadedErr error

	markProcessingCalled bool
	markReadyCalled      bool
	markFailedCalled     bool
	lastReadyStorageKey  string
	lastReadyTokenHash   string
	lastReadyExpiry      time.Time
	lastFailedErr        string
}

func (m *mockStore) Create(_ context.Context, _ string) (*exports.Request, error) {
	return m.create, m.createErr
}
func (m *mockStore) LatestForUser(_ context.Context, _ string) (*exports.Request, error) {
	return m.latest, m.latestErr
}
func (m *mockStore) GetByID(_ context.Context, _ string) (*exports.Request, error) {
	return m.byID, m.byIDErr
}
func (m *mockStore) GetByTokenHash(_ context.Context, _ string) (*exports.Request, error) {
	return m.byHash, m.byHashErr
}
func (m *mockStore) MarkProcessing(_ context.Context, _ string) error {
	m.markProcessingCalled = true
	return m.processingErr
}
func (m *mockStore) MarkReady(_ context.Context, _, storageKey, tokenHash string, expiresAt time.Time) error {
	m.markReadyCalled = true
	m.lastReadyStorageKey = storageKey
	m.lastReadyTokenHash = tokenHash
	m.lastReadyExpiry = expiresAt
	return m.readyErr
}
func (m *mockStore) MarkFailed(_ context.Context, _, errMsg string) error {
	m.markFailedCalled = true
	m.lastFailedErr = errMsg
	return m.failedErr
}
func (m *mockStore) MarkDownloaded(_ context.Context, _ string) error {
	return m.downloadedErr
}

type mockSource struct {
	bundle *exports.Bundle
	err    error
}

func (m *mockSource) BuildBundle(_ context.Context, _ string) (*exports.Bundle, error) {
	return m.bundle, m.err
}

type mockStorage struct {
	objects   map[string][]byte
	putErr    error
	getErr    error
	lastPutKey string
	lastPutCT  string
}

func newMockStorage() *mockStorage {
	return &mockStorage{objects: map[string][]byte{}}
}

func (m *mockStorage) GetObject(_ context.Context, key string) (io.ReadCloser, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	b, ok := m.objects[key]
	if !ok {
		return nil, errors.New("no such key")
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (m *mockStorage) PutObject(_ context.Context, key, ct string, r io.Reader, _ int64) error {
	if m.putErr != nil {
		return m.putErr
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.objects[key] = b
	m.lastPutKey = key
	m.lastPutCT = ct
	return nil
}

type mockMailer struct {
	readyCalled    bool
	readyTo        string
	readyURL       string
	failedCalled   bool
	failedTo       string
}

func (m *mockMailer) SendReadyEmail(_ context.Context, to, url string, _ time.Time) error {
	m.readyCalled = true
	m.readyTo = to
	m.readyURL = url
	return nil
}
func (m *mockMailer) SendFailedEmail(_ context.Context, to string) error {
	m.failedCalled = true
	m.failedTo = to
	return nil
}

func newService(t *testing.T, store *mockStore, src *mockSource, stor *mockStorage, mail *mockMailer) *exports.Service {
	t.Helper()
	return exports.NewService(exports.Config{
		Store:           store,
		Source:          src,
		Storage:         stor,
		Mailer:          mail,
		DownloadURLBase: "https://api.example/account/export",
		Log:             zerolog.Nop(),
	})
}

func sampleBundle() *exports.Bundle {
	return &exports.Bundle{
		Snapshot: exports.Snapshot{
			SchemaVersion: exports.SchemaVersion,
			GeneratedAt:   time.Now().UTC(),
			Account: exports.Account{
				ID:    "u-1",
				Email: "u@example.com",
			},
		},
	}
}

func TestRequest_EnqueueCalled(t *testing.T) {
	store := &mockStore{create: &exports.Request{ID: "r-1", Status: exports.StatusPending}}
	enqueueCalls := 0
	svc := exports.NewService(exports.Config{
		Store:  store,
		Source: &mockSource{},
		Storage: newMockStorage(),
		Enqueue: func(_ context.Context, _, _ string) error {
			enqueueCalls++
			return nil
		},
		Log: zerolog.Nop(),
	})
	r, err := svc.Request(t.Context(), "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ID != "r-1" {
		t.Errorf("ID = %q, want r-1", r.ID)
	}
	if enqueueCalls != 1 {
		t.Errorf("enqueue calls = %d, want 1", enqueueCalls)
	}
}

func TestRequest_AlreadyPending(t *testing.T) {
	svc := newService(t, &mockStore{createErr: exports.ErrAlreadyPending}, &mockSource{}, newMockStorage(), &mockMailer{})

	_, err := svc.Request(t.Context(), "u-1")
	if !errors.Is(err, exports.ErrAlreadyPending) {
		t.Errorf("err = %v, want ErrAlreadyPending", err)
	}
}

func TestRequest_RateLimited(t *testing.T) {
	svc := newService(t, &mockStore{createErr: exports.ErrRateLimited}, &mockSource{}, newMockStorage(), &mockMailer{})

	_, err := svc.Request(t.Context(), "u-1")
	if !errors.Is(err, exports.ErrRateLimited) {
		t.Errorf("err = %v, want ErrRateLimited", err)
	}
}

func TestBuild_WritesZipAndEmails(t *testing.T) {
	bundle := sampleBundle()
	bundle.Media = []exports.MediaItem{
		{StorageKey: "key/photo.jpg", ContentType: "image/jpeg", ArchivePath: "media/photos/p1.jpg"},
	}
	stor := newMockStorage()
	stor.objects["key/photo.jpg"] = []byte("FAKEPHOTO")
	store := &mockStore{byID: &exports.Request{ID: "r-1", UserID: "u-1", Status: exports.StatusPending}}
	mail := &mockMailer{}
	svc := newService(t, store, &mockSource{bundle: bundle}, stor, mail)

	if err := svc.Build(t.Context(), "r-1"); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if !store.markProcessingCalled {
		t.Error("MarkProcessing was not called")
	}
	if !store.markReadyCalled {
		t.Error("MarkReady was not called")
	}
	if store.lastReadyTokenHash == "" {
		t.Error("token hash not stored")
	}
	if store.lastReadyExpiry.Before(time.Now().Add(13 * 24 * time.Hour)) {
		t.Errorf("expiry = %v, want ~14 days out", store.lastReadyExpiry)
	}
	if stor.lastPutCT != "application/zip" {
		t.Errorf("zip content-type = %q, want application/zip", stor.lastPutCT)
	}

	// Verify zip contents.
	body, ok := stor.objects[stor.lastPutKey]
	if !ok {
		t.Fatal("zip not written to storage")
	}
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("zip open: %v", err)
	}
	names := make(map[string]bool, len(zr.File))
	var dataJSON []byte
	for _, f := range zr.File {
		names[f.Name] = true
		if f.Name == "data.json" {
			r, _ := f.Open()
			dataJSON, _ = io.ReadAll(r)
			r.Close()
		}
	}
	if !names["data.json"] {
		t.Error("zip is missing data.json")
	}
	if !names["media/photos/p1.jpg"] {
		t.Error("zip is missing media/photos/p1.jpg")
	}

	// data.json should round-trip to a Snapshot.
	var snap exports.Snapshot
	if err := json.Unmarshal(dataJSON, &snap); err != nil {
		t.Fatalf("data.json invalid: %v", err)
	}
	if snap.SchemaVersion != exports.SchemaVersion {
		t.Errorf("schema_version = %q, want %q", snap.SchemaVersion, exports.SchemaVersion)
	}

	if !mail.readyCalled || mail.readyTo != "u@example.com" {
		t.Errorf("ready email not sent correctly: called=%v to=%q", mail.readyCalled, mail.readyTo)
	}
	if !strings.HasPrefix(mail.readyURL, "https://api.example/account/export/") {
		t.Errorf("download URL = %q, want prefix https://api.example/account/export/", mail.readyURL)
	}
}

func TestBuild_SkipsAlreadyReady(t *testing.T) {
	store := &mockStore{byID: &exports.Request{ID: "r-1", UserID: "u-1", Status: exports.StatusReady}}
	svc := newService(t, store, &mockSource{bundle: sampleBundle()}, newMockStorage(), &mockMailer{})

	if err := svc.Build(t.Context(), "r-1"); err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if store.markProcessingCalled {
		t.Error("MarkProcessing should not be called for already-ready request")
	}
}

func TestBuild_MissingMediaIsBestEffort(t *testing.T) {
	bundle := sampleBundle()
	bundle.Media = []exports.MediaItem{
		{StorageKey: "missing/key", ContentType: "image/jpeg", ArchivePath: "media/photos/missing.jpg"},
	}
	stor := newMockStorage() // empty
	store := &mockStore{byID: &exports.Request{ID: "r-1", UserID: "u-1", Status: exports.StatusPending}}
	svc := newService(t, store, &mockSource{bundle: bundle}, stor, &mockMailer{})

	if err := svc.Build(t.Context(), "r-1"); err != nil {
		t.Fatalf("Build() should not fail for missing media: %v", err)
	}
	if !store.markReadyCalled {
		t.Error("MarkReady not called despite missing media")
	}
}

func TestBuild_SourceErrorMarksFailed(t *testing.T) {
	store := &mockStore{byID: &exports.Request{ID: "r-1", UserID: "u-1", Status: exports.StatusPending}}
	src := &mockSource{err: errors.New("db down")}
	// notifyFailed re-fetches via source which will keep failing — the mailer
	// must not be asserted on, but the failed-marker must fire.
	svc := newService(t, store, src, newMockStorage(), &mockMailer{})

	if err := svc.Build(t.Context(), "r-1"); err == nil {
		t.Fatal("expected Build() to fail")
	}
	if !store.markFailedCalled {
		t.Error("MarkFailed was not called")
	}
}

func TestDownload_InvalidToken(t *testing.T) {
	svc := newService(t, &mockStore{byHashErr: exports.ErrNotFound}, &mockSource{}, newMockStorage(), &mockMailer{})

	_, _, err := svc.Download(t.Context(), "anything")
	if !errors.Is(err, exports.ErrInvalidToken) {
		t.Errorf("err = %v, want ErrInvalidToken", err)
	}
}

func TestDownload_NotReady(t *testing.T) {
	svc := newService(t, &mockStore{byHash: &exports.Request{ID: "r-1", Status: exports.StatusPending}}, &mockSource{}, newMockStorage(), &mockMailer{})

	_, _, err := svc.Download(t.Context(), "tok")
	if !errors.Is(err, exports.ErrNotReady) {
		t.Errorf("err = %v, want ErrNotReady", err)
	}
}

func TestDownload_Expired(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	key := "exports/u-1/r-1.zip"
	svc := newService(t, &mockStore{byHash: &exports.Request{
		ID: "r-1", Status: exports.StatusReady, ExpiresAt: &past, StorageKey: &key,
	}}, &mockSource{}, newMockStorage(), &mockMailer{})

	_, _, err := svc.Download(t.Context(), "tok")
	if !errors.Is(err, exports.ErrInvalidToken) {
		t.Errorf("err = %v, want ErrInvalidToken (expired)", err)
	}
}

func TestDownload_HappyPath(t *testing.T) {
	future := time.Now().Add(time.Hour)
	key := "exports/u-1/r-1.zip"
	stor := newMockStorage()
	stor.objects[key] = []byte("ZIP-BYTES")
	svc := newService(t, &mockStore{byHash: &exports.Request{
		ID: "r-1", Status: exports.StatusReady, ExpiresAt: &future, StorageKey: &key,
	}}, &mockSource{}, stor, &mockMailer{})

	req, rc, err := svc.Download(t.Context(), "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer rc.Close()
	if req.ID != "r-1" {
		t.Errorf("req.ID = %q, want r-1", req.ID)
	}
	body, _ := io.ReadAll(rc)
	if string(body) != "ZIP-BYTES" {
		t.Errorf("body = %q, want ZIP-BYTES", string(body))
	}
}

func TestHashToken_DifferentForDifferentInput(t *testing.T) {
	if exports.HashToken("a") == exports.HashToken("b") {
		t.Error("hashes collided for different inputs")
	}
}

func TestGenerateToken_HashMatches(t *testing.T) {
	plain, hash, err := exports.GenerateToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plain == "" || hash == "" {
		t.Error("empty token")
	}
	if exports.HashToken(plain) != hash {
		t.Error("hash does not match plain")
	}
}
