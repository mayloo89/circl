package exports

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/rs/zerolog"
)

// MediaStorage is the subset of the storage interface the export builder
// needs. Narrow on purpose so the builder is easy to fake in tests.
type MediaStorage interface {
	GetObject(ctx context.Context, key string) (io.ReadCloser, error)
	PutObject(ctx context.Context, key, contentType string, r io.Reader, size int64) error
}

// Mailer is the subset of email.Sender used by this package, narrowed so the
// builder doesn't pull the email package into tests.
type Mailer interface {
	SendReadyEmail(ctx context.Context, toEmail, downloadURL string, expiresAt time.Time) error
	SendFailedEmail(ctx context.Context, toEmail string) error
}

// Enqueuer is satisfied by anything that can ask the asynq worker pool to
// schedule a build. main.go wires this to worker.EnqueueExportUser.
type Enqueuer func(ctx context.Context, requestID, userID string) error

// Service orchestrates the request → build → notify flow.
type Service struct {
	store           Store
	source          Source
	storage         MediaStorage
	mailer          Mailer
	enqueue         Enqueuer
	prefix          string
	downloadURLBase string
	log             zerolog.Logger
}

// Config holds Service dependencies. Some are optional (mailer can be nil in
// tests, enqueue can be nil for tests that build inline).
type Config struct {
	Store           Store
	Source          Source
	Storage         MediaStorage
	Mailer          Mailer
	Enqueue         Enqueuer
	StoragePrefix   string // e.g. "exports/" — zip files are written here
	DownloadURLBase string // e.g. "https://api.circl.app/account/export" — token is appended
	Log             zerolog.Logger
}

// NewService returns a Service.
func NewService(cfg Config) *Service {
	prefix := cfg.StoragePrefix
	if prefix == "" {
		prefix = "exports/"
	}
	return &Service{
		store:           cfg.Store,
		source:          cfg.Source,
		storage:         cfg.Storage,
		mailer:          cfg.Mailer,
		enqueue:         cfg.Enqueue,
		prefix:          prefix,
		downloadURLBase: cfg.DownloadURLBase,
		log:             cfg.Log.With().Str("component", "exports").Logger(),
	}
}

// Request creates a new export request and enqueues the build job. Returns
// ErrAlreadyPending or ErrRateLimited when the user can't request right now.
func (s *Service) Request(ctx context.Context, userID string) (*Request, error) {
	r, err := s.store.Create(ctx, userID)
	if err != nil {
		return nil, err
	}
	if s.enqueue != nil {
		if err := s.enqueue(ctx, r.ID, userID); err != nil {
			s.log.Error().Err(err).Str("request_id", r.ID).Msg("enqueue export task failed")
			// Mark failed so the user sees an actionable status rather than a
			// row stuck in 'pending' forever.
			if mErr := s.store.MarkFailed(ctx, r.ID, "enqueue failed"); mErr != nil {
				s.log.Error().Err(mErr).Str("request_id", r.ID).Msg("mark failed after enqueue error")
			}
			return nil, err
		}
	}
	return r, nil
}

// Status returns the user's latest request as a redacted view.
func (s *Service) Status(ctx context.Context, userID string) (*PublicStatus, error) {
	r, err := s.store.LatestForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &PublicStatus{
		ID:          r.ID,
		Status:      r.Status,
		RequestedAt: r.RequestedAt,
		CompletedAt: r.CompletedAt,
		ExpiresAt:   r.ExpiresAt,
	}, nil
}

