package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

type mockCacheRepo struct {
	recipes map[string]*repository.CachedRecipe
}

func (m *mockCacheRepo) FindByHash(_ context.Context, hash string) (*repository.CachedRecipe, error) {
	return m.recipes[hash], nil
}

func (m *mockCacheRepo) Save(_ context.Context, recipe *repository.CachedRecipe) error {
	m.recipes[recipe.IngredientHash] = recipe
	return nil
}

type mockFoodItemRepo struct{}

func (m *mockFoodItemRepo) FindByID(_ context.Context, id string) (*aggregate.FoodItem, error) {
	return nil, nil
}
func (m *mockFoodItemRepo) FindByName(_ context.Context, name string) (*aggregate.FoodItem, error) {
	return nil, nil
}
func (m *mockFoodItemRepo) FindActiveCatalog(_ context.Context) ([]vo.FoodNutrient, error) {
	return []vo.FoodNutrient{
		vo.NewFoodNutrient("1", "Ức gà", "PROTEIN", 165, 31, 0, 3.6, nil, "", "", false),
		vo.NewFoodNutrient("2", "Khoai lang", "CARB", 90, 2, 21, 0.1, nil, "", "", false),
		vo.NewFoodNutrient("3", "Bông cải", "VEGGIE", 34, 2.8, 7, 0.4, nil, "", "", false),
	}, nil
}
func (m *mockFoodItemRepo) FindNutiFoodProducts(_ context.Context) ([]vo.FoodNutrient, error) {
	return nil, nil
}
func (m *mockFoodItemRepo) Save(_ context.Context, item *aggregate.FoodItem) error   { return nil }
func (m *mockFoodItemRepo) Update(_ context.Context, item *aggregate.FoodItem) error { return nil }

type mockAIServiceStub struct{}

func (m *mockAIServiceStub) SelectCreativeMealOptions(
	_ context.Context,
	promptCtx repository.AIMenuPromptContext,
	_ vo.LockoutRegistry,
) ([]repository.GeneratedRecipeResult, error) {
	return []repository.GeneratedRecipeResult{
		{
			RecipeName:   "Ức gà áp chảo với khoai lang & bông cải",
			CookingSteps: []string{"Sơ chế", "Áp chảo 15 phút", "Dùng nóng"},
			SupplementaryIngredients: []aggregate.IngredientGram{
				aggregate.NewIngredientGram("Ức gà", 150, false),
				aggregate.NewIngredientGram("Khoai lang", 200, false),
				aggregate.NewIngredientGram("Bông cải", 120, false),
			},
		},
	}, nil
}

func (m *mockAIServiceStub) EstimateNutrient(_ context.Context, _, _ string) (*repository.EstimatedNutrientResult, error) {
	return &repository.EstimatedNutrientResult{Calories: 450, Protein: 25, Carbs: 50, Fat: 12}, nil
}

func (m *mockAIServiceStub) GenerateNutritionInsight(_ context.Context, _ repository.InsightPromptContext) (*repository.NutritionInsightResult, error) {
	return &repository.NutritionInsightResult{OverallScore: 75, Summary: "stub insight", WeeklyTrend: "STABLE"}, nil
}

func TestMenuGenerator_GenerateDailyPlan(t *testing.T) {
	t.Parallel()

	matrix := service.NewCombinatorialMatrix()
	cacheRepo := &mockCacheRepo{recipes: make(map[string]*repository.CachedRecipe)}
	foodRepo := &mockFoodItemRepo{}
	aiStub := &mockAIServiceStub{}

	generator := service.NewMenuGenerator(matrix, cacheRepo, foodRepo, aiStub)

	alloc, _ := vo.NewCalorieAllocation(2000, 150, 200, 66)
	registry := vo.NewLockoutRegistry(nil)

	plan, err := generator.GenerateDailyPlan(
		context.Background(), "user-test-1", time.Now(), alloc, registry, nil,
	)

	if err != nil {
		t.Fatalf("unexpected error generating plan: %v", err)
	}

	if plan == nil {
		t.Fatalf("expected non-nil plan")
	}

	if got := len(plan.DailyMeals()); got != 4 {
		t.Fatalf("got daily meals len %d, want 4", got)
	}

	// Verify recipe cache was saved
	if len(cacheRepo.recipes) == 0 {
		t.Fatalf("expected recipe cache to store generated recipes")
	}
}
