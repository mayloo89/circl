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
type ImageProcessPayload struct {
	UploadID    string `json:"upload_id"`
	StorageKey  string `json:"storage_key"`
	ContentType string `json:"content_type"`
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
	_, err = client.c.EnqueueContext(ctx, asynq.NewTask(TaskProcessImage, wrapped))
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
	// any further storage I/O. A rejection deletes the original from storage,
	// marks the row, and short-circuits. Moderator errors are logged and
	// treated as allow — we don't block uploads on a flaky classifier.
	if p.moderator != nil && p.modStore != nil {
		bounds := img.Bounds()
		decision, modErr := p.moderator.Check(ctx, moderation.Input{
			ContentType: payload.ContentType,
			SizeBytes:   int64(len(raw)),
			Width:       bounds.Dx(),
			Height:      bounds.Dy(),
			Hash:        moderation.HashFor(raw),
			Bytes:       raw,
		})
		if modErr != nil {
			p.log.Warn().Err(modErr).Str("upload_id", payload.UploadID).Msg("moderator error; failing open")
		} else if !decision.Allowed {
			retain := retainFileFor(decision.Code)
			p.log.Info().
				Str("upload_id", payload.UploadID).
				Str("code", decision.Code).
				Str("source", decision.Source).
				Float64("score", decision.Score).
				Bool("retain_file", retain).
				Msg("upload rejected by moderation")
			if !retain {
				if delErr := p.storage.Delete(ctx, payload.StorageKey); delErr != nil {
					// Best-effort: the rejection mark is the source of truth. Failure
					// to delete leaves an orphaned object that the storage GC sweep
					// will clean up; we never serve it because the public URL is
					// derived from a row whose moderation_status is 'rejected'.
					p.log.Warn().Err(delErr).Str("storage_key", payload.StorageKey).Msg("delete rejected upload failed")
				}
			}
			rec := uploads.RejectionRecord{
				UploadID:     payload.UploadID,
				Code:         decision.Code,
				Reason:       decision.Reason,
				Source:       decision.Source,
				Score:        decision.Score,
				Categories:   decision.Categories,
				FileRetained: retain,
			}
			if markErr := p.modStore.MarkRejected(ctx, rec); markErr != nil {
				return fmt.Errorf("mark rejected: %w", markErr)
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

// retainFileFor decides whether to keep the storage object for admin review
// after a rejection. NSFW and heuristic rejections are legal-to-store adult
// content (or benign metadata edge cases) — keep them so admins can verify
// false positives. Hash-list matches target CSAM / NCII feeds; we must purge
// those immediately and never expose them to the admin UI.
func retainFileFor(code string) bool {
	switch code {
	case moderation.CodeNSFWDetected,
		moderation.CodeSizeOutOfBounds,
		moderation.CodeAspectOutOfBounds:
		return true
	default:
		return false
	}
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
