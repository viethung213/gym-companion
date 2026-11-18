package simulation_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

type simCacheRepo struct {
	recipes map[string]*repository.CachedRecipe
}

func (m *simCacheRepo) FindByHash(_ context.Context, hash string) (*repository.CachedRecipe, error) {
	return m.recipes[hash], nil
}

func (m *simCacheRepo) Save(_ context.Context, recipe *repository.CachedRecipe) error {
	m.recipes[recipe.IngredientHash] = recipe
	return nil
}

type simFoodRepo struct {
	catalog []vo.FoodNutrient
}

func (m *simFoodRepo) FindByID(_ context.Context, _ string) (*aggregate.FoodItem, error) {
	return nil, nil
}
func (m *simFoodRepo) FindByName(_ context.Context, _ string) (*aggregate.FoodItem, error) {
	return nil, nil
}
func (m *simFoodRepo) FindActiveCatalog(_ context.Context) ([]vo.FoodNutrient, error) {
	return m.catalog, nil
}
func (m *simFoodRepo) FindNutiFoodProducts(_ context.Context) ([]vo.FoodNutrient, error) {
	return nil, nil
}
func (m *simFoodRepo) Save(_ context.Context, _ *aggregate.FoodItem) error   { return nil }
func (m *simFoodRepo) Update(_ context.Context, _ *aggregate.FoodItem) error { return nil }

// mockAIServiceForSim giả lập AIService sinh thực đơn đáp ứng theo nguyên tắc cá nhân hóa.
type mockAIServiceForSim struct {
	capturedPromptCtx repository.AIMenuPromptContext
}

func (m *mockAIServiceForSim) SelectCreativeMealOptions(
	_ context.Context,
	promptCtx repository.AIMenuPromptContext,
) ([]repository.GeneratedRecipeResult, error) {
	m.capturedPromptCtx = promptCtx

	// Xây dựng danh mục các món có sẵn dạng lookup map
	availableMap := make(map[string]vo.FoodNutrient)
	for _, f := range promptCtx.AvailableIngredients {
		availableMap[strings.ToLower(strings.TrimSpace(f.Name()))] = f
	}

	// Xác định nguyên liệu protein & carb để tạo option
	proteinName := "Thịt heo nạc"
	carbName := "Khoai lang luộc"
	veggieName := "Rau muống luộc"

	// Nếu có preferred ingredients và có trong available -> ưu tiên chọn
	for _, p := range promptCtx.PreferredIngredients {
		pLower := strings.ToLower(strings.TrimSpace(p))
		if _, ok := availableMap[pLower]; ok {
			if strings.Contains(pLower, "gà") || strings.Contains(pLower, "trứng") {
				proteinName = p
			} else if strings.Contains(pLower, "cơm") || strings.Contains(pLower, "khoai") {
				carbName = p
			}
		}
	}

	// Phong cách nấu ăn: nếu có WeeklyCookingStyle thì áp dụng ~70%
	cookingStyle := "Luộc thanh đạm"
	if promptCtx.WeeklyCookingStyle != "" {
		cookingStyle = promptCtx.WeeklyCookingStyle
	}

	recipe1 := repository.GeneratedRecipeResult{
		RecipeName:   proteinName + " " + cookingStyle,
		CookingSteps: []string{"Sơ chế sạch nguyên liệu", "Thực hiện theo phong cách: " + cookingStyle},
		SupplementaryIngredients: []aggregate.IngredientGram{
			aggregate.NewIngredientGram(proteinName, 150.0, false),
			aggregate.NewIngredientGram(carbName, 200.0, false),
			aggregate.NewIngredientGram(veggieName, 100.0, false),
		},
		TotalProteinGrams: 35.0,
		TotalCarbGrams:    45.0,
		TotalFatGrams:     10.0,
	}

	recipe2 := repository.GeneratedRecipeResult{
		RecipeName:   proteinName + " Áp chảo thảo mộc (chống ngán)",
		CookingSteps: []string{"Ướp gia vị nhẹ nhàng", "Áp chảo lửa vừa trong 8 phút"},
		SupplementaryIngredients: []aggregate.IngredientGram{
			aggregate.NewIngredientGram(proteinName, 160.0, false),
			aggregate.NewIngredientGram(carbName, 180.0, false),
			aggregate.NewIngredientGram(veggieName, 120.0, false),
		},
		TotalProteinGrams: 38.0,
		TotalCarbGrams:    40.0,
		TotalFatGrams:     12.0,
	}

	return []repository.GeneratedRecipeResult{recipe1, recipe2}, nil
}

func (m *mockAIServiceForSim) EstimateNutrient(
	_ context.Context,
	_ string,
	_ string,
) (*repository.EstimatedNutrientResult, error) {
	return &repository.EstimatedNutrientResult{Calories: 450, Protein: 30, Carbs: 50, Fat: 10}, nil
}

