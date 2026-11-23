// Package evidence keeps copies of reported media under the never-served
// storage.EvidencePrefix, tied to the lifetime of their report.
//
// A report freezes its media when it is filed: the attachments of a reported
// message and, for critical-priority reports, the reported account's profile
// photos. Retention, unsend, view-once expiry and account purge all delete the
// originals, so without a copy the evidence would vanish while the report is
// still open. A dismissal discards the copies at once; a sweep discards them a
// retention period after a report is reviewed.
package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"time"

	"github.com/google/uuid"

	"github.com/mayloo89/circl/backend/internal/reports"
	"github.com/mayloo89/circl/backend/internal/storage"
)

// File kinds.
const (
	KindMessageMedia = "message_media"
	KindProfilePhoto = "profile_photo"
)

// DefaultRetentionDays is how long a reviewed report keeps its evidence. It
// matches the one-year preservation period US providers owe NCMEC reports
// (18 U.S.C. § 2258A as amended by the REPORT Act); operators elsewhere set
// REPORT_EVIDENCE_RETENTION_DAYS to their own requirement.
const DefaultRetentionDays = 365

// ErrNotFound is returned when a report has no evidence file with that ID.
var ErrNotFound = errors.New("evidence file not found")

// Source is an original object a copy is taken from.
type Source struct {
	UploadID    string
	Kind        string
	Key         string
	ContentType string
	SizeBytes   int64
}

