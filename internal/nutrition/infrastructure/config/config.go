package config

import "os"

// Config holds all environmental parameters dedicated to the Nutrition module.
type Config struct {
	GeminiModel          string
	GeminiFallbackModels string
	KafkaBrokers         string
	GoogleAPIKey         string
}

// LoadConfig loads environment variables for the Nutrition module with default fallbacks.
func LoadConfig() Config {
	primaryModel := os.Getenv("GEMINI_MODEL")
	if primaryModel == "" {
		primaryModel = "gemini-2.5-flash"
	}

	fallbackStr := os.Getenv("GEMINI_FALLBACK_MODELS")
	if fallbackStr == "" {
		fallbackStr = "gemini-2.0-flash,gemini-2.5-flash-lite,gemini-1.5-flash"
	}

	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		kafkaBrokers = "localhost:9092"
	}

	return Config{
		GeminiModel:          primaryModel,
		GeminiFallbackModels: fallbackStr,
		KafkaBrokers:         kafkaBrokers,
		GoogleAPIKey:         os.Getenv("GOOGLE_API_KEY_NUTRITION"),
	}
}