func (m *mockAIServiceForSim) GenerateNutritionInsight(
	_ context.Context,
	_ repository.InsightPromptContext,
) (*repository.NutritionInsightResult, error) {
	return &repository.NutritionInsightResult{OverallScore: 80, Summary: "good"}, nil
}

// TestPersonalization_Scenario1_PreferredIngredientsInAvailable kiểm tra khi nguyên liệu ưa chuộng CÓ trong danh mục có sẵn:
// Hệ thống phải ưu tiên chọn nguyên liệu đó và phản ánh đúng WeeklyCookingStyle.
func TestPersonalization_Scenario1_PreferredIngredientsInAvailable(t *testing.T) {
	cacheRepo := &simCacheRepo{recipes: make(map[string]*repository.CachedRecipe)}
	catalog := []vo.FoodNutrient{
		vo.NewFoodNutrient("f1", "Ức gà tươi", "PROTEIN", 165, 31, 0, 3.6, nil, "CHICKEN", "", false, 13000, "BUDGET"),
		vo.NewFoodNutrient("f2", "Thịt heo nạc", "PROTEIN", 143, 21, 0, 6.2, nil, "PORK", "", false, 16000, "STANDARD"),
		vo.NewFoodNutrient("f3", "Cơm trắng gạo tẻ", "CARB", 130, 2.7, 28, 0.3, nil, "", "RICE", false, 3500, "BUDGET"),
		vo.NewFoodNutrient("f4", "Rau muống luộc", "VEGGIE", 19, 3.0, 2.1, 0.2, nil, "", "", false, 3500, "BUDGET"),
	}
	foodRepo := &simFoodRepo{catalog: catalog}
	aiMock := &mockAIServiceForSim{}
	generator := service.NewMenuGenerator(cacheRepo, foodRepo, aiMock)

	alloc, _ := vo.NewCalorieAllocation(2000, 140, 220, 60)
	personalization := service.DailyPlanPersonalization{
		PreferredIngredients: []string{"Ức gà tươi", "Cơm trắng gạo tẻ"},
		WeeklyCookingStyle:   "Hấp luộc ít dầu mỡ",
	}

	plan, err := generator.GenerateDailyPlan(
		context.Background(),
		"user-test-scenario-1",
		time.Now(),
		alloc,
		nil,
		vo.GoalMaintenance,
		personalization,
	)
	if err != nil {
		t.Fatalf("GenerateDailyPlan failed: %v", err)
	}

	if plan == nil {
		t.Fatal("Expected generated plan, got nil")
	}

	// 1. Kiểm tra prompt context truyền sang AI Service mang đầy đủ thông tin cá nhân hóa
	if len(aiMock.capturedPromptCtx.PreferredIngredients) != 2 {
		t.Errorf("Expected 2 preferred ingredients in AI promptCtx, got %d", len(aiMock.capturedPromptCtx.PreferredIngredients))
	}
	if aiMock.capturedPromptCtx.WeeklyCookingStyle != "Hấp luộc ít dầu mỡ" {
		t.Errorf("Expected WeeklyCookingStyle 'Hấp luộc ít dầu mỡ', got %q", aiMock.capturedPromptCtx.WeeklyCookingStyle)
	}

	// 2. Kiểm tra meal options sinh ra chứa nguyên liệu ưu tiên
	foundPreferred := false
	for _, meal := range plan.DailyMeals() {
		for _, opt := range meal.Options() {
			for _, ing := range opt.Ingredients() {
				if ing.IngredientName() == "Ức gà tươi" || ing.IngredientName() == "Cơm trắng gạo tẻ" {
					foundPreferred = true
				}
			}
		}
	}
	if !foundPreferred {
		t.Error("Expected meal options to prioritize preferred ingredients ('Ức gà tươi', 'Cơm trắng gạo tẻ')")
	}
}

