package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/moderation"
	"github.com/mayloo89/circl/backend/internal/uploads"
)

// --- Test doubles ---

type fakeStorage struct {
	objects     map[string]string
	putErr      error
	getErr      error
	delErr      error
	lastPutKey  string
	lastPutType string
}

func newFakeStorage(key, content string) *fakeStorage {
	fs := &fakeStorage{objects: make(map[string]string)}
	if key != "" {
		fs.objects[key] = content
	}
	return fs
}

func (f *fakeStorage) GetObject(_ context.Context, key string) (io.ReadCloser, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	content, ok := f.objects[key]
	if !ok {
		return nil, errors.New("storage: not found")
	}
	return io.NopCloser(strings.NewReader(content)), nil
}

func (f *fakeStorage) PutObject(_ context.Context, key, contentType string, r io.Reader, _ int64) error {
	if f.putErr != nil {
		return f.putErr
	}
	data, _ := io.ReadAll(r)
	f.objects[key] = string(data)
	f.lastPutKey = key
	f.lastPutType = contentType
	return nil
}

func (f *fakeStorage) Delete(_ context.Context, key string) error {
	if f.delErr != nil {
		return f.delErr
	}
	delete(f.objects, key)
	return nil
}

// fakeStore captures calls to SetThumbnailKey.
type fakeStore struct {
	thumbnailKey string
	uploadID     string
	err          error
}

func (f *fakeStore) SetThumbnailKey(_ context.Context, id, key string) error {
	f.uploadID = id
	f.thumbnailKey = key
	return f.err
}

// --- Helpers ---

func makeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 200, G: 100, B: 50, A: 255})
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// --- thumbnailKey ---

func TestThumbnailKey_JPEG(t *testing.T) {
	got := thumbnailKey("chat-attachment/user/abc-photo.jpg")
	want := "thumbnails/chat-attachment/user/abc-photo.jpg"
	if got != want {
		t.Errorf("thumbnailKey = %q, want %q", got, want)
	}
}

func TestThumbnailKey_PNG(t *testing.T) {
	got := thumbnailKey("chat-attachment/user/abc-photo.png")
	want := "thumbnails/chat-attachment/user/abc-photo.jpg"
	if got != want {
		t.Errorf("thumbnailKey = %q, want %q", got, want)
	}
}

func TestThumbnailKey_NoExtension(t *testing.T) {
	got := thumbnailKey("chat-attachment/user/file")
	want := "thumbnails/chat-attachment/user/file.jpg"
	if got != want {
		t.Errorf("thumbnailKey = %q, want %q", got, want)
	}
}

// --- resizeToFit ---

func TestResizeToFit_AlreadySmall(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 100, 100))
	out := resizeToFit(src, 480)
	if out != src {
		t.Error("expected same image to be returned when already within bounds")
	}
}

func TestResizeToFit_LandscapeReduces(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 960, 480))
	out := resizeToFit(src, 480)
	b := out.Bounds()
	if b.Dx() != 480 {
		t.Errorf("width = %d, want 480", b.Dx())
	}
	if b.Dy() != 240 {
		t.Errorf("height = %d, want 240", b.Dy())
	}
}

func TestResizeToFit_PortraitReduces(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 320, 640))
	out := resizeToFit(src, 480)
	b := out.Bounds()
	if b.Dy() != 480 {
		t.Errorf("height = %d, want 480", b.Dy())
	}
	if b.Dx() != 240 {
		t.Errorf("width = %d, want 240", b.Dx())
	}
}

// --- process: JPEG ---

func TestProcess_JPEG_StripsEXIFAndGeneratesThumbnail(t *testing.T) {
	key := "chat-attachment/user/photo.jpg"
	imgData := makeJPEG(t, 800, 600)

	st := newFakeStorage(key, string(imgData))
	store := &fakeStore{}
	proc := NewImageProcessor(st, store, 0, zerolog.Nop())

	err := proc.process(t.Context(), ImageProcessPayload{
		UploadID:    "upload-1",
		StorageKey:  key,
		ContentType: "image/jpeg",
	})
	if err != nil {
		t.Fatalf("process() error: %v", err)
	}

	// Original should be replaced (EXIF strip).
	if _, ok := st.objects[key]; !ok {
		t.Error("expected original key to be re-uploaded after EXIF strip")
	}

	// Thumbnail should be uploaded.
	thumbKey := thumbnailKey(key)
	if _, ok := st.objects[thumbKey]; !ok {
		t.Errorf("expected thumbnail at %q", thumbKey)
	}

	// Store should be updated.
	if store.uploadID != "upload-1" {
		t.Errorf("upload ID = %q, want upload-1", store.uploadID)
	}
	if store.thumbnailKey != thumbKey {
		t.Errorf("thumbnail key = %q, want %q", store.thumbnailKey, thumbKey)
	}
}

