package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
)

// TaskExportUser is the asynq task type for per-user data exports
// (Habeas Data / GDPR Art. 20).
const TaskExportUser = "export:user"

// ExportPayload is the data the export builder needs.
type ExportPayload struct {
	RequestID string `json:"request_id"`
	UserID    string `json:"user_id"`
}

// ExportBuilder is the interface the worker calls to actually build the zip.
// Satisfied by exports.Service.Build (wrapped by an adapter in main.go) so
// the worker package doesn't have to import the exports package directly.
type ExportBuilder interface {
	Build(ctx context.Context, requestID string) error
}

// ExportHandler handles the export:user asynq task.
type ExportHandler struct {
	builder ExportBuilder
	log     zerolog.Logger
}

// NewExportHandler returns an ExportHandler.
func NewExportHandler(b ExportBuilder, log zerolog.Logger) *ExportHandler {
	return &ExportHandler{
		builder: b,
		log:     log.With().Str("component", "export_worker").Logger(),
	}
}

// EnqueueExportUser schedules an export build via the asynq client.
func EnqueueExportUser(ctx context.Context, client *Client, p ExportPayload) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("worker: marshal export payload: %w", err)
	}
	wrapped, err := InjectTraceContext(ctx, raw)
	if err != nil {
		return fmt.Errorf("worker: inject trace context: %w", err)
	}
	_, err = client.c.EnqueueContext(ctx, asynq.NewTask(TaskExportUser, wrapped))
	return err
}

// Handle implements the asynq handler signature.
func (h *ExportHandler) Handle(ctx context.Context, t *asynq.Task) error {
	var payload ExportPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("worker: unmarshal export payload: %w", err)
	}
	if err := h.builder.Build(ctx, payload.RequestID); err != nil {
		h.log.Error().Err(err).Str("request_id", payload.RequestID).Msg("build export failed")
		return err
	}
	return nil
}
