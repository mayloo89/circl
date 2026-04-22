package logger

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExtractLevel(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{`{"level":"info","message":"ok"}`, "info"},
		{`{"level":"warn","message":"ok"}`, "warn"},
		{`{"level":"error","message":"ok"}`, "error"},
		{`{"message":"no level field"}`, "info"},
		{`not json`, "info"},
		{`{}`, "info"},
	}
	for _, c := range cases {
		got := extractLevel([]byte(c.input))
		if got != c.want {
			t.Errorf("extractLevel(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestLokiWriter_FlushesOnClose(t *testing.T) {
	type lokiPush struct {
		Streams []struct {
			Stream map[string]string `json:"stream"`
			Values [][2]string       `json:"values"`
		} `json:"streams"`
	}

	received := make(chan lokiPush, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var p lokiPush
		_ = json.Unmarshal(body, &p)
		received <- p
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	w := newLokiWriter(srv.URL, "test-service", "test")
	_, _ = w.Write([]byte(`{"level":"info","message":"hello"}`))
	_, _ = w.Write([]byte(`{"level":"warn","message":"watch out"}`))
	w.Close()

	select {
	case p := <-received:
		if len(p.Streams) == 0 {
			t.Fatal("expected at least one stream")
		}
		total := 0
		for _, s := range p.Streams {
			if s.Stream["service"] != "test-service" {
				t.Errorf("service label = %q, want %q", s.Stream["service"], "test-service")
			}
			if s.Stream["env"] != "test" {
				t.Errorf("env label = %q, want %q", s.Stream["env"], "test")
			}
			total += len(s.Values)
		}
		if total != 2 {
			t.Errorf("total log lines = %d, want 2", total)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Loki push")
	}
}

func TestLokiWriter_FlushesOnTicker(t *testing.T) {
	received := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case received <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	w := newLokiWriter(srv.URL, "svc", "dev")
	defer w.Close()

	_, _ = w.Write([]byte(`{"level":"info","message":"tick test"}`))

	select {
	case <-received:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for ticker flush")
	}
}

func TestLokiWriter_FlushOnFullBuffer(t *testing.T) {
	received := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case received <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	w := newLokiWriter(srv.URL, "svc", "dev")
	defer w.Close()

	for range 100 {
		_, _ = w.Write([]byte(`{"level":"info","message":"fill"}`))
	}

	select {
	case <-received:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for buffer-full flush")
	}
}
