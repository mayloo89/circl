package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"path"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"golang.org/x/image/draw"
	"golang.org/x/image/webp"

	"github.com/mayloo89/circl/backend/internal/moderation"
	"github.com/mayloo89/circl/backend/internal/uploads"
)

// TaskProcessImage is the task type name for background image processing.
const TaskProcessImage = "image:process"

const (
	thumbnailMaxPx     = 480
	defaultImageMaxPx  = 1024
)

// ImageProcessPayload carries the data needed to process an uploaded image.
//
// Category mirrors uploads.category and selects the moderation context —
// `album-private` uploads run through the chain with `ContextPrivate`,
// which makes the NSFW detector tag-not-block. Empty falls back to public
// semantics so callers that haven't been updated keep their old behaviour.
type ImageProcessPayload struct {
	UploadID    string `json:"upload_id"`
	StorageKey  string `json:"storage_key"`
	ContentType string `json:"content_type"`
	Category    string `json:"category,omitempty"`
}

// ProcessingStorage is the subset of the storage interface required by the
// image worker. Using a narrow interface keeps the worker decoupled from the
// full Storage contract and makes tests simpler.
type ProcessingStorage interface {
	GetObject(ctx context.Context, key string) (io.ReadCloser, error)
	PutObject(ctx context.Context, key, contentType string, r io.Reader, size int64) error
	Delete(ctx context.Context, key string) error
}

// ThumbnailStore is the minimal persistence interface the worker needs.
type ThumbnailStore interface {
	SetThumbnailKey(ctx context.Context, id, thumbnailKey string) error
}

// ModerationStore is the optional interface for recording moderation
// outcomes. Wired in via SetModeration; absent in tests that don't need it.
type ModerationStore interface {
	MarkApproved(ctx context.Context, uploadID string) error
	MarkRejected(ctx context.Context, rec uploads.RejectionRecord) error
	// MarkQuarantined records a CSAM-class hit whose object the worker has
	// moved to the restricted quarantineKey (preserved, never served).
	MarkQuarantined(ctx context.Context, rec uploads.RejectionRecord, quarantineKey string) error
}

// ImageProcessor handles the image:process task.
type ImageProcessor struct {
	storage    ProcessingStorage
	store      ThumbnailStore
	imageMaxPx int
	log        zerolog.Logger
	moderator  moderation.Moderator // nil disables async moderation
	modStore   ModerationStore      // nil disables async moderation
}

// NewImageProcessor creates an ImageProcessor. imageMaxPx caps the longest
// edge of JPEG and PNG originals; use 0 to apply the default (1024 px).
func NewImageProcessor(st ProcessingStorage, store ThumbnailStore, imageMaxPx int, log zerolog.Logger) *ImageProcessor {
	if imageMaxPx <= 0 {
		imageMaxPx = defaultImageMaxPx
	}
	return &ImageProcessor{
		storage:    st,
		store:      store,
		imageMaxPx: imageMaxPx,
		log:        log.With().Str("component", "image_worker").Logger(),
	}
}

// SetModeration wires the async moderation step. When both args are non-nil,
// every processed image runs through the moderator after decoding; rejections
// delete the storage object and mark the upload via modStore.
func (p *ImageProcessor) SetModeration(m moderation.Moderator, store ModerationStore) {
	p.moderator = m
	p.modStore = store
}

// EnqueueProcessImage enqueues a process-image task using the given client.
// The current span context from ctx is embedded in the payload so the worker
// can link its execution span to the HTTP request that triggered the upload.
func EnqueueProcessImage(ctx context.Context, client *Client, p ImageProcessPayload) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("worker: marshal payload: %w", err)
	}
	wrapped, err := InjectTraceContext(ctx, raw)
	if err != nil {
		return fmt.Errorf("worker: inject trace context: %w", err)
	}
	// MaxRetry bounds the "hold & retry" window for hard moderation failures
	// (CSAM / NCII vendor outage). At the 5-minute retry-delay cap this is
	// ~4 hours of re-checks; after that the task is archived but the upload
	// stays moderation_status='pending' and is never served — failing closed.
	_, err = client.c.EnqueueContext(ctx, asynq.NewTask(TaskProcessImage, wrapped), asynq.MaxRetry(48))
	return err
}

// Handle processes an image:process task. It is called by the asynq server.
func (p *ImageProcessor) Handle(ctx context.Context, t *asynq.Task) error {
	var payload ImageProcessPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("worker: unmarshal payload: %w", err)
	}
	if err := p.process(ctx, payload); err != nil {
		p.log.Error().Err(err).Str("upload_id", payload.UploadID).Msg("process image failed")
		return err
	}
	return nil
}

