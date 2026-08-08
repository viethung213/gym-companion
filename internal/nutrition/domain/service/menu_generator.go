package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

// MenuGenerator orchestrates thực đơn: check Recipe Cache trước, nếu miss thì gọi AI Agent.
// CombinatorialMatrix đã loại bỏ hoàn toàn — AI tự chọn combo và tính Gram Macro.
type MenuGenerator struct {
	hashFn      func(protein, carb, veggie, style string) string
	recipeCache repository.RecipeCacheRepository
	foodRepo    repository.FoodItemRepository
	aiService   repository.AIService
}

func NewMenuGenerator(
	matrix *CombinatorialMatrix,
	recipeCache repository.RecipeCacheRepository,
	foodRepo repository.FoodItemRepository,
	aiService repository.AIService,
) *MenuGenerator {
	return &MenuGenerator{
		hashFn:      matrix.ComputeIngredientHash,
		recipeCache: recipeCache,
		foodRepo:    foodRepo,
		aiService:   aiService,
	}
}

// mealSlot định nghĩa tên bữa và tỷ lệ Calo — AI Agent tự quyết định phân bổ Macro bên trong.
type mealSlot struct {
	name       string
	percentage float64
}

func getDailyMealSlots() []mealSlot {
	return []mealSlot{
		{"Breakfast", 0.25},
		{"Lunch", 0.35},
		{"Dinner", 0.30},
		{"Snack", 0.10},
	}
}

// GenerateDailyPlan sinh thực đơn 4 bữa cho userID.
// Luồng: check cache → hit: trả ngay | miss: gọi AI → lưu cache.
func (g *MenuGenerator) GenerateDailyPlan(
	ctx context.Context,
	userID string,
	planDate time.Time,
	allocation vo.CalorieAllocation,
	lockoutRegistry vo.LockoutRegistry,
	userRestrictions []string,
) (*aggregate.NutritionPlan, error) {
	activeCatalog, err := g.foodRepo.FindActiveCatalog(ctx)
	if err != nil {
		return nil, fmt.Errorf("menu generator fetch catalog: %w", err)
	}

	slots := getDailyMealSlots()
	dailyMeals := make([]aggregate.DailyMeal, 0, len(slots))

	currentLockout := lockoutRegistry
	for _, slot := range slots {
		targetMealCalo := allocation.TargetCalories() * slot.percentage

		promptCtx := repository.AIMenuPromptContext{
			UserID:               userID,
			MealType:             slot.name,
			TargetMealCalories:   targetMealCalo,
			AvailableIngredients: activeCatalog,
			UserRestrictions:     userRestrictions,
			PlanDate:             planDate,
		}

		options, optErr := g.resolveOptionsViaAI(ctx, promptCtx, currentLockout, targetMealCalo)
		if optErr != nil {
			return nil, fmt.Errorf("menu generator slot %s: %w", slot.name, optErr)
		}

		// Tự động khóa các nguyên liệu đã dùng ở các bữa trước đó trong ngày để các bữa sau (Sáng -> Trưa -> Tối -> Phụ) chọn các bộ nguyên liệu mới
		for _, opt := range options {
			for _, ing := range opt.Ingredients() {
				currentLockout = currentLockout.ApplyLockout(vo.LockoutTypeProtein, ing.IngredientName(), 24*time.Hour, planDate)
			}
		}

		dailyMeals = append(dailyMeals, aggregate.NewDailyMeal(slot.name, options))
	}

	planID := uuid.New().String()
	return aggregate.NewNutritionPlan(planID, userID, planDate, allocation, dailyMeals), nil
}

