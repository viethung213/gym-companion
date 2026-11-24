package config

import "os"

// defaultMediaBaseURL serves free-exercise-db images from the pinned upstream
// commit (internal/shared/seeder/data/upstream/free-exercise-db/UPSTREAM.md).
const defaultMediaBaseURL = "https://raw.githubusercontent.com/yuhonas/free-exercise-db/" +
	"f00c92c7dcf1216a928a52c3706c7ce8e2f71ed5/exercises/"

// Config holds all environmental parameters dedicated to the Exercise module.
type Config struct {
	// MediaBaseURL prefixes relative exercise image paths (EXERCISE_MEDIA_BASE_URL).
	MediaBaseURL string
}

// LoadConfig loads environment variables for the Exercise module with default fallbacks.
func LoadConfig() Config {
	mediaBaseURL := os.Getenv("EXERCISE_MEDIA_BASE_URL")
	if mediaBaseURL == "" {
		mediaBaseURL = defaultMediaBaseURL
	}

	return Config{
		MediaBaseURL: mediaBaseURL,
	}
}