func (p *ImageProcessor) process(ctx context.Context, payload ImageProcessPayload) error {
	rc, err := p.storage.GetObject(ctx, payload.StorageKey)
	if err != nil {
		return fmt.Errorf("get object: %w", err)
	}
	defer rc.Close()

	raw, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("read object: %w", err)
	}

	img, err := decodeImage(payload.ContentType, raw)
	if err != nil {
		return fmt.Errorf("decode image: %w", err)
	}

	// Async moderation runs after decode (so we have dimensions) and before
	// any further storage I/O. A rejection disposes of the object per the
	// decision (retain / purge / quarantine), marks the row, and short-circuits.
	//
	// Error handling is severity-aware: a SeverityHard detector (CSAM / NCII /
	// operator block list) that errors causes the task to FAIL so asynq retries
	// — the upload stays unapproved (never served) until a check completes. A
	// SeveritySoft detector (NSFW / heuristic) that errors fails open, because
	// a flaky classifier must never block legitimate uploads.
	if p.moderator != nil && p.modStore != nil {
		bounds := img.Bounds()
		decision, modErr := p.moderator.Check(ctx, moderation.Input{
			ContentType: payload.ContentType,
			SizeBytes:   int64(len(raw)),
			Width:       bounds.Dx(),
			Height:      bounds.Dy(),
			Hash:        moderation.HashFor(raw),
			Bytes:       raw,
			Context:     moderationContextFor(payload.Category),
		})
		if modErr != nil {
			if isHardModerationFailure(p.moderator, modErr) {
				// Fail closed: return the error so asynq retries with backoff.
				// The row's moderation_status stays 'pending', so the serve
				// layer never exposes the object while a legal-floor check is
				// unresolved.
				p.log.Error().Err(modErr).Str("upload_id", payload.UploadID).Msg("hard moderation failure; holding for retry")
				return fmt.Errorf("hard moderation failure: %w", modErr)
			}
			p.log.Warn().Err(modErr).Str("upload_id", payload.UploadID).Msg("soft moderator error; failing open")
		} else if !decision.Allowed {
			if err := p.disposeRejected(ctx, payload, raw, decision); err != nil {
				return err
			}
			return nil
		} else if markErr := p.modStore.MarkApproved(ctx, payload.UploadID); markErr != nil {
			// Approval is a soft signal — log and continue with thumbnail.
			p.log.Warn().Err(markErr).Str("upload_id", payload.UploadID).Msg("mark approved failed")
		}
	}

	// For JPEG and PNG: resize if oversized, then re-encode to strip all
	// metadata (EXIF, XMP, ICC profiles) — Go's image codec only carries pixel
	// data. WebP and GIF originals are left untouched (no encoder available
	// in the standard library); their thumbnails are still capped below.
	if payload.ContentType == "image/jpeg" || payload.ContentType == "image/png" {
		img = resizeToFit(img, p.imageMaxPx)
		stripped, err := encodeOriginal(img, payload.ContentType)
		if err != nil {
			return fmt.Errorf("encode stripped original: %w", err)
		}
		if err := p.storage.PutObject(ctx, payload.StorageKey, payload.ContentType, bytes.NewReader(stripped), int64(len(stripped))); err != nil {
			return fmt.Errorf("upload stripped original: %w", err)
		}
	}

	// Generate thumbnail as JPEG regardless of original format.
	thumb := resizeToFit(img, thumbnailMaxPx)
	var thumbBuf bytes.Buffer
	if err := jpeg.Encode(&thumbBuf, thumb, &jpeg.Options{Quality: 85}); err != nil {
		return fmt.Errorf("encode thumbnail: %w", err)
	}
	thumbKey := thumbnailKey(payload.StorageKey)
	if err := p.storage.PutObject(ctx, thumbKey, "image/jpeg", &thumbBuf, int64(thumbBuf.Len())); err != nil {
		return fmt.Errorf("upload thumbnail: %w", err)
	}

	if err := p.store.SetThumbnailKey(ctx, payload.UploadID, thumbKey); err != nil {
		return fmt.Errorf("set thumbnail key: %w", err)
	}
	return nil
}

// moderationContextFor maps an upload category to the moderation context
// the chain runs in. Private-album uploads use ContextPrivate so the NSFW
// detector tags-not-blocks; everything else stays on the public default.
func moderationContextFor(category string) string {
	if category == "album-private" {
		return moderation.ContextPrivate
	}
	return moderation.ContextPublic
}

