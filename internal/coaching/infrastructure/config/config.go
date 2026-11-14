package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Config holds all environmental parameters dedicated to the Coaching module.
type Config struct {
	AIProvider           string
	GroqAPIKey           string
	GroqBaseURL          string
	GroqModel            string
	GroqFallbackModels   string
	GeminiModel          string
	GeminiFallbackModels string
	KafkaBrokers         string
	CoachPromptDumpDir   string
	GoogleAPIKey         string
	MaxOutputTokens      int
}

// LoadConfig loads environment variables for the Coaching module with default fallbacks.
func LoadConfig() Config {
	aiProvider := strings.ToLower(strings.TrimSpace(os.Getenv("AI_PROVIDER")))
	if aiProvider == "" {
		aiProvider = "groq"
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
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}

	activeModel := groqModel
	activeKey := groqAPIKey
	if aiProvider == "gemini" {
		activeModel = geminiModel
		activeKey = apiKey
	}

	maxOutputTokens := 16384
	if val := os.Getenv("AI_MAX_OUTPUT_TOKENS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			maxOutputTokens = parsed
		}
	}

	masked := "<EMPTY>"
	if len(activeKey) > 8 {
		masked = fmt.Sprintf("%s...%s (len=%d)", activeKey[:6], activeKey[len(activeKey)-4:], len(activeKey))
	} else if activeKey != "" {
		masked = fmt.Sprintf("%s... (len=%d)", activeKey[:2], len(activeKey))
	}

	slog.Info("Coaching Config initialized",
		slog.String("provider", aiProvider),
		slog.String("active_model", activeModel),
		slog.Int("max_tokens", maxOutputTokens),
		slog.String("key_status", masked),
	)

	return Config{
		AIProvider:           aiProvider,
		GroqAPIKey:           groqAPIKey,
		GroqBaseURL:          groqBaseURL,
		GroqModel:            groqModel,
		GroqFallbackModels:   groqFallbackModels,
		GeminiModel:          geminiModel,
		GeminiFallbackModels: fallbackModels,
		KafkaBrokers:         kafkaBrokers,
		CoachPromptDumpDir:   os.Getenv("COACH_PROMPT_DUMP_DIR"),
		GoogleAPIKey:         apiKey,
		MaxOutputTokens:      maxOutputTokens,
	}
}
