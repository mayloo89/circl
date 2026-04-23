package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

// lokiWriter is a zerolog io.Writer that batches log lines and pushes them to
// Loki's HTTP push API. It extracts the "level" field from each zerolog JSON
// line and uses it as an indexed Loki label alongside "service" and "env".
//
// Entries are flushed every 2 seconds or when the buffer reaches 100 lines,
// whichever comes first. Close flushes any remaining entries synchronously.
type lokiWriter struct {
	url     string
	service string
	env     string

	mu  sync.Mutex
	buf map[string][]lokiEntry // keyed by level

	flushCh chan struct{}
	done    chan struct{}
	wg      sync.WaitGroup
	client  *http.Client
}

type lokiEntry [2]string // [timestamp_ns, log_line]

func newLokiWriter(url, service, env string) *lokiWriter {
	w := &lokiWriter{
		url:     url,
		service: service,
		env:     env,
		buf:     make(map[string][]lokiEntry),
		flushCh: make(chan struct{}, 1),
		done:    make(chan struct{}),
		client:  &http.Client{Timeout: 5 * time.Second},
	}
	w.wg.Go(w.run)
	return w
}

func (w *lokiWriter) Write(p []byte) (int, error) {
	level := extractLevel(p)
	ts := strconv.FormatInt(time.Now().UnixNano(), 10)

	w.mu.Lock()
	w.buf[level] = append(w.buf[level], lokiEntry{ts, string(p)})
	shouldFlush := len(w.buf[level]) >= 100
	w.mu.Unlock()

	if shouldFlush {
		select {
		case w.flushCh <- struct{}{}:
		default:
		}
	}
	return len(p), nil
}

func (w *lokiWriter) Close() {
	close(w.done)
	w.wg.Wait()
}

func (w *lokiWriter) run() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.flush()
		case <-w.flushCh:
			w.flush()
		case <-w.done:
			w.flush()
			return
		}
	}
}

func (w *lokiWriter) flush() {
	w.mu.Lock()
	if len(w.buf) == 0 {
		w.mu.Unlock()
		return
	}
	buf := w.buf
	w.buf = make(map[string][]lokiEntry)
	w.mu.Unlock()

	type stream struct {
		Stream map[string]string `json:"stream"`
		Values []lokiEntry       `json:"values"`
	}
	type payload struct {
		Streams []stream `json:"streams"`
	}

	streams := make([]stream, 0, len(buf))
	for level, entries := range buf {
		streams = append(streams, stream{
			Stream: map[string]string{
				"service": w.service,
				"env":     w.env,
				"level":   level,
			},
			Values: entries,
		})
	}

	body, err := json.Marshal(payload{Streams: streams})
	if err != nil {
		return
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, w.url+"/loki/api/v1/push", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return
	}
	defer func() {
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		resp.Body.Close()              //nolint:errcheck
	}()

	if resp.StatusCode/100 != 2 {
		fmt.Fprintf(os.Stderr, "loki: push failed with status %d\n", resp.StatusCode)
	}
}

// extractLevel returns the value of the "level" key in a zerolog JSON line.
// Falls back to "info" on any parse failure so Loki always gets a valid label.
func extractLevel(p []byte) string {
	var v struct {
		Level string `json:"level"`
	}
	if err := json.Unmarshal(p, &v); err != nil || v.Level == "" {
		return "info"
	}
	return v.Level
}

