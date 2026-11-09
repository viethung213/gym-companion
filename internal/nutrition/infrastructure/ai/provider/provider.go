package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/config"
	"google.golang.org/adk/v2/model"
)

// Provider định nghĩa interface phích cắm (plug adapter) cho các AI Provider (Gemini, Groq, ...).
type Provider interface {
	Name() string
	Build(ctx context.Context, cfg *config.Config) (model.LLM, error)
}

// Registry quản lý danh sách các provider adapter được hỗ trợ.
type Registry struct {
	providers map[string]Provider
}

// NewDefaultRegistry khởi tạo Registry với đầy đủ các adapter mặc định có sẵn (Gemini, Groq).
func NewDefaultRegistry() *Registry {
	r := &Registry{
		providers: make(map[string]Provider),
	}
	r.Register(&GeminiProvider{})
	r.Register(&GroqProvider{})
	return r
}

// Register đăng ký thêm một provider adapter vào Registry.
func (r *Registry) Register(p Provider) {
	if p != nil {
		r.providers[strings.ToLower(strings.TrimSpace(p.Name()))] = p
	}
}

// Get tìm kiếm provider adapter theo tên.
func (r *Registry) Get(name string) (Provider, bool) {
	p, ok := r.providers[strings.ToLower(strings.TrimSpace(name))]
	return p, ok
}

// Build khởi tạo model.LLM từ provider tương ứng trong Registry dựa vào cfg.AIProvider.
func (r *Registry) Build(ctx context.Context, cfg *config.Config) (model.LLM, error) {
	providerName := strings.ToLower(strings.TrimSpace(cfg.AIProvider))
	if providerName == "" {
		providerName = "groq"
	}

	p, ok := r.Get(providerName)
	if !ok {
		return nil, fmt.Errorf("ai provider %q is not supported or not registered", providerName)
	}

	return p.Build(ctx, cfg)
}

// NewLLM là hàm tiện ích cấp cao khởi tạo model.LLM bằng DefaultRegistry.
func NewLLM(ctx context.Context, cfg *config.Config) (model.LLM, error) {
	registry := NewDefaultRegistry()
	return registry.Build(ctx, cfg)
}