// File is a preserved copy.
type File struct {
	ID          string    `json:"id"`
	ReportID    string    `json:"report_id"`
	Kind        string    `json:"kind"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	SHA256      string    `json:"sha256"`
	CreatedAt   time.Time `json:"created_at"`
	StorageKey  string    `json:"-"`
	// ReportReason gates viewing: CSAM copies are kept for law enforcement and
	// are never streamed to the admin panel.
	ReportReason   string `json:"-"`
	SourceUploadID string `json:"-"`
}

// Viewable reports whether the admin panel may stream the file's bytes.
func (f *File) Viewable() bool { return f.ReportReason != reports.ReasonCSAM }

// Store persists evidence metadata and finds the originals to copy.
type Store interface {
	// MessageMedia returns the attachments of a message, including one already
	// moved to quarantine by the moderation pipeline.
	MessageMedia(ctx context.Context, messageID string) ([]Source, error)
	// ProfilePhotos returns the avatar and gallery uploads a profile currently
	// shows. urlPrefix is the storage public URL prefix the profile URLs carry.
	ProfilePhotos(ctx context.Context, userID, urlPrefix string) ([]Source, error)
	Record(ctx context.Context, f File) error
	List(ctx context.Context, reportID string) ([]File, error)
	Get(ctx context.Context, reportID, fileID string) (*File, error)
	Delete(ctx context.Context, fileID string) error
	// DueForPurge returns reports whose evidence must go: dismissed ones still
	// holding any, and reviewed ones older than cutoff.
	DueForPurge(ctx context.Context, cutoff time.Time, limit int) ([]string, error)
	// MarkPurged wipes a report's message text copy and stamps the purge.
	MarkPurged(ctx context.Context, reportID string) error
}

// ObjectStorage is the subset of storage.Storage the service needs.
type ObjectStorage interface {
	GetObject(ctx context.Context, key string) (io.ReadCloser, error)
	PutObject(ctx context.Context, key, contentType string, r io.Reader, size int64) error
	Delete(ctx context.Context, key string) error
}

// Service implements reports.EvidenceKeeper.
type Service struct {
	store         Store
	storage       ObjectStorage
	urlPrefix     string
	retentionDays int
	now           func() time.Time
}

// Config configures a Service.
type Config struct {
	Store   Store
	Storage ObjectStorage
	// URLPrefix is storage.PublicURL("") — what profile photo URLs start with.
	URLPrefix     string
	RetentionDays int
}

// NewService returns a Service. A non-positive RetentionDays falls back to
// DefaultRetentionDays.
func NewService(cfg Config) *Service {
	return &Service{
		store:         cfg.Store,
		storage:       cfg.Storage,
		urlPrefix:     cfg.URLPrefix,
		retentionDays: cmpPositive(cfg.RetentionDays, DefaultRetentionDays),
		now:           time.Now,
	}
}

func cmpPositive(v, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}

// Preserve copies a new report's media into evidence storage. Each copy is
// independent: one missing original does not stop the rest, and every failure
// is returned joined.
func (s *Service) Preserve(ctx context.Context, r *reports.Report) error {
	var sources []Source
	var errs []error
	if r.MessageID != nil {
		media, err := s.store.MessageMedia(ctx, *r.MessageID)
		if err != nil {
			errs = append(errs, fmt.Errorf("evidence: message media: %w", err))
		}
		sources = append(sources, media...)
	}
	if r.Priority == reports.PriorityCritical && r.ReportedUserID != nil {
		photos, err := s.store.ProfilePhotos(ctx, *r.ReportedUserID, s.urlPrefix)
		if err != nil {
			errs = append(errs, fmt.Errorf("evidence: profile photos: %w", err))
		}
		sources = append(sources, photos...)
	}
	for _, src := range sources {
		if err := s.copy(ctx, r, src); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *Service) copy(ctx context.Context, r *reports.Report, src Source) error {
	rc, err := s.storage.GetObject(ctx, src.Key)
	if err != nil {
		return fmt.Errorf("evidence: read %q: %w", src.Key, err)
	}
	defer func() { _ = rc.Close() }()

	id := uuid.NewString()
	key := storage.EvidencePrefix + r.ID + "/" + id + path.Ext(src.Key)
	hash := sha256.New()
	var written byteCounter
	body := io.TeeReader(rc, io.MultiWriter(hash, &written))
	if err := s.storage.PutObject(ctx, key, src.ContentType, body, src.SizeBytes); err != nil {
		return fmt.Errorf("evidence: write %q: %w", key, err)
	}
	f := File{
		ID:             id,
		ReportID:       r.ID,
		Kind:           src.Kind,
		ContentType:    src.ContentType,
		SizeBytes:      int64(written),
		SHA256:         hex.EncodeToString(hash.Sum(nil)),
		StorageKey:     key,
		SourceUploadID: src.UploadID,
	}
	if err := s.store.Record(ctx, f); err != nil {
		// An object no row points at would never be discarded.
		_ = s.storage.Delete(ctx, key)
		return fmt.Errorf("evidence: record %q: %w", key, err)
	}
	return nil
}

// Discard deletes every copy a report holds. A row is removed only after its
// object is gone, so a failed delete is retried by the next sweep.
func (s *Service) Discard(ctx context.Context, reportID string) error {
	files, err := s.store.List(ctx, reportID)
	if err != nil {
		return fmt.Errorf("evidence: list: %w", err)
	}
	var errs []error
	for _, f := range files {
		if err := s.storage.Delete(ctx, f.StorageKey); err != nil && !errors.Is(err, storage.ErrNotFound) {
			errs = append(errs, fmt.Errorf("evidence: delete %q: %w", f.StorageKey, err))
			continue
		}
		if err := s.store.Delete(ctx, f.ID); err != nil {
			errs = append(errs, fmt.Errorf("evidence: delete row %s: %w", f.ID, err))
		}
	}
	return errors.Join(errs...)
}

// List returns a report's preserved files.
func (s *Service) List(ctx context.Context, reportID string) ([]File, error) {
	return s.store.List(ctx, reportID)
}

// Open returns a preserved file and a reader over its bytes. The caller must
// check File.Viewable before streaming, and close the reader.
func (s *Service) Open(ctx context.Context, reportID, fileID string) (*File, io.ReadCloser, error) {
	f, err := s.store.Get(ctx, reportID, fileID)
	if err != nil {
		return nil, nil, err
	}
	if !f.Viewable() {
		return f, nil, nil
	}
	rc, err := s.storage.GetObject(ctx, f.StorageKey)
	if err != nil {
		return nil, nil, fmt.Errorf("evidence: open %q: %w", f.StorageKey, err)
	}
	return f, rc, nil
}

// SweepResult summarises one sweep.
type SweepResult struct {
	Purged int
	Failed int
}

// Sweep discards the evidence of reports that no longer need it, in batches,
// and stamps each one purged once all its copies are gone.
func (s *Service) Sweep(ctx context.Context, batch int) (SweepResult, error) {
	var res SweepResult
	cutoff := s.now().AddDate(0, 0, -s.retentionDays)
	ids, err := s.store.DueForPurge(ctx, cutoff, batch)
	if err != nil {
		return res, fmt.Errorf("evidence: due for purge: %w", err)
	}
	var errs []error
	for _, id := range ids {
		if err := s.Discard(ctx, id); err != nil {
			res.Failed++
			errs = append(errs, err)
			continue
		}
		if err := s.store.MarkPurged(ctx, id); err != nil {
			res.Failed++
			errs = append(errs, fmt.Errorf("evidence: mark purged %s: %w", id, err))
			continue
		}
		res.Purged++
	}
	return res, errors.Join(errs...)
}

// byteCounter counts what was actually copied, so the recorded size and hash
// describe the same bytes even if the upload row's size was off.
type byteCounter int64

func (c *byteCounter) Write(p []byte) (int, error) {
	*c += byteCounter(len(p))
	return len(p), nil
}
