package config

import "os"

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

	return Config{
		GeminiModel:          geminiModel,
		GeminiFallbackModels: fallbackModels,
		KafkaBrokers:         kafkaBrokers,
		CoachPromptDumpDir:   os.Getenv("COACH_PROMPT_DUMP_DIR"),
		GoogleAPIKey:         os.Getenv("GOOGLE_API_KEY_COACHING"),
	}
}
