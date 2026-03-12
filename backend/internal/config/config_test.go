package config_test

import (
	"testing"

	"github.com/mayloo89/circl/backend/internal/config"
)

func TestEnvOrDefault(t *testing.T) {
	t.Run("returns env value when set", func(t *testing.T) {
		t.Setenv("TEST_KEY", "myvalue")
		got := config.EnvOrDefault("TEST_KEY", "default")
		if got != "myvalue" {
			t.Errorf("got %q, want %q", got, "myvalue")
		}
	})

	t.Run("returns default when env not set", func(t *testing.T) {
		t.Setenv("TEST_KEY", "")
		got := config.EnvOrDefault("TEST_KEY", "default")
		if got != "default" {
			t.Errorf("got %q, want %q", got, "default")
		}
	})

	t.Run("returns default when env missing", func(t *testing.T) {
		got := config.EnvOrDefault("DEFINITELY_NOT_SET_XYZ", "fallback")
		if got != "fallback" {
			t.Errorf("got %q, want %q", got, "fallback")
		}
	})
}

func TestRequireEnv(t *testing.T) {
	t.Run("returns value when env is set", func(t *testing.T) {
		t.Setenv("TEST_REQUIRED", "secret")
		got, err := config.RequireEnv("TEST_REQUIRED")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "secret" {
			t.Errorf("got %q, want %q", got, "secret")
		}
	})

	t.Run("returns error when env is missing", func(t *testing.T) {
		_, err := config.RequireEnv("DEFINITELY_NOT_SET_XYZ")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error when env is empty", func(t *testing.T) {
		t.Setenv("TEST_REQUIRED_EMPTY", "")
		_, err := config.RequireEnv("TEST_REQUIRED_EMPTY")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
