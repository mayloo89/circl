package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

// New creates a zerolog.Logger for the given environment and log level.
// Development: human-readable console output. Production: JSON to stdout.
// Unknown level strings default to info.
func New(env, level string) zerolog.Logger {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	if env == "development" {
		return zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).
			Level(lvl).
			With().
			Timestamp().
			Logger()
	}

	return zerolog.New(os.Stdout).Level(lvl).With().Timestamp().Logger()
}
