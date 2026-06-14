package logger_test

import (
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/logger"
)

func TestGo_RecoversPanicAndNotifiesHook(t *testing.T) {
	// The hook fires from inside Recover, so synchronising on it is the
	// deterministic signal that the panic was caught (fn's own defers run
	// before the guard, so we can't key off fn completing).
	got := make(chan string, 1)
	logger.PanicHook = func(s string) { got <- s }
	t.Cleanup(func() { logger.PanicHook = nil })

	logger.Go(zerolog.Nop(), "test.goroutine", func() { panic("boom") })

	select {
	case source := <-got:
		if source != "test.goroutine" {
			t.Errorf("PanicHook source = %q, want %q", source, "test.goroutine")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("panic was not recovered / hook not called")
	}
}

func TestGo_RunsFnNormally(t *testing.T) {
	done := make(chan int, 1)
	logger.Go(zerolog.Nop(), "ok", func() { done <- 42 })
	if got := <-done; got != 42 {
		t.Errorf("fn result = %d, want 42", got)
	}
}
