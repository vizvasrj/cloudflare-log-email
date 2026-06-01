package internals

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	// Cloudflare
	CFAPIToken string
	CFZoneTag  string

	// PostgreSQL
	DatabaseURL string

	// Web server
	Port          string
	UIPassword    string
	SessionSecret string

	// Polling
	PollInterval time.Duration
	Lookback     time.Duration
}

func LoadConfig() Config {
	cfg := Config{
		CFAPIToken:   mustEnv("CF_API_TOKEN"),
		CFZoneTag:    mustEnv("CF_ZONE_TAG"),
		DatabaseURL:  mustEnv("DATABASE_URL"),
		Port:         envOr("PORT", "8080"),
		UIPassword:   mustEnv("UI_PASSWORD"),
		PollInterval: time.Minute,
	}
	cfg.SessionSecret = envOr("SESSION_SECRET", generateSecret())

	mins, _ := strconv.Atoi(envOr("LOOKBACK_MINUTES", "5"))
	if mins < 1 {
		mins = 5
	}
	cfg.Lookback = time.Duration(mins) * time.Minute
	return cfg
}

func mustEnv(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	log.Fatalf("required env var %q is not set — copy .env.example to .env and fill it in", key)
	return ""
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func generateSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