// --- process: PNG ---

func TestProcess_PNG_StripsEXIFAndGeneratesThumbnail(t *testing.T) {
	key := "chat-attachment/user/image.png"
	imgData := makePNG(t, 600, 400)

	st := newFakeStorage(key, string(imgData))
	store := &fakeStore{}
	proc := NewImageProcessor(st, store, 0, zerolog.Nop())

	if err := proc.process(t.Context(), ImageProcessPayload{
		UploadID: "upload-2", StorageKey: key, ContentType: "image/png",
	}); err != nil {
		t.Fatalf("process() error: %v", err)
	}

	thumbKey := thumbnailKey(key)
	if _, ok := st.objects[thumbKey]; !ok {
		t.Errorf("expected thumbnail at %q", thumbKey)
	}
	if store.thumbnailKey != thumbKey {
		t.Errorf("thumbnail key = %q, want %q", store.thumbnailKey, thumbKey)
	}
}

// --- process: GIF (thumbnail only, no EXIF strip) ---

func TestProcess_GIF_ThumbnailOnly(t *testing.T) {
	key := "chat-attachment/user/anim.gif"

	// Minimal valid 1x1 GIF89a.
	gifData := []byte{
		0x47, 0x49, 0x46, 0x38, 0x39, 0x61, // GIF89a
		0x01, 0x00, 0x01, 0x00, 0x80, 0x00, 0x00, // Logical Screen Descriptor
		0xff, 0xff, 0xff, 0x00, 0x00, 0x00, // Global Color Table (white, black)
		0x2c, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, // Image Descriptor
		0x02, 0x02, 0x4c, 0x01, 0x00, // Image Data (LZW)
		0x3b, // Trailer
	}

	st := newFakeStorage(key, string(gifData))
	store := &fakeStore{}
	proc := NewImageProcessor(st, store, 0, zerolog.Nop())

	if err := proc.process(t.Context(), ImageProcessPayload{
		UploadID: "upload-3", StorageKey: key, ContentType: "image/gif",
	}); err != nil {
		t.Fatalf("process() error: %v", err)
	}

	// Original must NOT be replaced (no EXIF strip for GIF).
	if st.lastPutKey == key {
		t.Error("GIF original should not be re-uploaded")
	}

	// Thumbnail must exist.
	thumbKey := thumbnailKey(key)
	if _, ok := st.objects[thumbKey]; !ok {
		t.Errorf("expected thumbnail at %q", thumbKey)
	}
}

// --- process: error cases ---

