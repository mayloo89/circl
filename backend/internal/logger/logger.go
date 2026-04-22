package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// New creates a zerolog.Logger for the given environment and log level.
// Development: human-readable console output on stdout. Production: JSON.
// Unknown level strings default to info.
//
// When LOKI_URL is set, log lines are also shipped to Loki in batches. The
// returned flush function must be called on shutdown to deliver any buffered
// entries; it is safe to call even when Loki is not configured.
func New(env, level string) (zerolog.Logger, func()) {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	var baseWriter io.Writer
	if env == "development" {
		baseWriter = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	} else {
		baseWriter = os.Stdout
	}

	lokiURL := os.Getenv("LOKI_URL")
	if lokiURL == "" {
		return zerolog.New(baseWriter).Level(lvl).With().Timestamp().Logger(), func() {}
	}

	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "circl-backend"
	}

	lw := newLokiWriter(lokiURL, serviceName, env)
	multi := zerolog.MultiLevelWriter(baseWriter, lw)
	return zerolog.New(multi).Level(lvl).With().Timestamp().Logger(), lw.Close
}
