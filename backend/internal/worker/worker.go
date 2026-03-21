// Package worker provides a Redis-backed background task queue using asynq.
// Tasks are enqueued by the HTTP server and processed by a worker server
// running in the same process.
package worker

import (
	"github.com/hibiken/asynq"
)

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
func NewServer(redisOpt asynq.RedisClientOpt, concurrency int) *Server {
	srv := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: concurrency,
		Queues:      map[string]int{"default": 1},
	})
	return &Server{s: srv}
}

// Start registers task handlers and begins processing. It is non-blocking.
func (s *Server) Start(processor *ImageProcessor) error {
	mux := asynq.NewServeMux()
	mux.HandleFunc(TaskProcessImage, processor.Handle)
	return s.s.Start(mux)
}

// Shutdown gracefully stops the worker server, waiting for in-flight tasks to
// complete.
func (s *Server) Shutdown() { s.s.Shutdown() }