// GeneratePlanWithPantry sinh thực đơn từ nguyên liệu trong tủ lạnh của người dùng.
// Logic:
// 1. Phân loại nguyên liệu người dùng gửi lên theo nhóm (PROTEIN, CARB, VEGGIE).
// 2. Nếu THIẾU nhóm nào, tự động lấy các thực phẩm thuộc nhóm đó từ DB activeCatalog để bổ sung.
// 3. Nếu KHÔNG THIẾU nhóm nào, sử dụng CHÍNH NGUYÊN LIỆU người dùng gửi lên.
// 4. Tuyệt đối không thay thế nguyên liệu người dùng bằng nguyên liệu tương đương.
func (g *MenuGenerator) GeneratePlanWithPantry(
	ctx context.Context,
	userID string,
	planDate time.Time,
	allocation vo.CalorieAllocation,
	lockoutRegistry vo.LockoutRegistry,
	userIngredients []string,
) (*aggregate.NutritionPlan, error) {
	activeCatalog, err := g.foodRepo.FindActiveCatalog(ctx)
	if err != nil {
		return nil, fmt.Errorf("menu generator fetch catalog: %w", err)
	}

	pantryCatalog := make([]vo.FoodNutrient, 0, len(userIngredients))

	for _, ingName := range userIngredients {
		trimmed := strings.TrimSpace(ingName)
		if trimmed == "" {
			continue
		}

		lower := strings.ToLower(trimmed)
		var foundItem vo.FoodNutrient
		var isFound bool
		for i := range activeCatalog {
			actLower := strings.ToLower(activeCatalog[i].Name())
			if actLower == lower || strings.Contains(actLower, lower) || strings.Contains(lower, actLower) {
				foundItem = activeCatalog[i]
				isFound = true
				break
			}
		}

		if !isFound {
			// Nguyên liệu mới -> Tạo đối tượng gửi sang để AI tự nhận diện 100% nhóm thực phẩm (PROTEIN/CARB/VEGGIE) và khai báo new_food_catalog_items
			foundItem = vo.NewFoodNutrient(
				uuid.New().String(),
				trimmed,
				"INGREDIENT",
				100.0, 10.0, 10.0, 2.0,
				nil, "", "", false,
			)
		}
		pantryCatalog = append(pantryCatalog, foundItem)
	}

	// Gọi AI 1 lần duy nhất để sinh đúng 2 công thức món ăn từ tủ lạnh (tiết kiệm token và giảm thời gian chờ)
	targetCalo := allocation.TargetCalories() * 0.35
	promptCtx := repository.AIMenuPromptContext{
		UserID:               userID,
		MealType:             "PANTRY_RECIPE",
		TargetMealCalories:   targetCalo,
		AvailableIngredients: pantryCatalog,
		UserRestrictions:     nil,
		PlanDate:             planDate,
	}

	options, optErr := g.resolveOptionsViaAI(ctx, promptCtx, lockoutRegistry, targetCalo)
	if optErr != nil {
		return nil, fmt.Errorf("menu generator pantry: %w", optErr)
	}

	// Giới hạn đúng 2 công thức món ăn duy nhất
	if len(options) > 2 {
		options = options[:2]
	}

	slots := getDailyMealSlots()
	dailyMeals := make([]aggregate.DailyMeal, 0, len(slots))

	for i, slot := range slots {
		slotOptions := options
		if len(options) == 2 {
			if i%2 == 0 {
				slotOptions = []aggregate.MealOption{options[0]}
			} else {
				slotOptions = []aggregate.MealOption{options[1]}
			}
		}
		dailyMeals = append(dailyMeals, aggregate.NewDailyMeal(slot.name, slotOptions))
	}

	planID := uuid.New().String()
	return aggregate.NewNutritionPlan(planID, userID, planDate, allocation, dailyMeals), nil
}

// resolveOptionsViaAI gọi AI Agent để sinh options, sau đó lưu Recipe Cache cho lần sau.
//
//nolint:gocritic // promptCtx parameter struct is passed by value
func (g *MenuGenerator) resolveOptionsViaAI(
	ctx context.Context,
	promptCtx repository.AIMenuPromptContext,
	lockoutRegistry vo.LockoutRegistry,
	targetMealCalo float64,
) ([]aggregate.MealOption, error) {
	if g.aiService == nil {
		return nil, errors.New("ai service is not configured")
	}

	aiResults, err := g.aiService.SelectCreativeMealOptions(ctx, promptCtx, lockoutRegistry)
	if err != nil {
		return nil, fmt.Errorf("ai select meal options: %w", err)
	}

	options := make([]aggregate.MealOption, 0, len(aiResults))
	for _, res := range aiResults {
		// Lưu Recipe Cache nếu có đủ ít nhất 3 nguyên liệu để tính hash.
		if len(res.SupplementaryIngredients) >= 3 {
			p := res.SupplementaryIngredients[0]
			c := res.SupplementaryIngredients[1]
			v := res.SupplementaryIngredients[2]
			hash := g.hashFn(p.IngredientName(), c.IngredientName(), v.IngredientName(), "ai-generated")

			cached, findErr := g.recipeCache.FindByHash(ctx, hash)
			if findErr != nil || cached == nil {
				toCache := &repository.CachedRecipe{
					ID:                       uuid.New().String(),
					IngredientHash:           hash,
					RecipeName:               res.RecipeName,
					CookingStyle:             "ai-generated",
					Ingredients:              res.SupplementaryIngredients,
					CookingSteps:             res.CookingSteps,
					SupplementaryIngredients: nil,
					CreatedAt:                time.Now(),
				}
				_ = g.recipeCache.Save(ctx, toCache)
			}
		}

		// NewFoodCatalogItems đã được agent tự lưu vào DB — không cần append vào allIngs.
		allIngs := make([]aggregate.IngredientGram, 0, len(res.SupplementaryIngredients))
		allIngs = append(allIngs, res.SupplementaryIngredients...)

		// Macro được AI tính sẵn — map trực tiếp vào MealOption.
		options = append(options, aggregate.NewMealOption(
			uuid.New().String(),
			res.RecipeName,
			targetMealCalo,
			res.TotalProteinGrams,
			res.TotalCarbGrams,
			res.TotalFatGrams,
			allIngs,
			res.CookingSteps,
			false,
		))
	}

	return options, nil
}
