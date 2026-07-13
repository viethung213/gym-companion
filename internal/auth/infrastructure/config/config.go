package config

import (
	"errors"
	"log"
	"os"
	"time"
)

// Config holds all environmental parameters dedicated to the Auth module.
type Config struct {
	GoogleClientID       string
	GoogleClientSecret   string
	GoogleRedirectURI    string
	FacebookClientID     string
	FacebookClientSecret string
	FacebookRedirectURI  string
	JWTIssuer            string
	AccessTokenTTL       time.Duration
	RefreshTokenTTL      time.Duration
	KeyRotationTTL       time.Duration
	StateSecret          string
}

// Load fetches all environment variables for the Auth module and sets defaults.
func Load() (*Config, error) {
	issuer := os.Getenv("JWT_ISSUER")
	if issuer == "" {
		issuer = "fitai-app"
	}

	stateSecret := os.Getenv("OAUTH_STATE_SECRET")
	if stateSecret == "" {
		return nil, errors.New("required environment variable OAUTH_STATE_SECRET is not set")
	}

	return &Config{
		GoogleClientID:       os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:   os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURI:    os.Getenv("GOOGLE_REDIRECT_URI"),
		FacebookClientID:     os.Getenv("FACEBOOK_CLIENT_ID"),
		FacebookClientSecret: os.Getenv("FACEBOOK_CLIENT_SECRET"),
		FacebookRedirectURI:  os.Getenv("FACEBOOK_REDIRECT_URI"),
		JWTIssuer:            issuer,
		AccessTokenTTL:       getEnvDuration("JWT_ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:      getEnvDuration("JWT_REFRESH_TOKEN_TTL", 30*24*time.Hour),
		KeyRotationTTL:       getEnvDuration("JWT_KEY_ROTATION_TTL", 7*24*time.Hour),
		StateSecret:          stateSecret,
	}, nil
}

func getEnvDuration(key string, defaultDuration time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return defaultDuration
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Printf("WARNING: Invalid duration for environment variable %s='%s' (expected format like '15m', '24h', '720h'). Using default: %v", key, v, defaultDuration)
		return defaultDuration
	}
	return d
}
