package provider

import (
	"context"
	"fmt"
	"iter"
	"log"
	"strings"
	"time"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

// Compile-time interface check.
var _ model.LLM = (*FallbackLLM)(nil)

// FallbackLLM bọc một danh sách các model.LLM và tự động fallback sang model dự phòng
// khi model hiện tại trả về lỗi Rate Limit (429) hoặc Resource Exhausted / Quota Exceeded.
type FallbackLLM struct {
	models []model.LLM
}

// NewFallbackLLM khởi tạo FallbackLLM bọc danh sách các model.LLM.
func NewFallbackLLM(models []model.LLM) *FallbackLLM {
	return &FallbackLLM{models: models}
}

// Name trả về tên của model đầu tiên (primary model).
func (f *FallbackLLM) Name() string {
	if len(f.models) == 0 {
		return "fallback-llm-empty"
	}
	return f.models[0].Name()
}

// Models trả về bản sao danh sách model bên trong.
func (f *FallbackLLM) Models() []model.LLM {
	cp := make([]model.LLM, len(f.models))
	copy(cp, f.models)
	return cp
}

// GenerateContent thực hiện gọi LLM với cơ chế tự động thử model dự phòng nếu bị 429 / Quota Exceeded.
func (f *FallbackLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		for idx, m := range f.models {
			modelReq := req
			if req != nil {
				reqCopy := *req
				reqCopy.Model = m.Name()
				if reqCopy.Config == nil {
					reqCopy.Config = &genai.GenerateContentConfig{MaxOutputTokens: 3500}
				} else if reqCopy.Config.MaxOutputTokens <= 0 || reqCopy.Config.MaxOutputTokens > 4000 {
					cfgCopy := *reqCopy.Config
					cfgCopy.MaxOutputTokens = 3500
					reqCopy.Config = &cfgCopy
				}
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
				return // Gọi thành công hoàn toàn
			}

			// Nếu đã nhận được 1 phần kết quả stream trước khi có lỗi, không thử fallback nữa để tránh trùng dữ liệu
			if receivedAny {
				yield(nil, lastErr)
				return
			}

			// Kiểm tra nếu lỗi do Quota / Rate limit (429) và còn model dự phòng
			if isQuotaOrRateLimitErr(lastErr) {
				if idx < len(f.models)-1 {
					nextModel := f.models[idx+1]
					log.Printf("[FallbackLLM] Model %s hit quota/rate limit error (%v). Auto-falling back to %s after 1s...",
						m.Name(), lastErr, nextModel.Name())
					time.Sleep(1 * time.Second)
					continue
				}

				// Nếu là model cuối cùng mà bị 429, chờ 2s rồi thử lại 1 lần cuối
				log.Printf("[FallbackLLM] Model %s hit rate limit (429). Retrying after 2s cooldown...", m.Name())
				time.Sleep(2 * time.Second)
				var retryErr error
				for resp, err := range m.GenerateContent(ctx, modelReq, stream) {
					if err != nil {
						retryErr = err
						break
					}
					if !yield(resp, nil) {
						return
					}
				}
				if retryErr == nil {
					return
				}
			}

			// Nếu không phải lỗi 429 hoặc đã hết model dự phòng, trả về lỗi
			yield(nil, fmt.Errorf("model %s error: %w", m.Name(), lastErr))
			return
		}
	}
}

// isQuotaOrRateLimitErr kiểm tra xem lỗi có thuộc loại tạm thời (503, high demand, server overload) hoặc 429 / Quota Exceeded hay không.
func isQuotaOrRateLimitErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "413") ||
		strings.Contains(msg, "503") ||
		strings.Contains(msg, "unavailable") ||
		strings.Contains(msg, "high demand") ||
		strings.Contains(msg, "overloaded") ||
		strings.Contains(msg, "resource_exhausted") ||
		strings.Contains(msg, "quota") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "rate_limit") ||
		strings.Contains(msg, "tokens per minute") ||
		strings.Contains(msg, "tpm") ||
		strings.Contains(msg, "too many requests")
}
