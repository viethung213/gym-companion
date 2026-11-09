package provider

import (
	"context"
	"errors"
	"iter"
	"strings"
	"testing"

	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/config"
	"google.golang.org/adk/v2/model"
)

type mockLLM struct {
	name      string
	responses []*model.LLMResponse
	err       error
	called    int
}

func (m *mockLLM) Name() string { return m.name }

func (m *mockLLM) GenerateContent(_ context.Context, _ *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		m.called++
		if m.err != nil {
			yield(nil, m.err)
			return
		}
		for _, resp := range m.responses {
			if !yield(resp, nil) {
				return
			}
		}
	}
}

func TestNewDefaultRegistry(t *testing.T) {
	r := NewDefaultRegistry()
	if r == nil {
		t.Fatal("expected non-nil Registry")
	}

	groq, ok := r.Get("groq")
	if !ok || groq == nil || groq.Name() != "groq" {
		t.Fatalf("expected groq provider registered, got %v", groq)
	}

	gemini, ok := r.Get("gemini")
	if !ok || gemini == nil || gemini.Name() != "gemini" {
		t.Fatalf("expected gemini provider registered, got %v", gemini)
	}
}

type dummyCustomProvider struct{}

func (d *dummyCustomProvider) Name() string { return "custom" }
func (d *dummyCustomProvider) Build(_ context.Context, _ *config.Config) (model.LLM, error) {
	return &mockLLM{name: "custom-model"}, nil
}

func TestRegistry_RegisterCustom(t *testing.T) {
	r := NewDefaultRegistry()
	r.Register(&dummyCustomProvider{})

	p, ok := r.Get("custom")
	if !ok || p == nil {
		t.Fatal("expected custom provider to be registered")
	}

	llm, err := r.Build(context.Background(), &config.Config{AIProvider: "custom"})
	if err != nil {
		t.Fatalf("unexpected error building custom provider: %v", err)
	}
	if llm.Name() != "custom-model" {
		t.Fatalf("got model name %q, want custom-model", llm.Name())
	}
}

func TestRegistry_Build_Unsupported(t *testing.T) {
	r := NewDefaultRegistry()
	_, err := r.Build(context.Background(), &config.Config{AIProvider: "unknown_provider"})
	if err == nil {
		t.Fatal("expected error for unsupported provider, got nil")
	}
}

func TestGroqProvider_Build(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{
		GroqModel:          "openai/gpt-oss-120b",
		GroqFallbackModels: "openai/gpt-oss-20b",
		GroqAPIKey:         "dummy-key",
		GroqBaseURL:        "https://api.groq.com/openai/v1",
	}

	p := &GroqProvider{}
	llm, err := p.Build(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to build groq provider: %v", err)
	}
	if llm.Name() != "openai/gpt-oss-120b" {
		t.Fatalf("got name %q, want openai/gpt-oss-120b", llm.Name())
	}
}

func TestGeminiProvider_Build(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{
		GeminiModel:          "gemini-2.5-flash",
		GeminiFallbackModels: "gemini-2.0-flash",
		GoogleAPIKey:         "dummy-key",
	}

	p := &GeminiProvider{}
	llm, err := p.Build(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to build gemini provider: %v", err)
	}
	if llm.Name() != "gemini-2.5-flash" {
		t.Fatalf("got name %q, want gemini-2.5-flash", llm.Name())
	}
}

func TestFallbackLLM_SuccessOnPrimary(t *testing.T) {
	ctx := context.Background()
	model1 := &mockLLM{name: "model-1", responses: []*model.LLMResponse{{}}}
	model2 := &mockLLM{name: "model-2", responses: []*model.LLMResponse{{}}}

	fallback := NewFallbackLLM([]model.LLM{model1, model2})

	var resps []*model.LLMResponse
	var errs []error

	for resp, err := range fallback.GenerateContent(ctx, &model.LLMRequest{}, false) {
		if err != nil {
			errs = append(errs, err)
		} else {
			resps = append(resps, resp)
		}
	}

	if len(errs) > 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	if model1.called != 1 {
		t.Fatalf("expected model1 called 1 time, got %d", model1.called)
	}
	if model2.called != 0 {
		t.Fatalf("expected model2 called 0 times, got %d", model2.called)
	}
}

func TestFallbackLLM_FallbackOn429(t *testing.T) {
	ctx := context.Background()
	model1 := &mockLLM{name: "model-1", err: errors.New("googleapi: Error 429: ResourceExhausted")}
	model2 := &mockLLM{name: "model-2", responses: []*model.LLMResponse{{}}}

	fallback := NewFallbackLLM([]model.LLM{model1, model2})

	var resps []*model.LLMResponse
	var errs []error

	for resp, err := range fallback.GenerateContent(ctx, &model.LLMRequest{}, false) {
		if err != nil {
			errs = append(errs, err)
		} else {
			resps = append(resps, resp)
		}
	}

	if len(errs) > 0 {
		t.Fatalf("expected no errors after fallback, got %v", errs)
	}
	if len(resps) != 1 {
		t.Fatalf("expected 1 response from fallback model, got %d", len(resps))
	}
	if model1.called != 1 {
		t.Fatalf("expected model1 called 1 time, got %d", model1.called)
	}
	if model2.called != 1 {
		t.Fatalf("expected model2 called 1 time on fallback, got %d", model2.called)
	}
}

func TestFallbackLLM_AllModelsFail(t *testing.T) {
	ctx := context.Background()
	model1 := &mockLLM{name: "model-1", err: errors.New("429 Too Many Requests")}
	model2 := &mockLLM{name: "model-2", err: errors.New("429 ResourceExhausted")}

	fallback := NewFallbackLLM([]model.LLM{model1, model2})

	var errs []error
	for _, err := range fallback.GenerateContent(ctx, &model.LLMRequest{}, false) {
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) == 0 {
		t.Fatalf("expected error when all models fail")
	}
	if !strings.Contains(errs[0].Error(), "model-2") {
		t.Fatalf("expected error message to contain final model name, got: %v", errs[0])
	}
}
