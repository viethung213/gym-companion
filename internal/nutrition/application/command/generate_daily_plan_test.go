package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

type mockPlanRepo struct {
	plans map[string]*aggregate.NutritionPlan
}

func (m *mockPlanRepo) FindByUserIDAndDate(ctx context.Context, userID string, date time.Time) (*aggregate.NutritionPlan, error) {
	key := userID + date.Format("2006-01-02")
	return m.plans[key], nil
}

func (m *mockPlanRepo) Save(ctx context.Context, plan *aggregate.NutritionPlan) error {
	key := plan.UserID() + plan.PlanDate().Format("2006-01-02")
	m.plans[key] = plan
	return nil
}

func (m *mockPlanRepo) Update(ctx context.Context, plan *aggregate.NutritionPlan) error {
	return m.Save(ctx, plan)
}

func (m *mockPlanRepo) FindActiveUserIDs(_ context.Context, _ int) ([]string, error) {
	return nil, nil
}

func (m *mockPlanRepo) FindPlansForDate(_ context.Context, _ time.Time) ([]*aggregate.NutritionPlan, error) {
	return nil, nil
}

func (m *mockPlanRepo) GetUserMealSchedules(_ context.Context, _ string) (map[string]string, error) {
	return nil, nil
}

func (m *mockPlanRepo) SaveUserMealSchedules(_ context.Context, _ string, _ map[string]string) error {
	return nil
}

type mockHistoryRepo struct{}

func (m *mockHistoryRepo) FindByUserID(ctx context.Context, userID string) (*aggregate.MealHistory, error) {
	return aggregate.NewMealHistory("hist-1", userID, vo.NewLockoutRegistry(nil)), nil
}

func (m *mockHistoryRepo) Save(ctx context.Context, history *aggregate.MealHistory) error {
	return nil
}

type mockFoodRepo struct{}

func (m *mockFoodRepo) FindByID(ctx context.Context, id string) (*aggregate.FoodItem, error) {
	return nil, nil
}

func (m *mockFoodRepo) FindByName(ctx context.Context, name string) (*aggregate.FoodItem, error) {
	return nil, nil
}

func (m *mockFoodRepo) FindActiveCatalog(ctx context.Context) ([]vo.FoodNutrient, error) {
	return []vo.FoodNutrient{
		vo.NewFoodNutrient("1", "Ức gà tươi", "PROTEIN", 165.0, 31.0, 0.0, 3.6, nil, "CHICKEN", "", false),
		vo.NewFoodNutrient("2", "Khoai lang luộc", "CARB", 90.0, 2.0, 21.0, 0.1, nil, "", "SWEET_POTATO", false),
		vo.NewFoodNutrient("3", "Bông cải xanh", "VEGGIE", 34.0, 2.8, 7.0, 0.4, nil, "", "", false),
	}, nil
}

func (m *mockFoodRepo) FindNutiFoodProducts(ctx context.Context) ([]vo.FoodNutrient, error) {
	return []vo.FoodNutrient{
		vo.NewFoodNutrient("4", "Sữa NutiFood Varna Complete", "NUTIFOOD", 80.0, 4.0, 10.0, 2.5, nil, "DAIRY", "", true),
	}, nil
}

func (m *mockFoodRepo) Save(ctx context.Context, item *aggregate.FoodItem) error   { return nil }
func (m *mockFoodRepo) Update(ctx context.Context, item *aggregate.FoodItem) error { return nil }

type mockRecipeCacheRepo struct{}

func (m *mockRecipeCacheRepo) FindByHash(ctx context.Context, hash string) (*repository.CachedRecipe, error) {
	return nil, nil
}

func (m *mockRecipeCacheRepo) Save(ctx context.Context, recipe *repository.CachedRecipe) error {
	return nil
}

// mockAIService trả về 1 option mẩu để test MenuGenerator đi qua được.
type mockAIService struct{}

func (m *mockAIService) SelectCreativeMealOptions(
	_ context.Context,
	_ repository.AIMenuPromptContext,
	_ vo.LockoutRegistry,
) ([]repository.GeneratedRecipeResult, error) {
	return []repository.GeneratedRecipeResult{
		{
			RecipeName:   "Món Test AI",
			CookingSteps: []string{"Đưa vào nồi.", "Nấu chín."},
			SupplementaryIngredients: []aggregate.IngredientGram{
				aggregate.NewIngredientGram("Ủc gà", 150.0, false),
				aggregate.NewIngredientGram("Khoai lang", 200.0, false),
				aggregate.NewIngredientGram("Bông cải", 120.0, false),
			},
			NewFoodCatalogItems: nil,
		},
	}, nil
}

func (m *mockAIService) EstimateNutrient(
	_ context.Context,
	_, _ string,
) (*repository.EstimatedNutrientResult, error) {
	return &repository.EstimatedNutrientResult{Calories: 450, Protein: 25, Carbs: 50, Fat: 12}, nil
}

func (m *mockAIService) GenerateNutritionInsight(
	_ context.Context,
	_ repository.InsightPromptContext,
) (*repository.NutritionInsightResult, error) {
	return &repository.NutritionInsightResult{
		OverallScore: 80,
		Summary:      "Mock insight",
		WeeklyTrend:  "STABLE",
	}, nil
}

func TestGenerateDailyPlanHandler_Handle(t *testing.T) {
	t.Parallel()

	planRepo := &mockPlanRepo{plans: make(map[string]*aggregate.NutritionPlan)}
	historyRepo := &mockHistoryRepo{}
	foodRepo := &mockFoodRepo{}
	recipeCacheRepo := &mockRecipeCacheRepo{}

	tdeeCalc := service.NewTDEECalculator()
	matrix := service.NewCombinatorialMatrix()
	menuGen := service.NewMenuGenerator(matrix, recipeCacheRepo, foodRepo, &mockAIService{})

	handler := command.NewGenerateDailyPlanHandler(planRepo, historyRepo, nil, tdeeCalc, menuGen)

	cmdObj := command.GenerateDailyPlanCommand{
		UserID:   "user-app-test",
		PlanDate: time.Now(),
		BiologicalMetrics: service.BiologicalMetrics{
			WeightKg:      75.0,
			HeightCm:      175.0,
			Age:           28,
			Gender:        "MALE",
			ActivityLevel: "MODERATELY_ACTIVE",
		},
		NutritionGoal:    vo.GoalMuscleGain,
		UserRestrictions: nil,
	}

	plan, err := handler.Handle(context.Background(), cmdObj)
	if err != nil {
		t.Fatalf("unexpected error handling generate daily plan: %v", err)
	}

	if plan == nil {
		t.Fatalf("got nil plan, want valid NutritionPlan")
	}

	if len(plan.DailyMeals()) != 4 {
		t.Fatalf("got %d daily meals, want 4", len(plan.DailyMeals()))
	}
}