// disposeRejected executes the file disposition a rejection carries and records
// the outcome. Retain keeps the object for admin review; Purge deletes it;
// Quarantine moves it to a restricted, never-served prefix and preserves it
// (CSAM — destroying it can itself be unlawful).
func (p *ImageProcessor) disposeRejected(ctx context.Context, payload ImageProcessPayload, raw []byte, decision moderation.Decision) error {
	rec := uploads.RejectionRecord{
		UploadID:   payload.UploadID,
		Code:       decision.Code,
		Reason:     decision.Reason,
		Source:     decision.Source,
		Score:      decision.Score,
		Categories: decision.Categories,
	}
	log := p.log.Info().
		Str("upload_id", payload.UploadID).
		Str("code", decision.Code).
		Str("source", decision.Source).
		Float64("score", decision.Score)

	switch decision.Disposition {
	case moderation.DispositionQuarantine:
		log.Str("disposition", "quarantine").Msg("upload quarantined by moderation")
		qKey := quarantineKeyFor(payload.StorageKey)
		// Preserve the object under the restricted prefix, then remove the
		// original so its public URL 404s. We already have the bytes in memory.
		if putErr := p.storage.PutObject(ctx, qKey, payload.ContentType, bytes.NewReader(raw), int64(len(raw))); putErr != nil {
			// Preservation is the legal priority; if we cannot preserve, fail the
			// task so it retries rather than silently losing evidence.
			return fmt.Errorf("quarantine put: %w", putErr)
		}
		if delErr := p.storage.Delete(ctx, payload.StorageKey); delErr != nil {
			p.log.Warn().Err(delErr).Str("storage_key", payload.StorageKey).Msg("delete original after quarantine failed")
		}
		if markErr := p.modStore.MarkQuarantined(ctx, rec, qKey); markErr != nil {
			return fmt.Errorf("mark quarantined: %w", markErr)
		}
	case moderation.DispositionPurge:
		log.Str("disposition", "purge").Msg("upload rejected by moderation")
		if delErr := p.storage.Delete(ctx, payload.StorageKey); delErr != nil {
			p.log.Warn().Err(delErr).Str("storage_key", payload.StorageKey).Msg("delete rejected upload failed")
		}
		rec.FileRetained = false
		if markErr := p.modStore.MarkRejected(ctx, rec); markErr != nil {
			return fmt.Errorf("mark rejected: %w", markErr)
		}
	default: // DispositionRetain
		log.Str("disposition", "retain").Msg("upload rejected by moderation")
		rec.FileRetained = true
		if markErr := p.modStore.MarkRejected(ctx, rec); markErr != nil {
			return fmt.Errorf("mark rejected: %w", markErr)
		}
	}
	return nil
}

// isHardModerationFailure reports whether a moderator error must fail closed.
// A Chain wraps the failing moderator's severity in *ChainError; a bare
// moderator (tests) is queried directly via Severity().
func isHardModerationFailure(m moderation.Moderator, err error) bool {
	var ce *moderation.ChainError
	if errors.As(err, &ce) {
		return ce.Hard
	}
	return m.Severity() == moderation.SeverityHard
}

// quarantineKeyFor returns the restricted storage key a quarantined object is
// preserved under. The "quarantine/" prefix is the never-served namespace the
// serve layer and storage GC both treat as off-limits.
func quarantineKeyFor(storageKey string) string {
	return "quarantine/" + storageKey
}

// thumbnailKey returns the storage key for the thumbnail of storageKey.
// The extension is replaced with .jpg since thumbnails are always JPEG.
func thumbnailKey(storageKey string) string {
	ext := path.Ext(storageKey)
	base := strings.TrimSuffix(storageKey, ext)
	return "thumbnails/" + base + ".jpg"
}

// decodeImage decodes raw image bytes into an image.Image.
// Supports JPEG, PNG, WebP, and GIF (first frame for animated GIFs).
func decodeImage(contentType string, data []byte) (image.Image, error) {
	r := bytes.NewReader(data)
	switch contentType {
	case "image/jpeg":
		return jpeg.Decode(r)
	case "image/png":
		return png.Decode(r)
	case "image/webp":
		return webp.Decode(r)
	case "image/gif":
		g, err := gif.DecodeAll(r)
		if err != nil {
			return nil, err
		}
		if len(g.Image) == 0 {
			return nil, errors.New("gif has no frames")
		}
		return g.Image[0], nil
	default:
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}
}

// encodeOriginal re-encodes img as JPEG or PNG, discarding all metadata.
func encodeOriginal(img image.Image, contentType string) ([]byte, error) {
	var buf bytes.Buffer
	switch contentType {
	case "image/jpeg":
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92}); err != nil {
			return nil, err
		}
	case "image/png":
		if err := png.Encode(&buf, img); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("cannot re-encode %s", contentType)
	}
	return buf.Bytes(), nil
}

// resizeToFit scales src so that neither dimension exceeds maxPx,
// preserving the aspect ratio. Returns src unchanged if already within bounds.
func resizeToFit(src image.Image, maxPx int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxPx && h <= maxPx {
		return src
	}
	var dw, dh int
	if w >= h {
		dw = maxPx
		dh = h * maxPx / w
	} else {
		dh = maxPx
		dw = w * maxPx / h
	}
	dw = max(dw, 1)
	dh = max(dh, 1)
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	draw.BiLinear.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}
