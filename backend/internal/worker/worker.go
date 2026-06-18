// Package worker provides a Redis-backed background task queue using asynq.
// Tasks are enqueued by the HTTP server and processed by a worker server
// running in the same process.
package worker

import (
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
)

// maxRetryDelay caps the exponential backoff between task retries. asynq's
// default backoff grows without bound, which would space later retries days
// apart — wrong for a moderation "hold & retry" where a hard-fail detector
// (CSAM / NCII) must keep being re-checked at a steady cadence until the
// vendor recovers. Capping at 5 minutes keeps held uploads moving without
// hammering the vendor.
const maxRetryDelay = 5 * time.Minute

// cappedRetryDelay is exponential backoff (2^n seconds) clamped to maxRetryDelay.
func cappedRetryDelay(n int, _ error, _ *asynq.Task) time.Duration {
	d := time.Duration(1<<min(n, 20)) * time.Second
	return min(d, maxRetryDelay)
}

// asynqLogger bridges asynq's Logger interface to a zerolog.Logger.
type asynqLogger struct{ log zerolog.Logger }

func (l *asynqLogger) Debug(args ...any) { l.log.Debug().Msg(fmt.Sprint(args...)) }
func (l *asynqLogger) Info(args ...any)  { l.log.Info().Msg(fmt.Sprint(args...)) }
func (l *asynqLogger) Warn(args ...any)  { l.log.Warn().Msg(fmt.Sprint(args...)) }
func (l *asynqLogger) Error(args ...any) { l.log.Error().Msg(fmt.Sprint(args...)) }
func (l *asynqLogger) Fatal(args ...any) { l.log.Fatal().Msg(fmt.Sprint(args...)) }

// Client enqueues background tasks.
type Client struct {
	c *asynq.Client
}

// NewClient creates an asynq task client backed by Redis.
func NewClient(redisOpt asynq.RedisClientOpt) *Client {
	return &Client{c: asynq.NewClient(redisOpt)}
}

// Close releases the client's Redis connection.
func (c *Client) Close() error { return c.c.Close() }

// Server runs background task handlers.
type Server struct {
	s *asynq.Server
}

// NewServer creates an asynq server that processes tasks from Redis.
func NewServer(redisOpt asynq.RedisClientOpt, concurrency int, log zerolog.Logger) *Server {
	srv := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency:    concurrency,
		Queues:         map[string]int{"default": 1},
		Logger:         &asynqLogger{log: log.With().Str("component", "asynq").Logger()},
		RetryDelayFunc: cappedRetryDelay,
	})
	return &Server{s: srv}
}

// Start registers task handlers and begins processing. It is non-blocking.
// exportHandler may be nil — in that case the export queue is skipped (useful
// for tests and one-off deployments that don't ship the data-export feature).
func (s *Server) Start(processor *ImageProcessor, exportHandler *ExportHandler) error {
	mux := asynq.NewServeMux()
	mux.Use(otelMiddleware)
	mux.HandleFunc(TaskProcessImage, processor.Handle)
	if exportHandler != nil {
		mux.HandleFunc(TaskExportUser, exportHandler.Handle)
	}
	return s.s.Start(mux)
}

// Shutdown gracefully stops the worker server, waiting for in-flight tasks to
// complete.
func (s *Server) Shutdown() { s.s.Shutdown() }
