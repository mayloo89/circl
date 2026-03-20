package storage

import (
	"context"
	"errors"
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

	// Captured arguments for assertions.
	lastBucket  string
	lastKey     string
	lastExpires time.Duration
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
