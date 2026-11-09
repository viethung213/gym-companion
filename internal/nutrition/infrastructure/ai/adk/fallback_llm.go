package adk

import (
	"context"
	"strings"

	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/ai/provider"
	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/config"
	"google.golang.org/adk/v2/model"
)

// FallbackLLM aliases provider.FallbackLLM for backward compatibility.
type FallbackLLM = provider.FallbackLLM

// NewFallbackLLMWrapper constructs a FallbackLLM wrapping raw model.LLM instances directly.
func NewFallbackLLMWrapper(models []model.LLM) *FallbackLLM {
	return provider.NewFallbackLLM(models)
}

// NewFallbackLLMFromEnv delegates model creation to the pluggable provider adapter factory.
func NewFallbackLLMFromEnv(ctx context.Context) (model.LLM, error) {
	cfg := config.LoadConfig()
	return provider.NewLLM(ctx, &cfg)
}

// NewFallbackLLM constructs a FallbackLLM from Gemini models (backward compatibility).
func NewFallbackLLM(ctx context.Context, modelNames []string, apiKey string) (model.LLM, error) {
	var primary string
	var fallbackStr string
	if len(modelNames) > 0 {
		primary = modelNames[0]
		fallbackStr = strings.Join(modelNames[1:], ",")
	}
	cfg := &config.Config{
		AIProvider:           "gemini",
		GeminiModel:          primary,
		GeminiFallbackModels: fallbackStr,
		GoogleAPIKey:         apiKey,
	}
	p := &provider.GeminiProvider{}
	return p.Build(ctx, cfg)
}

// NewGroqFallbackLLM constructs a FallbackLLM from Groq models (backward compatibility).
func NewGroqFallbackLLM(ctx context.Context, modelNames []string, apiKey, baseURL string) (model.LLM, error) {
	var primary string
	var fallbackStr string
	if len(modelNames) > 0 {
		primary = modelNames[0]
		fallbackStr = strings.Join(modelNames[1:], ",")
	}
	cfg := &config.Config{
		AIProvider:         "groq",
		GroqModel:          primary,
		GroqFallbackModels: fallbackStr,
		GroqAPIKey:         apiKey,
		GroqBaseURL:        baseURL,
	}
	p := &provider.GroqProvider{}
	return p.Build(ctx, cfg)
}
