// Package config is responsible for reading settings from environment
// variables and turning them into a simple, typed struct that the rest of
// the program can use. Centralizing this in one place means nobody else in
// the codebase needs to call os.Getenv directly or guess what the default
// values are.
package config

import (
	"log/slog"
	"os"
)

// devJWTSecret is what we use if the operator forgot to set JWT_SECRET.
// It is intentionally obvious and well-known so that nobody mistakes it for
// something safe - it exists only so the app can still start up in a local
// dev/demo environment without extra setup.
const devJWTSecret = "dev-insecure-secret-change-me"

// Config holds every setting the server needs to run, gathered from
// environment variables (or their defaults) exactly once at startup.
type Config struct {
	// Port is the TCP port the HTTP server listens on.
	Port string
	// DBPath is the filesystem path to the SQLite database file.
	DBPath string
	// JWTSecret is the key used to sign and verify login tokens (JWTs).
	// Anyone who has this value could forge a valid login token, so in a
	// real deployment it must be set to a long, random, secret value.
	JWTSecret string
}

// Load reads configuration from environment variables, applying sensible
// defaults for anything that is missing so the server can start with zero
// required configuration (useful for `docker run` with no extra flags).
func Load() Config {
	cfg := Config{
		Port:      getEnv("PORT", "8080"),
		DBPath:    getEnv("DB_PATH", "./data/tickets.db"),
		JWTSecret: getEnv("JWT_SECRET", ""),
	}

	if cfg.JWTSecret == "" {
		// We log this loudly at Warn level (not Info) because running with
		// a publicly-known secret in production would let anyone forge
		// login tokens for any user. This is not a fatal error though,
		// because we still want local development and grading/demo
		// environments to "just work" without extra setup.
		slog.Warn("JWT_SECRET is not set - using an insecure development default. " +
			"Set the JWT_SECRET environment variable before deploying this anywhere real.")
		cfg.JWTSecret = devJWTSecret
	}

	return cfg
}

// getEnv reads an environment variable, returning fallback when it is unset
// or empty. This tiny helper avoids repeating the same "check and default"
// logic for every single setting.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
