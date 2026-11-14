package provider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/config"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/openaimodel"
)

// Compile-time interface check.
var _ Provider = (*GroqProvider)(nil)

// GroqProvider triển khai phích cắm cho Groq API (OpenAI-compatible).
type GroqProvider struct{}

// Name trả về định danh của provider.
func (g *GroqProvider) Name() string {
	return "groq"
}

// Build khởi tạo model.LLM cho Groq (có bọc FallbackLLM nếu nhiều model).
func (g *GroqProvider) Build(ctx context.Context, cfg *config.Config) (model.LLM, error) {
	rawNames := append([]string{cfg.GroqModel}, strings.Split(cfg.GroqFallbackModels, ",")...)
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
		return nil, errors.New("groq provider: modelNames list cannot be empty")
	}

	clientCfg := &openaimodel.ClientConfig{
		APIKey:  cfg.GroqAPIKey,
		BaseURL: cfg.GroqBaseURL,
	}

	models := make([]model.LLM, 0, len(modelNames))
	for _, name := range modelNames {
		m, err := openaimodel.NewModel(ctx, name, clientCfg)
		if err != nil {
			slog.WarnContext(ctx, "Failed to initialize Groq model", slog.String("model", name), slog.Any("error", err))
			continue
		}
		models = append(models, m)
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("groq provider: could not initialize any Groq model from %v", modelNames)
	}

	return NewFallbackLLM(models), nil
}
