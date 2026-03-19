package storage

import (
	"context"
	"fmt"
	"net/url" //nolint:depguard // used for *url.URL return type in minioClient interface
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const presignedURLTTL = 15 * time.Minute

// minioClient is the subset of the MinIO client API used by S3Storage.
// It exists so that tests can inject a fake without a real MinIO server.
type minioClient interface {
	PresignedPutObject(ctx context.Context, bucket, key string, expires time.Duration) (*url.URL, error)
	RemoveObject(ctx context.Context, bucket, key string, opts minio.RemoveObjectOptions) error
}

// S3Config holds the configuration for an S3-compatible storage provider.
type S3Config struct {
	// Endpoint is the host (and optional port) of the S3-compatible server,
	// e.g. "localhost:9000" or "s3.amazonaws.com".
	Endpoint string

	// AccessKey and SecretKey are the credentials for the storage provider.
	AccessKey string
	SecretKey string

	// Bucket is the name of the bucket where files are stored.
	Bucket string

	// PublicURL is the base URL used to build public file URLs.
	// For a public MinIO bucket: "http://localhost:9000/circl-media".
	// For CloudFront or similar CDN: "https://cdn.example.com".
	PublicURL string

	// UseSSL controls whether TLS is used when connecting to the endpoint.
	UseSSL bool
}

// S3Storage implements Storage using any S3-compatible provider
// (MinIO, AWS S3, Cloudflare R2, DigitalOcean Spaces, etc.).
type S3Storage struct {
	client    minioClient
	bucket    string
	publicURL string
}

// NewS3Storage creates an S3Storage backed by the provided configuration.
// It returns an error if the MinIO client cannot be initialised.
func NewS3Storage(cfg S3Config) (*S3Storage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("s3 storage: create client: %w", err)
	}
	return newS3StorageWithClient(client, cfg.Bucket, cfg.PublicURL), nil
}

// newS3StorageWithClient is the internal constructor used by tests to inject a
// fake MinIO client.
func newS3StorageWithClient(client minioClient, bucket, publicURL string) *S3Storage {
	return &S3Storage{
		client:    client,
		bucket:    bucket,
		publicURL: strings.TrimRight(publicURL, "/"),
	}
}

// GenerateUploadURL returns a pre-signed PUT URL the client can use to upload
// a file directly to the storage provider.
func (s *S3Storage) GenerateUploadURL(ctx context.Context, params UploadParams) (*UploadResult, error) {
	u, err := s.client.PresignedPutObject(ctx, s.bucket, params.Key, presignedURLTTL)
	if err != nil {
		return nil, fmt.Errorf("s3 storage: presign: %w", err)
	}

	exp := time.Now().Add(presignedURLTTL)
	return &UploadResult{
		UploadURL: u.String(),
		ExpiresAt: &exp,
	}, nil
}

// PublicURL returns the serving URL for a stored file.
func (s *S3Storage) PublicURL(key string) string {
	return s.publicURL + "/" + key
}

// Delete removes a file from the bucket.
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("s3 storage: delete %q: %w", key, err)
	}
	return nil
}
