package adk

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

type SessionFilterContext struct {
	UserID           string
	LockoutRegistry  vo.LockoutRegistry
	UserRestrictions []string
	PlanDate         time.Time
}

type NutritionTools struct {
	foodRepo repository.FoodItemRepository
	mu       sync.RWMutex
	sessions map[string]SessionFilterContext
}

func NewNutritionTools(foodRepo repository.FoodItemRepository) *NutritionTools {
	return &NutritionTools{
		foodRepo: foodRepo,
		sessions: make(map[string]SessionFilterContext),
	}
}

//nolint:gocritic // hugeParam: sCtx value object mapping
func (t *NutritionTools) RegisterSessionContext(sessionID string, sCtx SessionFilterContext) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sessions[sessionID] = sCtx
}

func (t *NutritionTools) UnregisterSessionContext(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.sessions, sessionID)
}

func (t *NutritionTools) GetSessionContext(sessionID string) (SessionFilterContext, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	sCtx, ok := t.sessions[sessionID]
	return sCtx, ok
}

// FetchCatalogOptions chứa các tham số tùy chọn nâng cao khi tải catalog thực phẩm.
type FetchCatalogOptions struct {
	Limit      int      `json:"limit,omitempty"`
	Page       int      `json:"page,omitempty"`
	ExcludeIDs []string `json:"exclude_ids,omitempty"`
}

// FetchActiveFoodCatalog tải một nhóm nhỏ 5-8 nguyên liệu an toàn ngẫu nhiên/xoay vòng theo nhóm (PROTEIN, CARB, VEGGIE, SNACK).
// Tự động chuẩn hóa:
// 1. Loại bỏ 100% món chứa dị ứng của người dùng (UserRestrictions).
// 2. Loại bỏ 100% món bị khóa lặp chu kỳ 7 ngày (LockoutRegistry).
// 3. Loại bỏ các món nằm trong ExcludeIDs (các món xung đột ẩm thực hoặc bị Reviewer phản biện).
// 4. Hỗ trợ Page để lấy các đợt nguyên liệu tiếp theo khi đợt trước không hòa hợp với nguyên liệu tủ lạnh.
// 5. Giới hạn trả về 5-8 nguyên liệu ngẫu nhiên thay vì gửi toàn bộ DB, giúp tiết kiệm >90% token.
func (t *NutritionTools) FetchActiveFoodCatalog(ctx context.Context, sessionID, category string, opts ...FetchCatalogOptions) ([]FoodNutrientDTO, error) {
	catalog, err := t.foodRepo.FindActiveCatalog(ctx)
	if err != nil {
		return nil, fmt.Errorf("tools fetch active catalog: %w", err)
	}

	sCtx, _ := t.GetSessionContext(sessionID)
	planDate := sCtx.PlanDate
	if planDate.IsZero() {
		planDate = time.Now()
	}

	// 1. Lọc an toàn chuẩn hóa: Dị ứng + Chu kỳ lặp 7 ngày
	safeCatalog := service.FilterSafeFoods(catalog, sCtx.LockoutRegistry, sCtx.UserRestrictions, planDate)
	if len(safeCatalog) == 0 {
		return nil, nil
	}

	targetLimit := 6
	pageOpt := 1
	var excludeIDs []string

	if len(opts) > 0 {
		if opts[0].Limit > 0 && opts[0].Limit <= 10 {
			targetLimit = opts[0].Limit
		}
		if opts[0].Page > 1 {
			pageOpt = opts[0].Page
		}
		excludeIDs = opts[0].ExcludeIDs
	}

	// 2. Loại trừ các món được chỉ định không hợp vị (exclude_ids)
	if len(excludeIDs) > 0 {
		excludeMap := make(map[string]struct{}, len(excludeIDs))
		for _, id := range excludeIDs {
			trimmed := strings.ToLower(strings.TrimSpace(id))
			if trimmed != "" {
				excludeMap[trimmed] = struct{}{}
			}
		}
		if len(excludeMap) > 0 {
			filtered := make([]vo.FoodNutrient, 0, len(safeCatalog))
			for i := range safeCatalog {
				item := &safeCatalog[i]
				if _, excluded := excludeMap[strings.ToLower(item.ID())]; excluded {
					continue
				}
				if _, excluded := excludeMap[strings.ToLower(item.Name())]; excluded {
					continue
				}
				filtered = append(filtered, safeCatalog[i])
			}
			safeCatalog = filtered
		}
	}

	// 3. Phân loại theo nhóm dinh dưỡng
	proteins, carbs, veggies, snacks, others := service.CategorizeFoods(safeCatalog)

	catUpper := strings.ToUpper(strings.TrimSpace(category))
	var candidates []vo.FoodNutrient

	switch catUpper {
	case "PROTEIN":
		candidates = proteins
	case "CARB":
		candidates = carbs
	case "VEGGIE":
		candidates = veggies
	case "SNACK", "NUTIFOOD":
		candidates = snacks
	default:
		candidates = safeCatalog
	}

	if len(candidates) == 0 {
		candidates = others
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	// 4. Xáo trộn và lấy mẫu ngẫu nhiên tất định có phân trang
	seedKey := fmt.Sprintf("%s_%s", sCtx.UserID, catUpper)
	rng := service.NewDeterministicRNG(seedKey, planDate)
	sampled := service.SamplePaged(candidates, targetLimit, pageOpt, rng)

	return ToFoodNutrientDTOs(sampled), nil
}

func (t *NutritionTools) CheckLockoutRules(
	_ context.Context,
	lockoutRegistry vo.LockoutRegistry,
	ingredients []string,
) (collisions, warnings []string) {
	return lockoutRegistry.CheckCollisions(ingredients, time.Now())
}

func (t *NutritionTools) CalculateMacroGramRatio(
	targetCalories float64,
	proteinRatio, carbRatio, fatRatio float64,
) (proteinGrams, carbGrams, fatGrams float64) {
	if proteinRatio <= 0 {
		proteinRatio = 0.45
	}
	if carbRatio <= 0 {
		carbRatio = 0.45
	}
	if fatRatio <= 0 {
		fatRatio = 0.10
	}

	proteinGrams = (targetCalories * proteinRatio) / 4.0
	carbGrams = (targetCalories * carbRatio) / 4.0
	fatGrams = (targetCalories * fatRatio) / 9.0
	return proteinGrams, carbGrams, fatGrams
}

func (t *NutritionTools) SuggestNutiFoodSupplement(ctx context.Context) ([]FoodNutrientDTO, error) {
	products, err := t.foodRepo.FindNutiFoodProducts(ctx)
	if err != nil {
		return nil, fmt.Errorf("tools suggest nutifood products: %w", err)
	}
	return ToFoodNutrientDTOs(products), nil
}