func TestProcess_GetObjectError(t *testing.T) {
	st := &fakeStorage{getErr: errors.New("not found"), objects: map[string]string{}}
	proc := NewImageProcessor(st, &fakeStore{}, 0, zerolog.Nop())

	err := proc.process(t.Context(), ImageProcessPayload{
		UploadID: "u", StorageKey: "k", ContentType: "image/jpeg",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProcess_PutObjectError_OnStrip(t *testing.T) {
	key := "chat-attachment/user/photo.jpg"
	imgData := makeJPEG(t, 10, 10)
	st := newFakeStorage(key, string(imgData))
	st.putErr = errors.New("put failed")
	proc := NewImageProcessor(st, &fakeStore{}, 0, zerolog.Nop())

	err := proc.process(t.Context(), ImageProcessPayload{
		UploadID: "u", StorageKey: key, ContentType: "image/jpeg",
	})
	if err == nil {
		t.Fatal("expected error on PutObject")
	}
}

func TestProcess_SetThumbnailKeyError(t *testing.T) {
	key := "chat-attachment/user/photo.jpg"
	imgData := makeJPEG(t, 10, 10)
	st := newFakeStorage(key, string(imgData))
	store := &fakeStore{err: errors.New("db error")}
	proc := NewImageProcessor(st, store, 0, zerolog.Nop())

	err := proc.process(t.Context(), ImageProcessPayload{
		UploadID: "u", StorageKey: key, ContentType: "image/jpeg",
	})
	if err == nil {
		t.Fatal("expected error from SetThumbnailKey")
	}
}

// --- Handle ---

func TestHandle_ValidPayload(t *testing.T) {
	key := "chat-attachment/user/photo.jpg"
	imgData := makeJPEG(t, 10, 10)
	st := newFakeStorage(key, string(imgData))
	proc := NewImageProcessor(st, &fakeStore{}, 0, zerolog.Nop())

	payload, _ := json.Marshal(ImageProcessPayload{
		UploadID: "u1", StorageKey: key, ContentType: "image/jpeg",
	})
	task := asynq.NewTask(TaskProcessImage, payload)
	if err := proc.Handle(t.Context(), task); err != nil {
		t.Fatalf("Handle() error: %v", err)
	}
}

func TestHandle_InvalidJSON(t *testing.T) {
	proc := NewImageProcessor(newFakeStorage("", ""), &fakeStore{}, 0, zerolog.Nop())
	task := asynq.NewTask(TaskProcessImage, []byte("not-json"))
	if err := proc.Handle(t.Context(), task); err == nil {
		t.Fatal("expected error for invalid JSON payload")
	}
}

func TestHandle_ProcessError(t *testing.T) {
	// Storage returns an error → Handle must return the error.
	st := &fakeStorage{getErr: errors.New("not found"), objects: map[string]string{}}
	proc := NewImageProcessor(st, &fakeStore{}, 0, zerolog.Nop())

	payload, _ := json.Marshal(ImageProcessPayload{
		UploadID: "u1", StorageKey: "missing/key.jpg", ContentType: "image/jpeg",
	})
	task := asynq.NewTask(TaskProcessImage, payload)
	if err := proc.Handle(t.Context(), task); err == nil {
		t.Fatal("expected error when process fails")
	}
}

// --- EnqueueProcessImage ---

func TestEnqueueProcessImage(t *testing.T) {
	mr := miniredis.RunT(t)
	client := NewClient(asynq.RedisClientOpt{Addr: mr.Addr()})
	defer client.Close() //nolint:errcheck

	err := EnqueueProcessImage(t.Context(), client, ImageProcessPayload{
		UploadID: "u1", StorageKey: "chat-attachment/u/f.jpg", ContentType: "image/jpeg",
	})
	if err != nil {
		t.Fatalf("EnqueueProcessImage() error: %v", err)
	}
}

// --- NewServer / Start / Shutdown ---

func TestServer_StartAndShutdown(t *testing.T) {
	mr := miniredis.RunT(t)
	srv := NewServer(asynq.RedisClientOpt{Addr: mr.Addr()}, 2, zerolog.Nop())
	proc := NewImageProcessor(newFakeStorage("", ""), &fakeStore{}, 0, zerolog.Nop())
	if err := srv.Start(proc, nil); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	srv.Shutdown() // must not panic
}

// --- decodeImage: unsupported type ---

func TestDecodeImage_UnsupportedType(t *testing.T) {
	_, err := decodeImage("application/pdf", []byte("pdf bytes"))
	if err == nil {
		t.Fatal("expected error for unsupported content type")
	}
}

func TestDecodeImage_WebPInvalidData(t *testing.T) {
	_, err := decodeImage("image/webp", []byte("not-valid-webp-data"))
	if err == nil {
		t.Fatal("expected error for invalid WebP bytes")
	}
}

func TestDecodeImage_GIFInvalidData(t *testing.T) {
	_, err := decodeImage("image/gif", []byte("not-valid-gif"))
	if err == nil {
		t.Fatal("expected error for invalid GIF bytes")
	}
}

// --- encodeOriginal: unsupported type ---

func TestEncodeOriginal_UnsupportedType(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	_, err := encodeOriginal(img, "image/gif")
	if err == nil {
		t.Fatal("expected error for unsupported encode type")
	}
}

// --- resizeToFit: extreme aspect ratios ---

func TestResizeToFit_ExtremeWide(t *testing.T) {
	// Very wide image: 10000×1. dh would be 0 without the guard.
	src := image.NewRGBA(image.Rect(0, 0, 10000, 1))
	out := resizeToFit(src, 480)
	b := out.Bounds()
	if b.Dy() < 1 {
		t.Errorf("height = %d, want >= 1", b.Dy())
	}
	if b.Dx() != 480 {
		t.Errorf("width = %d, want 480", b.Dx())
	}
}

func TestResizeToFit_ExtremeTall(t *testing.T) {
	// Very tall image: 1×10000. dw would be 0 without the guard.
	src := image.NewRGBA(image.Rect(0, 0, 1, 10000))
	out := resizeToFit(src, 480)
	b := out.Bounds()
	if b.Dx() < 1 {
		t.Errorf("width = %d, want >= 1", b.Dx())
	}
	if b.Dy() != 480 {
		t.Errorf("height = %d, want 480", b.Dy())
	}
}

// --- process: io.ReadAll error ---

func TestProcess_ReadError(t *testing.T) {
	st := &brokenReadStorage{}
	proc := NewImageProcessor(st, &fakeStore{}, 0, zerolog.Nop())

	err := proc.process(t.Context(), ImageProcessPayload{
		UploadID: "u", StorageKey: "k.jpg", ContentType: "image/jpeg",
	})
	if err == nil {
		t.Fatal("expected error when reading object fails")
	}
}

// brokenReadStorage returns a reader that always fails on Read.
type brokenReadStorage struct{}

func (b *brokenReadStorage) GetObject(_ context.Context, _ string) (io.ReadCloser, error) {
	return &errReadCloser{}, nil
}

func (b *brokenReadStorage) PutObject(_ context.Context, _, _ string, _ io.Reader, _ int64) error {
	return nil
}

func (b *brokenReadStorage) Delete(_ context.Context, _ string) error {
	return nil
}

type errReadCloser struct{}

func (e *errReadCloser) Read(_ []byte) (int, error) { return 0, errors.New("read error") }
func (e *errReadCloser) Close() error               { return nil }

// --- process: PutObject error on thumbnail ---

func TestProcess_PutObjectError_OnThumbnail(t *testing.T) {
	key := "chat-attachment/user/photo.png"
	imgData := makePNG(t, 10, 10)

	callCount := 0
	st := &fakeStorage{objects: map[string]string{key: string(imgData)}}
	// Fail only the second PutObject call (thumbnail), not the first (EXIF strip).
	origPutObject := st.PutObject
	_ = origPutObject // not a method we can replace; use putErr approach differently

	// We need a storage that fails on the second call. Use a wrapper.
	wrapped := &countingPutStorage{inner: st, failAfter: 1}
	proc := NewImageProcessor(wrapped, &fakeStore{}, 0, zerolog.Nop())

	err := proc.process(t.Context(), ImageProcessPayload{
		UploadID: "u", StorageKey: key, ContentType: "image/png",
	})
	_ = callCount
	if err == nil {
		t.Fatal("expected error when thumbnail PutObject fails")
	}
}

// countingPutStorage wraps fakeStorage and fails PutObject after failAfter calls.
type countingPutStorage struct {
	inner     *fakeStorage
	failAfter int
	calls     int
}

func (c *countingPutStorage) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	return c.inner.GetObject(ctx, key)
}

func (c *countingPutStorage) PutObject(ctx context.Context, key, contentType string, r io.Reader, size int64) error {
	c.calls++
	if c.calls > c.failAfter {
		return errors.New("put failed")
	}
	return c.inner.PutObject(ctx, key, contentType, r, size)
}

func (c *countingPutStorage) Delete(ctx context.Context, key string) error {
	return c.inner.Delete(ctx, key)
}

// --- Original resize ---

func TestProcess_ResizesOversizedJPEG(t *testing.T) {
	key := "chat-attachment/user/big.jpg"
	// Create a JPEG that exceeds imageMaxPx in both dimensions.
	imgData := makeJPEG(t, 200, 200)
	st := newFakeStorage(key, string(imgData))

	// Set imageMaxPx to 50 so our 200×200 image gets resized.
	proc := NewImageProcessor(st, &fakeStore{}, 50, zerolog.Nop())
	if err := proc.process(t.Context(), ImageProcessPayload{
		UploadID: "u", StorageKey: key, ContentType: "image/jpeg",
	}); err != nil {
		t.Fatalf("process() error: %v", err)
	}

	// The stored original should now be ≤50px on its longest edge.
	storedData := []byte(st.objects[key])
	img, err := jpeg.Decode(bytes.NewReader(storedData))
	if err != nil {
		t.Fatalf("decode stored JPEG: %v", err)
	}
	b := img.Bounds()
	if b.Dx() > 50 || b.Dy() > 50 {
		t.Errorf("stored image dimensions %dx%d exceed imageMaxPx=50", b.Dx(), b.Dy())
	}
}

func TestProcess_DoesNotResizeWithinLimitJPEG(t *testing.T) {
	key := "chat-attachment/user/small.jpg"
	imgData := makeJPEG(t, 20, 20)
	st := newFakeStorage(key, string(imgData))

	proc := NewImageProcessor(st, &fakeStore{}, 100, zerolog.Nop())
	if err := proc.process(t.Context(), ImageProcessPayload{
		UploadID: "u", StorageKey: key, ContentType: "image/jpeg",
	}); err != nil {
		t.Fatalf("process() error: %v", err)
	}

	storedData := []byte(st.objects[key])
	img, err := jpeg.Decode(bytes.NewReader(storedData))
	if err != nil {
		t.Fatalf("decode stored JPEG: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != 20 || b.Dy() != 20 {
		t.Errorf("stored image dimensions %dx%d, want 20x20", b.Dx(), b.Dy())
	}
}

func TestProcess_ResizesOversizedPNG(t *testing.T) {
	key := "chat-attachment/user/big.png"
	imgData := makePNG(t, 200, 200)
	st := newFakeStorage(key, string(imgData))

	proc := NewImageProcessor(st, &fakeStore{}, 50, zerolog.Nop())
	if err := proc.process(t.Context(), ImageProcessPayload{
		UploadID: "u", StorageKey: key, ContentType: "image/png",
	}); err != nil {
		t.Fatalf("process() error: %v", err)
	}

	storedData := []byte(st.objects[key])
	img, err := png.Decode(bytes.NewReader(storedData))
	if err != nil {
		t.Fatalf("decode stored PNG: %v", err)
	}
	b := img.Bounds()
	if b.Dx() > 50 || b.Dy() > 50 {
		t.Errorf("stored PNG dimensions %dx%d exceed imageMaxPx=50", b.Dx(), b.Dy())
	}
}

// --- moderation integration ---

// fakeModerator returns whatever decision it was constructed with.
type fakeModerator struct {
	decision moderation.Decision
	err      error
	sev      moderation.Severity
}

func (f *fakeModerator) Check(_ context.Context, _ moderation.Input) (moderation.Decision, error) {
	return f.decision, f.err
}
func (f *fakeModerator) Name() string                 { return "fake" }
func (f *fakeModerator) Severity() moderation.Severity { return f.sev }

// fakeModerationStore captures MarkApproved / MarkRejected / MarkQuarantined calls.
type fakeModerationStore struct {
	approvedID         string
	rejectedID         string
	rejectedCode       string
	rejectedScore      float64
	rejectedCategories []string
	rejectedRetained   bool
	quarantinedID      string
	quarantineKey      string
}

func (f *fakeModerationStore) MarkApproved(_ context.Context, id string) error {
	f.approvedID = id
	return nil
}
func (f *fakeModerationStore) MarkRejected(_ context.Context, rec uploads.RejectionRecord) error {
	f.rejectedID = rec.UploadID
	f.rejectedCode = rec.Code
	f.rejectedScore = rec.Score
	f.rejectedCategories = rec.Categories
	f.rejectedRetained = rec.FileRetained
	return nil
}
func (f *fakeModerationStore) MarkQuarantined(_ context.Context, rec uploads.RejectionRecord, quarantineKey string) error {
	f.quarantinedID = rec.UploadID
	f.quarantineKey = quarantineKey
	f.rejectedCode = rec.Code
	return nil
}

func TestProcess_ModerationApprovesContinuesPipeline(t *testing.T) {
	key := "chat-attachment/user/photo.png"
	imgData := makePNG(t, 200, 200)
	st := newFakeStorage(key, string(imgData))
	store := &fakeStore{}
	modStore := &fakeModerationStore{}

	proc := NewImageProcessor(st, store, 0, zerolog.Nop())
	proc.SetModeration(&fakeModerator{decision: moderation.Allow()}, modStore)

	err := proc.process(t.Context(), ImageProcessPayload{
		UploadID: "u-1", StorageKey: key, ContentType: "image/png",
	})
	if err != nil {
		t.Fatalf("process error: %v", err)
	}
	if modStore.approvedID != "u-1" {
		t.Errorf("expected MarkApproved(u-1), got %q", modStore.approvedID)
	}
	if store.thumbnailKey == "" {
		t.Error("expected thumbnail to be generated after approval")
	}
	if _, ok := st.objects[key]; !ok {
		t.Error("original should remain in storage after approval")
	}
}

func TestProcess_ModerationRejectsDeletesOriginal(t *testing.T) {
	key := "chat-attachment/user/photo.png"
	imgData := makePNG(t, 200, 200)
	st := newFakeStorage(key, string(imgData))
	store := &fakeStore{}
	modStore := &fakeModerationStore{}

	proc := NewImageProcessor(st, store, 0, zerolog.Nop())
	proc.SetModeration(&fakeModerator{
		decision: moderation.RejectWith(moderation.CodeHashMatch, "test reason", "fake", moderation.DispositionPurge),
	}, modStore)

	err := proc.process(t.Context(), ImageProcessPayload{
		UploadID: "u-2", StorageKey: key, ContentType: "image/png",
	})
	if err != nil {
		t.Fatalf("process error: %v", err)
	}
	if modStore.rejectedID != "u-2" {
		t.Errorf("expected MarkRejected(u-2), got %q", modStore.rejectedID)
	}
	if modStore.rejectedCode != moderation.CodeHashMatch {
		t.Errorf("rejected code = %q, want hash_match", modStore.rejectedCode)
	}
	if _, ok := st.objects[key]; ok {
		t.Error("original should be deleted after rejection")
	}
	if store.thumbnailKey != "" {
		t.Error("thumbnail should NOT be generated after rejection")
	}
}

func TestProcess_ModerationErrorFailsOpen(t *testing.T) {
	// A flaky classifier should not block the upload — moderator errors are
	// logged and the pipeline proceeds. This protects the user from outages
	// in a downstream NSFW model.
	key := "chat-attachment/user/photo.png"
	imgData := makePNG(t, 200, 200)
	st := newFakeStorage(key, string(imgData))
	store := &fakeStore{}
	modStore := &fakeModerationStore{}

	proc := NewImageProcessor(st, store, 0, zerolog.Nop())
	proc.SetModeration(&fakeModerator{err: errors.New("classifier down")}, modStore)

	err := proc.process(t.Context(), ImageProcessPayload{
		UploadID: "u-3", StorageKey: key, ContentType: "image/png",
	})
	if err != nil {
		t.Fatalf("process error: %v", err)
	}
	if modStore.rejectedID != "" {
		t.Error("expected no rejection on moderator error")
	}
	if store.thumbnailKey == "" {
		t.Error("expected thumbnail to be generated despite moderator error")
	}
}

func TestProcess_NSFWRejectionRetainsFileAndScore(t *testing.T) {
	// NSFW rejections keep the file around for admin review (false-positive
	// auditing) and persist the classifier score + per-region categories.
	key := "chat-attachment/user/photo.png"
	imgData := makePNG(t, 200, 200)
	st := newFakeStorage(key, string(imgData))
	store := &fakeStore{}
	modStore := &fakeModerationStore{}

	decision := moderation.Reject(moderation.CodeNSFWDetected, "explicit", "nsfw")
	decision.Score = 0.94
	decision.Categories = []string{"FEMALE_BREAST_EXPOSED", "BUTTOCKS_EXPOSED"}

	proc := NewImageProcessor(st, store, 0, zerolog.Nop())
	proc.SetModeration(&fakeModerator{decision: decision}, modStore)

	if err := proc.process(t.Context(), ImageProcessPayload{
		UploadID: "u-nsfw", StorageKey: key, ContentType: "image/png",
	}); err != nil {
		t.Fatalf("process error: %v", err)
	}
	if !modStore.rejectedRetained {
		t.Error("nsfw rejection should retain the storage object")
	}
	if _, ok := st.objects[key]; !ok {
		t.Error("storage object should still be present after nsfw rejection")
	}
	if modStore.rejectedScore != 0.94 {
		t.Errorf("rejected score = %v, want 0.94", modStore.rejectedScore)
	}
	if len(modStore.rejectedCategories) != 2 {
		t.Errorf("expected 2 categories, got %v", modStore.rejectedCategories)
	}
}

func TestProcess_QuarantineDispositionPreservesObject(t *testing.T) {
	// A CSAM-class hit must preserve the object under the restricted prefix
	// and remove the original — never delete outright.
	key := "album-private/user/photo.png"
	imgData := makePNG(t, 200, 200)
	st := newFakeStorage(key, string(imgData))
	store := &fakeStore{}
	modStore := &fakeModerationStore{}

	proc := NewImageProcessor(st, store, 0, zerolog.Nop())
	proc.SetModeration(&fakeModerator{
		decision: moderation.RejectWith(moderation.CodeHashMatch, "csam match", "photodna", moderation.DispositionQuarantine),
	}, modStore)

	if err := proc.process(t.Context(), ImageProcessPayload{
		UploadID: "u-q", StorageKey: key, ContentType: "image/png",
	}); err != nil {
		t.Fatalf("process error: %v", err)
	}
	if modStore.quarantinedID != "u-q" {
		t.Errorf("expected MarkQuarantined(u-q), got %q", modStore.quarantinedID)
	}
	if _, ok := st.objects[key]; ok {
		t.Error("original should be removed after quarantine")
	}
	if _, ok := st.objects[quarantineKeyFor(key)]; !ok {
		t.Error("object should be preserved under the quarantine prefix")
	}
	if store.thumbnailKey != "" {
		t.Error("thumbnail should NOT be generated for a quarantined upload")
	}
}

func TestProcess_QuarantineFailsTaskWhenDeleteFails(t *testing.T) {
	// If the original object can't be removed, the task must fail (so asynq
	// retries) rather than mark the row terminal — leaving a CSAM-class object
	// reachable at its key would violate the "never served" contract.
	key := "album-private/user/photo.png"
	imgData := makePNG(t, 200, 200)
	st := newFakeStorage(key, string(imgData))
	st.delErr = errors.New("storage delete failed")
	store := &fakeStore{}
	modStore := &fakeModerationStore{}

	proc := NewImageProcessor(st, store, 0, zerolog.Nop())
	proc.SetModeration(&fakeModerator{
		decision: moderation.RejectWith(moderation.CodeHashMatch, "csam match", "photodna", moderation.DispositionQuarantine),
	}, modStore)

	err := proc.process(t.Context(), ImageProcessPayload{
		UploadID: "u-qf", StorageKey: key, ContentType: "image/png",
	})
	if err == nil {
		t.Fatal("expected process to fail so asynq retries")
	}
	if modStore.quarantinedID != "" {
		t.Error("must not mark quarantined when the original could not be deleted")
	}
}

func TestProcess_HardModerationFailureHoldsForRetry(t *testing.T) {
	// A SeverityHard detector that errors must fail the task (asynq retries)
	// and leave the object untouched and unapproved — never served.
	key := "album-private/user/photo.png"
	imgData := makePNG(t, 200, 200)
	st := newFakeStorage(key, string(imgData))
	store := &fakeStore{}
	modStore := &fakeModerationStore{}

	proc := NewImageProcessor(st, store, 0, zerolog.Nop())
	proc.SetModeration(&fakeModerator{
		err: errors.New("photodna unreachable"),
		sev: moderation.SeverityHard,
	}, modStore)

	err := proc.process(t.Context(), ImageProcessPayload{
		UploadID: "u-h", StorageKey: key, ContentType: "image/png",
	})
	if err == nil {
		t.Fatal("expected process to fail so asynq retries")
	}
	if modStore.approvedID != "" {
		t.Error("a held upload must not be approved")
	}
	if _, ok := st.objects[key]; !ok {
		t.Error("original must remain untouched while held for retry")
	}
}
