package config

import (
	"os"
	"time"
)

// Config holds environmental parameters dedicated to the Solo Match module.
type Config struct {
	BotFallbackTimeout time.Duration
}

// LoadConfig loads environment variables for the Solo Match module with default fallbacks.
func LoadConfig() Config {
	botTimeout := 15 * time.Second
	if envTimeout := os.Getenv("SOLO_BOT_FALLBACK_TIMEOUT"); envTimeout != "" {
		if d, err := time.ParseDuration(envTimeout); err == nil && d > 0 {
			botTimeout = d
		}
	}

	return Config{
		BotFallbackTimeout: botTimeout,
	}
}
