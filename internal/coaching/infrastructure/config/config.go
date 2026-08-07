package config

import (
	"fmt"
	"log"
	"os"
)

// Config holds all environmental parameters dedicated to the Coaching module.
type Config struct {
	GeminiModel          string
	GeminiFallbackModels string
	KafkaBrokers         string
	CoachPromptDumpDir   string
	GoogleAPIKey         string
}

// LoadConfig loads environment variables for the Coaching module with default fallbacks.
func LoadConfig() Config {
	geminiModel := os.Getenv("GEMINI_MODEL")
	if geminiModel == "" {
		geminiModel = "gemini-3.5-flash-lite"
	}

	fallbackModels := os.Getenv("GEMINI_FALLBACK_MODELS")
	if fallbackModels == "" {
		fallbackModels = "gemini-2.5-flash,gemini-2.0-flash,gemini-1.5-flash"
	}

	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		kafkaBrokers = "localhost:9092"
	}

	apiKey := os.Getenv("GOOGLE_API_KEY_COACHING")
	masked := "<EMPTY>"
	if len(apiKey) > 8 {
		masked = fmt.Sprintf("%s...%s (len=%d)", apiKey[:6], apiKey[len(apiKey)-4:], len(apiKey))
	} else if apiKey != "" {
		masked = fmt.Sprintf("%s... (len=%d)", apiKey[:2], len(apiKey))
	}

	log.Printf("[Coaching Config] Loaded Gemini Model: %s, API Key Status: %s", geminiModel, masked)

	return Config{
		GeminiModel:          geminiModel,
		GeminiFallbackModels: fallbackModels,
		KafkaBrokers:         kafkaBrokers,
		CoachPromptDumpDir:   os.Getenv("COACH_PROMPT_DUMP_DIR"),
		GoogleAPIKey:         apiKey,
	}
}
