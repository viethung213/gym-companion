package adk

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log"
	"os"
	"strings"

	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/gemini"
)

// FallbackLLM bọc một danh sách các model.LLM và tự động fallback sang model dự phòng
// khi model hiện tại trả về lỗi Rate Limit (429) hoặc Resource Exhausted / Quota Exceeded.
type FallbackLLM struct {
	models []model.LLM
}

// NewFallbackLLMFromEnv tạo FallbackLLM đọc danh sách model từ biến môi trường
// GEMINI_MODEL (mặc định: gemini-2.5-flash) và GEMINI_FALLBACK_MODELS (mặc định: gemini-2.0-flash,gemini-2.5-flash-lite,gemini-1.5-flash).
func NewFallbackLLMFromEnv(ctx context.Context) (model.LLM, error) {
	primaryModel := os.Getenv("GEMINI_MODEL")
	if primaryModel == "" {
		primaryModel = "gemini-2.5-flash"
	}

	fallbackStr := os.Getenv("GEMINI_FALLBACK_MODELS")
	if fallbackStr == "" {
		fallbackStr = "gemini-2.0-flash,gemini-2.5-flash-lite,gemini-1.5-flash"
	}

	rawNames := append([]string{primaryModel}, strings.Split(fallbackStr, ",")...)
	modelNames := make([]string, 0, len(rawNames))
	seen := make(map[string]bool)

	for _, name := range rawNames {
		trimmed := strings.TrimSpace(name)
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			modelNames = append(modelNames, trimmed)
		}
	}

	return NewFallbackLLM(ctx, modelNames)
}

// NewFallbackLLM khởi tạo FallbackLLM từ danh sách tên model.
func NewFallbackLLM(ctx context.Context, modelNames []string) (model.LLM, error) {
	if len(modelNames) == 0 {
		return nil, errors.New("fallback llm: modelNames list cannot be empty")
	}

	models := make([]model.LLM, 0, len(modelNames))
	for _, name := range modelNames {
		m, err := gemini.NewModel(ctx, name, nil)
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

// Name trả về tên của model đầu tiên (primary model).
func (f *FallbackLLM) Name() string {
	if len(f.models) == 0 {
		return "fallback-llm-empty"
	}
	return f.models[0].Name()
}

// GenerateContent thực hiện gọi LLM với cơ chế tự động thử model dự phòng nếu bị 429 / Quota Exceeded.
func (f *FallbackLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		for idx, m := range f.models {
			var lastErr error
			var receivedAny bool

			for resp, err := range m.GenerateContent(ctx, req, stream) {
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
				return // Gọi thành công hoàn toàn
			}

			// Nếu đã nhận được 1 phần kết quả stream trước khi có lỗi, không thử fallback nữa để tránh trùng dữ liệu
			if receivedAny {
				yield(nil, lastErr)
				return
			}

			// Kiểm tra nếu lỗi do Quota / Rate limit (429) và còn model dự phòng
			if isQuotaOrRateLimitErr(lastErr) && idx < len(f.models)-1 {
				nextModel := f.models[idx+1]
				log.Printf("[FallbackLLM] Model %s hit quota/rate limit error (%v). Auto-falling back to %s...",
					m.Name(), lastErr, nextModel.Name())
				continue
			}

			// Nếu không phải lỗi 429 hoặc đã hết model dự phòng, trả về lỗi
			yield(nil, fmt.Errorf("model %s error: %w", m.Name(), lastErr))
			return
		}
	}
}

// isQuotaOrRateLimitErr kiểm tra xem lỗi có thuộc loại 429 / Quota Exceeded / Rate Limit hay không.
func isQuotaOrRateLimitErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "resource_exhausted") ||
		strings.Contains(msg, "quota") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "too many requests")
}
