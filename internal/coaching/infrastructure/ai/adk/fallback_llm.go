package adk

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log"
	"strings"

	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/config"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/model/openaimodel"
	"google.golang.org/genai"
)

// Compile-time interface check.
var _ model.LLM = (*FallbackLLM)(nil)

// FallbackLLM wraps a list of model.LLM instances and automatically falls back
// to a secondary model when the current model returns a Rate Limit (429) or
// Quota Exceeded error.
type FallbackLLM struct {
	models []model.LLM
}

// NewFallbackLLMFromEnv creates a FallbackLLM reading model list and API Key from Coaching module config.
func NewFallbackLLMFromEnv(ctx context.Context) (model.LLM, error) {
	cfg := config.LoadConfig()
	if cfg.AIProvider == "groq" {
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

		return NewGroqFallbackLLM(ctx, modelNames, cfg.GroqAPIKey, cfg.GroqBaseURL)
	}

	rawNames := append([]string{cfg.GeminiModel}, strings.Split(cfg.GeminiFallbackModels, ",")...)
	modelNames := make([]string, 0, len(rawNames))
	seen := make(map[string]bool)

	for _, name := range rawNames {
		trimmed := strings.TrimSpace(name)
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			modelNames = append(modelNames, trimmed)
		}
	}

	return NewFallbackLLM(ctx, modelNames, cfg.GoogleAPIKey)
}

// NewGroqFallbackLLM initializes a FallbackLLM from a slice of Groq model names, API key, and baseURL.
func NewGroqFallbackLLM(ctx context.Context, modelNames []string, apiKey, baseURL string) (model.LLM, error) {
	if len(modelNames) == 0 {
		return nil, errors.New("fallback llm: modelNames list cannot be empty")
	}

	clientCfg := &openaimodel.ClientConfig{
		APIKey:  apiKey,
		BaseURL: baseURL,
	}

	models := make([]model.LLM, 0, len(modelNames))
	for _, name := range modelNames {
		m, err := openaimodel.NewModel(ctx, name, clientCfg)
		if err != nil {
			log.Printf("[FallbackLLM] Warning: Failed to initialize Groq model %s: %v", name, err)
			continue
		}
		models = append(models, m)
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("fallback llm: could not initialize any Groq model from %v", modelNames)
	}

	return &FallbackLLM{models: models}, nil
}

// NewFallbackLLM initializes a FallbackLLM from a slice of model names and an API key.
func NewFallbackLLM(ctx context.Context, modelNames []string, apiKey string) (model.LLM, error) {
	if len(modelNames) == 0 {
		return nil, errors.New("fallback llm: modelNames list cannot be empty")
	}

	var clientCfg *genai.ClientConfig
	if apiKey != "" {
		clientCfg = &genai.ClientConfig{APIKey: apiKey}
	}

	models := make([]model.LLM, 0, len(modelNames))
	for _, name := range modelNames {
		m, err := gemini.NewModel(ctx, name, clientCfg)
		if err != nil {
			log.Printf("[FallbackLLM] Warning: Failed to initialize model %s: %v", name, err)
			continue
		}
		models = append(models, m)
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("fallback llm: could not initialize any Gemini model from %v", modelNames)
	}

	return &FallbackLLM{models: models}, nil
}

// Name returns the name of the primary model.
func (f *FallbackLLM) Name() string {
	if len(f.models) == 0 {
		return "fallback-llm-empty"
	}
	return f.models[0].Name()
}

// GenerateContent executes LLM generation with automatic fallback if 429 / Quota Exceeded occurs.
func (f *FallbackLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		for idx, m := range f.models {
			modelReq := req
			if req != nil {
				reqCopy := *req
				reqCopy.Model = m.Name()
				modelReq = &reqCopy
			}

			var lastErr error
			var receivedAny bool

			for resp, err := range m.GenerateContent(ctx, modelReq, stream) {
				if err != nil {
					lastErr = err
					break
				}
				receivedAny = true
				if !yield(resp, nil) {
					return
				}
			}

			if lastErr == nil {
				return
			}

			if receivedAny {
				yield(nil, lastErr)
				return
			}

			if isQuotaOrRateLimitErr(lastErr) && idx < len(f.models)-1 {
				nextModel := f.models[idx+1]
				log.Printf("[FallbackLLM] Model %s hit quota/rate limit error (%v). Auto-falling back to %s...",
					m.Name(), lastErr, nextModel.Name())
				continue
			}

			yield(nil, fmt.Errorf("model %s error: %w", m.Name(), lastErr))
			return
		}
	}
}

// isQuotaOrRateLimitErr checks if err is a 429 / 413 / Quota Exceeded / Rate Limit error.
func isQuotaOrRateLimitErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "413") ||
		strings.Contains(msg, "resource_exhausted") ||
		strings.Contains(msg, "quota") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "rate_limit") ||
		strings.Contains(msg, "tokens per minute") ||
		strings.Contains(msg, "tpm") ||
		strings.Contains(msg, "too many requests")
}
