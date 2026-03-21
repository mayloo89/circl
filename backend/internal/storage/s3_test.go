package storage

import (
	"context"
	"errors"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
)

// fakeMinioClient implements minioClient for tests.
type fakeMinioClient struct {
	presignURL string
	presignErr error
	removeErr  error
	getContent string
	getErr     error
	putErr     error

	// Captured arguments for assertions.
	lastBucket      string
	lastKey         string
	lastExpires     time.Duration
	lastPutKey      string
	lastPutData     []byte
	lastPutType     string
}

func (f *fakeMinioClient) PresignedPutObject(_ context.Context, bucket, key string, expires time.Duration) (*url.URL, error) {
	f.lastBucket = bucket
	f.lastKey = key
	f.lastExpires = expires
	if f.presignErr != nil {
		return nil, f.presignErr
	}
	u, _ := url.Parse(f.presignURL)
	return u, nil
}

func (f *fakeMinioClient) RemoveObject(_ context.Context, bucket, key string, _ minio.RemoveObjectOptions) error {
	f.lastBucket = bucket
	f.lastKey = key
	return f.removeErr
}

func (f *fakeMinioClient) GetObject(_ context.Context, _, key string) (io.ReadCloser, error) {
	f.lastKey = key
	if f.getErr != nil {
		return nil, f.getErr
	}
	return io.NopCloser(strings.NewReader(f.getContent)), nil
}

func (f *fakeMinioClient) PutObject(_ context.Context, bucket, key string, r io.Reader, _ int64, contentType string) error {
	f.lastBucket = bucket
	f.lastPutKey = key
	f.lastPutType = contentType
	if f.putErr != nil {
		return f.putErr
	}
	data, _ := io.ReadAll(r)
	f.lastPutData = data
	return nil
}

func newTestS3(client *fakeMinioClient) *S3Storage {
	return newS3StorageWithClient(client, "test-bucket", "http://localhost:9000/test-bucket")
}

// --- GenerateUploadURL ---

func TestS3_GenerateUploadURL_Success(t *testing.T) {
	fake := &fakeMinioClient{presignURL: "http://localhost:9000/test-bucket/avatar/u-1/file.jpg?X-Amz-Signature=abc"}
	s := newTestS3(fake)

	result, err := s.GenerateUploadURL(context.Background(), UploadParams{
		Key:         "avatar/u-1/file.jpg",
		ContentType: "image/jpeg",
		MaxSize:     5 << 20,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UploadURL != fake.presignURL {
		t.Errorf("UploadURL = %q, want %q", result.UploadURL, fake.presignURL)
	}
	if result.ExpiresAt == nil {
		t.Error("ExpiresAt should not be nil")
	}
	if fake.lastBucket != "test-bucket" {
		t.Errorf("bucket = %q, want test-bucket", fake.lastBucket)
	}
	if fake.lastKey != "avatar/u-1/file.jpg" {
		t.Errorf("key = %q, want avatar/u-1/file.jpg", fake.lastKey)
	}
	if fake.lastExpires != presignedURLTTL {
		t.Errorf("expires = %v, want %v", fake.lastExpires, presignedURLTTL)
	}
}

func TestS3_GenerateUploadURL_PresignError(t *testing.T) {
	fake := &fakeMinioClient{presignErr: errors.New("minio down")}
	s := newTestS3(fake)

	_, err := s.GenerateUploadURL(context.Background(), UploadParams{Key: "k", ContentType: "image/jpeg"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "presign") {
		t.Errorf("error = %q, want it to mention presign", err.Error())
	}
}

// --- PublicURL ---

func TestS3_PublicURL(t *testing.T) {
	s := newS3StorageWithClient(&fakeMinioClient{}, "bucket", "https://cdn.example.com")
	got := s.PublicURL("avatar/u-1/photo.jpg")
	want := "https://cdn.example.com/avatar/u-1/photo.jpg"
	if got != want {
		t.Errorf("PublicURL = %q, want %q", got, want)
	}
}

func TestS3_PublicURL_TrailingSlashStripped(t *testing.T) {
	s := newS3StorageWithClient(&fakeMinioClient{}, "bucket", "https://cdn.example.com/")
	got := s.PublicURL("some/file.pdf")
	if strings.Contains(got, "//some") {
		t.Errorf("PublicURL has double slash: %q", got)
	}
}

// --- Delete ---

func TestS3_Delete_Success(t *testing.T) {
	fake := &fakeMinioClient{}
	s := newTestS3(fake)

	if err := s.Delete(context.Background(), "avatar/u-1/photo.jpg"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.lastBucket != "test-bucket" {
		t.Errorf("bucket = %q, want test-bucket", fake.lastBucket)
	}
	if fake.lastKey != "avatar/u-1/photo.jpg" {
		t.Errorf("key = %q, want avatar/u-1/photo.jpg", fake.lastKey)
	}
}

func TestS3_Delete_Error(t *testing.T) {
	fake := &fakeMinioClient{removeErr: errors.New("bucket not found")}
	s := newTestS3(fake)

	err := s.Delete(context.Background(), "some/key")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "delete") {
		t.Errorf("error = %q, want it to mention delete", err.Error())
	}
}

// --- GetObject ---

func TestS3_GetObject_Success(t *testing.T) {
	fake := &fakeMinioClient{getContent: "hello world"}
	s := newTestS3(fake)

	rc, err := s.GetObject(context.Background(), "chat-attachment/u/file.jpg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer rc.Close()
	data, _ := io.ReadAll(rc)
	if string(data) != "hello world" {
		t.Errorf("data = %q, want %q", data, "hello world")
	}
	if fake.lastKey != "chat-attachment/u/file.jpg" {
		t.Errorf("key = %q, want chat-attachment/u/file.jpg", fake.lastKey)
	}
}

func TestS3_GetObject_Error(t *testing.T) {
	fake := &fakeMinioClient{getErr: errors.New("not found")}
	s := newTestS3(fake)

	_, err := s.GetObject(context.Background(), "missing/key")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "get") {
		t.Errorf("error = %q, want it to mention get", err.Error())
	}
}

// --- PutObject ---

func TestS3_PutObject_Success(t *testing.T) {
	fake := &fakeMinioClient{}
	s := newTestS3(fake)

	body := strings.NewReader("image data")
	if err := s.PutObject(context.Background(), "thumbnails/chat-attachment/u/file.jpg", "image/jpeg", body, 10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.lastBucket != "test-bucket" {
		t.Errorf("bucket = %q, want test-bucket", fake.lastBucket)
	}
	if fake.lastPutKey != "thumbnails/chat-attachment/u/file.jpg" {
		t.Errorf("key = %q, want thumbnails/chat-attachment/u/file.jpg", fake.lastPutKey)
	}
	if fake.lastPutType != "image/jpeg" {
		t.Errorf("content type = %q, want image/jpeg", fake.lastPutType)
	}
	if string(fake.lastPutData) != "image data" {
		t.Errorf("data = %q, want image data", fake.lastPutData)
	}
}

func TestS3_PutObject_Error(t *testing.T) {
	fake := &fakeMinioClient{putErr: errors.New("write failed")}
	s := newTestS3(fake)

	err := s.PutObject(context.Background(), "some/key", "image/jpeg", strings.NewReader("x"), 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "put") {
		t.Errorf("error = %q, want it to mention put", err.Error())
	}
}
