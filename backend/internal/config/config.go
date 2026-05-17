package config

import (
	"cmp"
	"fmt"
	"os"
	"strconv"
)

// EnvOrDefault returns the value of the environment variable key,
// or def if the variable is not set or empty.
func EnvOrDefault(key, def string) string {
	return cmp.Or(os.Getenv(key), def)
}

// EnvIntOrDefault returns the integer value of the environment variable key,
// or def if the variable is not set, empty, or not a valid integer.
func EnvIntOrDefault(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// EnvFloatOrDefault returns the float64 value of the environment variable
// key, or def if the variable is not set, empty, or not a valid float.
func EnvFloatOrDefault(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
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
