// Package config reads server settings from environment variables.
package config

import (
	"log/slog"
	"os"
)

// devJWTSecret is a fallback so the app can still start without setup.
const devJWTSecret = "dev-insecure-secret-change-me"

// Config holds the settings the server needs to run.
type Config struct {
	Port      string
	DBPath    string
	JWTSecret string
}

// Load reads Config from environment variables, filling in defaults.
func Load() Config {
	cfg := Config{
		Port:      getEnv("PORT", "8080"),
		DBPath:    getEnv("DB_PATH", "./data/tickets.db"),
		JWTSecret: getEnv("JWT_SECRET", ""),
	}

	if cfg.JWTSecret == "" {
		// A known secret in production would let anyone forge login tokens,
		// so warn loudly instead of failing - local/demo runs still work.
		slog.Warn("JWT_SECRET is not set - using an insecure development default. " +
			"Set the JWT_SECRET environment variable before deploying this anywhere real.")
		cfg.JWTSecret = devJWTSecret
	}

	return cfg
}

// getEnv reads an environment variable, or returns fallback if unset.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
