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
	model1 := &mockLLM{name: "gemini-3.5-flash-lite", responses: []*model.LLMResponse{{}}}
	model2 := &mockLLM{name: "gemini-2.5-flash", responses: []*model.LLMResponse{{}}}

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
		t.Fatalf("got errs = %v, want no errors", errs)
	}
	if len(resps) != 1 {
		t.Fatalf("got len(resps) = %d, want 1", len(resps))
	}
	if model1.called != 1 {
		t.Fatalf("got model1.called = %d, want 1", model1.called)
	}
	if model2.called != 0 {
		t.Fatalf("got model2.called = %d, want 0", model2.called)
	}
}

func TestFallbackLLM_FallbackOn429(t *testing.T) {
	ctx := context.Background()
	model1 := &mockLLM{name: "gemini-3.5-flash-lite", err: errors.New("googleapi: Error 429: ResourceExhausted")}
	model2 := &mockLLM{name: "gemini-2.5-flash", responses: []*model.LLMResponse{{}}}

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
		t.Fatalf("got errs = %v, want no errors after fallback", errs)
	}
	if len(resps) != 1 {
		t.Fatalf("got len(resps) = %d, want 1", len(resps))
	}
	if model1.called != 1 {
		t.Fatalf("got model1.called = %d, want 1", model1.called)
	}
	if model2.called != 1 {
		t.Fatalf("got model2.called = %d, want 1", model2.called)
	}
}

func TestFallbackLLM_AllModelsFail(t *testing.T) {
	ctx := context.Background()
	model1 := &mockLLM{name: "gemini-3.5-flash-lite", err: errors.New("429 Too Many Requests")}
	model2 := &mockLLM{name: "gemini-2.5-flash", err: errors.New("429 ResourceExhausted")}

	fallback := &FallbackLLM{models: []model.LLM{model1, model2}}

	var errs []error
	for _, err := range fallback.GenerateContent(ctx, &model.LLMRequest{}, false) {
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) == 0 {
		t.Fatalf("got no error, want error when all models fail")
	}
	if !strings.Contains(errs[0].Error(), "gemini-2.5-flash") {
		t.Fatalf("got err = %v, want error to contain final model name", errs[0])
	}
}
