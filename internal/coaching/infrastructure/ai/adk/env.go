package adk

import (
	"os"
	"strings"
)

// loadEnvFile reads GOOGLE_API_KEY from root .env file if present.
func loadEnvFile() {
	data, err := os.ReadFile(".env")
	if err != nil {
		data, err = os.ReadFile("internal/coaching/infrastructure/ai/adk/.env")
		if err != nil {
			return
		}
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		parts := strings.SplitN(strings.TrimSpace(line), "=", 2)
		if len(parts) == 2 && os.Getenv(parts[0]) == "" {
			os.Setenv(parts[0], parts[1])
		}
	}
}
