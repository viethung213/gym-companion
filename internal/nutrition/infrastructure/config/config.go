package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

// Config holds all environmental parameters dedicated to the Nutrition module.
type Config struct {
	AIProvider           string
	GroqAPIKey           string
	GroqBaseURL          string
	GroqModel            string
	GroqFallbackModels   string
	GeminiModel          string
	GeminiFallbackModels string
	KafkaBrokers         string
	GoogleAPIKey         string
	MaxOutputTokens      int
}

// LoadConfig loads environment variables for the Nutrition module with default fallbacks.
func LoadConfig() Config {
	aiProvider := strings.ToLower(strings.TrimSpace(os.Getenv("AI_PROVIDER")))
	if aiProvider == "" {
		aiProvider = "gemini"
	}

	groqAPIKey := os.Getenv("GROQ_API_KEY")
	groqBaseURL := os.Getenv("GROQ_BASE_URL")
	if groqBaseURL == "" {
		groqBaseURL = "https://api.groq.com/openai/v1"
	}

	groqModel := os.Getenv("GROQ_MODEL")
	if groqModel == "" {
		groqModel = "llama-3.3-70b-versatile"
	}

	groqFallbackModels := os.Getenv("GROQ_FALLBACK_MODELS")
	if groqFallbackModels == "" {
		groqFallbackModels = "llama-3.1-8b-instant"
	}

	geminiModel := os.Getenv("GEMINI_MODEL")
	if geminiModel == "" {
		geminiModel = "gemini-3.8-flash"
	}

	fallbackModels := os.Getenv("GEMINI_FALLBACK_MODELS")
	if fallbackModels == "" {
		fallbackModels = "gemini-3.5-flash-lite"
	}

	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		kafkaBrokers = "localhost:9092"
	}

	apiKey := os.Getenv("GOOGLE_API_KEY_NUTRITION")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}

	maxOutputTokens := 3000
	if val := os.Getenv("AI_MAX_OUTPUT_TOKENS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			maxOutputTokens = parsed
		}
	}

	activeModel := groqModel
	activeKey := groqAPIKey
	if aiProvider == "gemini" {
		activeModel = geminiModel
		activeKey = apiKey
	}

	masked := "<EMPTY>"
	if len(activeKey) > 8 {
		masked = fmt.Sprintf("%s...%s (len=%d)", activeKey[:6], activeKey[len(activeKey)-4:], len(activeKey))
	} else if activeKey != "" {
		masked = fmt.Sprintf("%s... (len=%d)", activeKey[:2], len(activeKey))
	}

	log.Printf("[Nutrition Config] Provider: %s, Active Model: %s, Max Tokens: %d, Key Status: %s", aiProvider, activeModel, maxOutputTokens, masked)

	return Config{
		AIProvider:           aiProvider,
		GroqAPIKey:           groqAPIKey,
		GroqBaseURL:          groqBaseURL,
		GroqModel:            groqModel,
		GroqFallbackModels:   groqFallbackModels,
		GeminiModel:          geminiModel,
		GeminiFallbackModels: fallbackModels,
		KafkaBrokers:         kafkaBrokers,
		GoogleAPIKey:         apiKey,
		MaxOutputTokens:      maxOutputTokens,
	}
}
