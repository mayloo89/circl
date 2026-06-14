package logger

import (
	"runtime/debug"

	"github.com/rs/zerolog"
)

// PanicHook, when set, is called with a short source label every time Recover
// catches a panic. main wires it to the Prometheus panic counter. Kept as a
// package-level hook so the chat/worker packages don't have to depend on the
// metrics package just to count panics.
var PanicHook func(source string)

// Recover is a deferred guard for background goroutines. It turns a panic into
// a structured error log (event=panic) with a stack trace instead of letting
// it crash the whole process, and notifies PanicHook for alerting. Call it as
// `defer logger.Recover(log, "name")` at the top of a goroutine.
func Recover(log zerolog.Logger, source string) {
	if r := recover(); r != nil {
		log.Error().
			Str("event", "panic").
			Str("source", source).
			Interface("panic", r).
			Bytes("stack", debug.Stack()).
			Msg("recovered panic in goroutine")
		if PanicHook != nil {
			PanicHook(source)
		}
	}
}

// Go runs fn in a new goroutine guarded by Recover, so a panic in background
// work is logged rather than fatal. source labels the goroutine in logs/metrics.
func Go(log zerolog.Logger, source string, fn func()) {
	go func() {
		defer Recover(log, source)
		fn()
	}()
}
