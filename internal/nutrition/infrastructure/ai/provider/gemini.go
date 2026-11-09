package provider

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/config"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/genai"
)

// Compile-time interface check.
var _ Provider = (*GeminiProvider)(nil)

// GeminiProvider triển khai phích cắm cho Google Gemini API.
type GeminiProvider struct{}

// Name trả về định danh của provider.
func (g *GeminiProvider) Name() string {
	return "gemini"
}

// Build khởi tạo model.LLM cho Gemini (có bọc FallbackLLM nếu nhiều model).
func (g *GeminiProvider) Build(ctx context.Context, cfg *config.Config) (model.LLM, error) {
	defaultModels := []string{"gemini-3.1-flash-lite", "gemini-3.5-flash", "gemini-3.5-flash-lite"}
	rawNames := append([]string{cfg.GeminiModel}, strings.Split(cfg.GeminiFallbackModels, ",")...)
	rawNames = append(rawNames, defaultModels...)

	modelNames := make([]string, 0, len(rawNames))
	seen := make(map[string]bool)

	for _, name := range rawNames {
		trimmed := strings.TrimSpace(name)
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			modelNames = append(modelNames, trimmed)
		}
	}

	if len(modelNames) == 0 {
		return nil, errors.New("gemini provider: modelNames list cannot be empty")
	}

	var clientCfg *genai.ClientConfig
	if cfg.GoogleAPIKey != "" {
		clientCfg = &genai.ClientConfig{APIKey: cfg.GoogleAPIKey}
	}

	models := make([]model.LLM, 0, len(modelNames))
	for _, name := range modelNames {
		m, err := gemini.NewModel(ctx, name, clientCfg)
		if err != nil {
			log.Printf("[GeminiProvider] Warning: Failed to initialize model %s: %v", name, err)
			continue
		}
		models = append(models, m)
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("gemini provider: could not initialize any Gemini model from %v", modelNames)
	}

	return NewFallbackLLM(models), nil
}
