package adk

import (
	"context"
	"errors"
	"iter"
	"strings"
	"testing"

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

func TestFallbackLLM_SuccessOnPrimary(t *testing.T) {
	ctx := context.Background()
	model1 := &mockLLM{name: "gemini-2.5-flash", responses: []*model.LLMResponse{{}}}
	model2 := &mockLLM{name: "gemini-2.0-flash", responses: []*model.LLMResponse{{}}}

	fallback := &FallbackLLM{models: []model.LLM{model1, model2}}

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
	model1 := &mockLLM{name: "gemini-2.5-flash", err: errors.New("googleapi: Error 429: ResourceExhausted")}
	model2 := &mockLLM{name: "gemini-2.0-flash", responses: []*model.LLMResponse{{}}}

	fallback := &FallbackLLM{models: []model.LLM{model1, model2}}

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
	model1 := &mockLLM{name: "gemini-2.5-flash", err: errors.New("429 Too Many Requests")}
	model2 := &mockLLM{name: "gemini-2.0-flash", err: errors.New("429 ResourceExhausted")}

	fallback := &FallbackLLM{models: []model.LLM{model1, model2}}

	var errs []error
	for _, err := range fallback.GenerateContent(ctx, &model.LLMRequest{}, false) {
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) == 0 {
		t.Fatalf("expected error when all models fail")
	}
	if !strings.Contains(errs[0].Error(), "gemini-2.0-flash") {
		t.Fatalf("expected error message to contain final model name, got: %v", errs[0])
	}
}