// Build runs the actual export: queries the source, zips the data + media,
// uploads the zip, mints the download token, stamps the request, and emails
// the user. Called by the asynq worker.
//
// Returned errors are logged at the call site; the row is also marked failed
// here so a re-queue won't double-up the user-visible state.
func (s *Service) Build(ctx context.Context, requestID string) (err error) {
	req, err := s.store.GetByID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("get request: %w", err)
	}
	if req.Status != StatusPending && req.Status != StatusProcessing {
		// Already done — nothing to do.
		return nil
	}
	if err := s.store.MarkProcessing(ctx, requestID); err != nil && !errors.Is(err, ErrNotFound) {
		// ErrNotFound means another worker already grabbed it; fine.
		return fmt.Errorf("mark processing: %w", err)
	}

	defer func() {
		if err == nil {
			return
		}
		if mErr := s.store.MarkFailed(ctx, requestID, err.Error()); mErr != nil {
			s.log.Error().Err(mErr).Str("request_id", requestID).Msg("mark failed after build error")
		}
		if s.mailer != nil {
			if mErr := s.notifyFailed(ctx, req.UserID); mErr != nil {
				s.log.Warn().Err(mErr).Str("request_id", requestID).Msg("send failure email failed")
			}
		}
	}()

	bundle, err := s.source.BuildBundle(ctx, req.UserID)
	if err != nil {
		return fmt.Errorf("build bundle: %w", err)
	}

	zipBytes, err := s.assembleZip(ctx, bundle)
	if err != nil {
		return fmt.Errorf("assemble zip: %w", err)
	}

	storageKey := fmt.Sprintf("%s%s/%s.zip", s.prefix, req.UserID, requestID)
	if err := s.storage.PutObject(ctx, storageKey, "application/zip", bytes.NewReader(zipBytes), int64(len(zipBytes))); err != nil {
		return fmt.Errorf("upload zip: %w", err)
	}

	plain, hash, err := GenerateToken()
	if err != nil {
		return fmt.Errorf("generate token: %w", err)
	}
	expiresAt := time.Now().Add(TTL)
	if err := s.store.MarkReady(ctx, requestID, storageKey, hash, expiresAt); err != nil {
		return fmt.Errorf("mark ready: %w", err)
	}

	if s.mailer != nil {
		downloadURL := s.downloadURLBase + "/" + plain
		if mErr := s.mailer.SendReadyEmail(ctx, bundle.Snapshot.Account.Email, downloadURL, expiresAt); mErr != nil {
			s.log.Warn().Err(mErr).Str("request_id", requestID).Msg("send ready email failed")
		}
	}
	return nil
}

// Download looks up an export by its plaintext token and returns the request
// + a ReadCloser to the zip. The caller is responsible for closing the
// reader; this method also stamps downloaded_at.
func (s *Service) Download(ctx context.Context, plainToken string) (*Request, io.ReadCloser, error) {
	if plainToken == "" {
		return nil, nil, ErrInvalidToken
	}
	r, err := s.store.GetByTokenHash(ctx, HashToken(plainToken))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil, ErrInvalidToken
		}
		return nil, nil, err
	}
	if r.Status != StatusReady {
		return nil, nil, ErrNotReady
	}
	if r.ExpiresAt != nil && r.ExpiresAt.Before(time.Now()) {
		return nil, nil, ErrInvalidToken
	}
	if r.StorageKey == nil || *r.StorageKey == "" {
		return nil, nil, ErrNotReady
	}
	rc, err := s.storage.GetObject(ctx, *r.StorageKey)
	if err != nil {
		return nil, nil, fmt.Errorf("get zip: %w", err)
	}
	if mErr := s.store.MarkDownloaded(ctx, r.ID); mErr != nil {
		s.log.Warn().Err(mErr).Str("request_id", r.ID).Msg("mark downloaded failed")
	}
	return r, rc, nil
}

// assembleZip writes data.json plus the media manifest into a zip buffer.
// Each media item is best-effort: a fetch failure logs and continues so a
// single missing file does not abort the entire export.
func (s *Service) assembleZip(ctx context.Context, b *Bundle) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	jsonBytes, err := json.MarshalIndent(b.Snapshot, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal snapshot: %w", err)
	}
	jw, err := zw.Create("data.json")
	if err != nil {
		return nil, fmt.Errorf("zip create data.json: %w", err)
	}
	if _, err := jw.Write(jsonBytes); err != nil {
		return nil, fmt.Errorf("zip write data.json: %w", err)
	}

	seen := make(map[string]bool, len(b.Media))
	for _, m := range b.Media {
		if m.StorageKey == "" || seen[m.ArchivePath] {
			continue
		}
		seen[m.ArchivePath] = true
		if err := s.writeMedia(ctx, zw, m); err != nil {
			s.log.Warn().Err(err).Str("storage_key", m.StorageKey).Msg("export: media skipped")
		}
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}
	return buf.Bytes(), nil
}

func (s *Service) writeMedia(ctx context.Context, zw *zip.Writer, m MediaItem) error {
	rc, err := s.storage.GetObject(ctx, m.StorageKey)
	if err != nil {
		return fmt.Errorf("get %s: %w", m.StorageKey, err)
	}
	defer func() { _ = rc.Close() }()
	w, err := zw.Create(m.ArchivePath)
	if err != nil {
		return fmt.Errorf("zip create %s: %w", m.ArchivePath, err)
	}
	if _, err := io.Copy(w, rc); err != nil {
		return fmt.Errorf("zip copy %s: %w", m.ArchivePath, err)
	}
	return nil
}

func (s *Service) notifyFailed(ctx context.Context, userID string) error {
	// Re-fetch the email from the source since the bundle build failed.
	bundle, err := s.source.BuildBundle(ctx, userID)
	if err != nil {
		return err
	}
	return s.mailer.SendFailedEmail(ctx, bundle.Snapshot.Account.Email)
}
