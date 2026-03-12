package config

import (
	"fmt"
	"os"
)

// EnvOrDefault returns the value of the environment variable key,
// or def if the variable is not set or empty.
func EnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// RequireEnv returns the value of the environment variable key,
// or an error if the variable is not set or empty.
func RequireEnv(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("required environment variable %q is not set", key)
	}
	return v, nil
}