// TestPersonalization_Scenario2_PreferredIngredientsNotInAvailable kiểm tra khi nguyên liệu ưa chuộng KHÔNG có trong danh mục:
// Hệ thống phải bỏ qua, tuyệt đối KHÔNG tự ý bịa thêm nguyên liệu không có trong catalog.
func TestPersonalization_Scenario2_PreferredIngredientsNotInAvailable(t *testing.T) {
	cacheRepo := &simCacheRepo{recipes: make(map[string]*repository.CachedRecipe)}
	// Catalog bình dân, KHÔNG chứa Cá tuyết Nauy hay Bò Wagyu
	catalog := []vo.FoodNutrient{
		vo.NewFoodNutrient("f1", "Thịt heo nạc", "PROTEIN", 143, 21, 0, 6.2, nil, "PORK", "", false, 16000, "STANDARD"),
		vo.NewFoodNutrient("f2", "Khoai lang luộc", "CARB", 90, 2.0, 21, 0.1, nil, "", "SWEET_POTATO", false, 4500, "BUDGET"),
		vo.NewFoodNutrient("f3", "Rau muống luộc", "VEGGIE", 19, 3.0, 2.1, 0.2, nil, "", "", false, 3500, "BUDGET"),
	}
	foodRepo := &simFoodRepo{catalog: catalog}
	aiMock := &mockAIServiceForSim{}
	generator := service.NewMenuGenerator(cacheRepo, foodRepo, aiMock)

	alloc, _ := vo.NewCalorieAllocation(1800, 130, 200, 50)
	personalization := service.DailyPlanPersonalization{
		PreferredIngredients: []string{"Cá tuyết Nauy", "Thịt bò Wagyu", "Măng tây tím"},
		WeeklyCookingStyle:   "Áp chảo sốt bơ tỏi",
	}

	plan, err := generator.GenerateDailyPlan(
		context.Background(),
		"user-test-scenario-2",
		time.Now(),
		alloc,
		nil,
		vo.GoalWeightLoss,
		personalization,
	)
	if err != nil {
		t.Fatalf("GenerateDailyPlan failed: %v", err)
	}

	// Tập hợp tên tất cả nguyên liệu trong catalog có sẵn
	validCatalogNames := make(map[string]bool)
	for _, c := range catalog {
		validCatalogNames[c.Name()] = true
	}

	// Kiểm tra mọi meal option sinh ra:
	// - KHÔNG được chứa bất kỳ nguyên liệu nào ngoài danh mục có sẵn
	// - KHÔNG được chứa "Cá tuyết Nauy" hay "Thịt bò Wagyu"
	for _, meal := range plan.DailyMeals() {
		for _, opt := range meal.Options() {
			for _, ing := range opt.Ingredients() {
				ingName := ing.IngredientName()
				if !validCatalogNames[ingName] {
					t.Errorf("VIOLATION: Invented ingredient %q detected! Must strictly adhere to available catalog.", ingName)
				}
				if ingName == "Cá tuyết Nauy" || ingName == "Thịt bò Wagyu" || ingName == "Măng tây tím" {
					t.Errorf("VIOLATION: Unlisted preferred ingredient %q was incorrectly selected!", ingName)
				}
			}
		}
	}
}

// TestPersonalization_Scenario3_ColdStart_NoInsight kiểm tra khi không có đánh giá tuần trước (User mới hoặc dormant):
// Hệ thống không bị crash, để trống thông tin cá nhân hóa và sinh thực đơn bình thường với đa dạng hóa tự nhiên.
func TestPersonalization_Scenario3_ColdStart_NoInsight(t *testing.T) {
	cacheRepo := &simCacheRepo{recipes: make(map[string]*repository.CachedRecipe)}
	catalog := []vo.FoodNutrient{
		vo.NewFoodNutrient("f1", "Thịt heo nạc", "PROTEIN", 143, 21, 0, 6.2, nil, "PORK", "", false, 16000, "STANDARD"),
		vo.NewFoodNutrient("f2", "Khoai lang luộc", "CARB", 90, 2.0, 21, 0.1, nil, "", "SWEET_POTATO", false, 4500, "BUDGET"),
		vo.NewFoodNutrient("f3", "Rau muống luộc", "VEGGIE", 19, 3.0, 2.1, 0.2, nil, "", "", false, 3500, "BUDGET"),
	}
	foodRepo := &simFoodRepo{catalog: catalog}
	aiMock := &mockAIServiceForSim{}
	generator := service.NewMenuGenerator(cacheRepo, foodRepo, aiMock)

	alloc, _ := vo.NewCalorieAllocation(2200, 150, 250, 65)

	// Không truyền personalization (Cold start)
	plan, err := generator.GenerateDailyPlan(
		context.Background(),
		"user-test-scenario-3",
		time.Now(),
		alloc,
		nil,
		vo.GoalMuscleGain,
	)
	if err != nil {
		t.Fatalf("GenerateDailyPlan failed on cold start: %v", err)
	}

	if plan == nil {
		t.Fatal("Expected valid plan on cold start, got nil")
	}

	// Xác nhận prompt context gửi sang AI không có preferred ingredients và cooking style
	if len(aiMock.capturedPromptCtx.PreferredIngredients) != 0 {
		t.Errorf("Expected 0 preferred ingredients on cold start, got %d", len(aiMock.capturedPromptCtx.PreferredIngredients))
	}
	if aiMock.capturedPromptCtx.WeeklyCookingStyle != "" {
		t.Errorf("Expected empty WeeklyCookingStyle on cold start, got %q", aiMock.capturedPromptCtx.WeeklyCookingStyle)
	}

	// Kế hoạch sinh ra vẫn có đủ các bữa ăn tiêu chuẩn
	if len(plan.DailyMeals()) == 0 {
		t.Error("Expected meals in plan on cold start, got 0")
	}
}
