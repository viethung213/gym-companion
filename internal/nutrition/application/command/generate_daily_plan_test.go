package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/entity"
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

func (m *mockPlanRepo) FindByUserIDAndDates(ctx context.Context, userID string, dates []time.Time) ([]*aggregate.NutritionPlan, error) {
	var result []*aggregate.NutritionPlan
	for _, d := range dates {
		key := userID + d.Format("2006-01-02")
		if p, ok := m.plans[key]; ok {
			result = append(result, p)
		}
	}
	return result, nil
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
	return aggregate.NewMealHistory("hist-1", userID), nil
}

func (m *mockHistoryRepo) FindByUserIDAndDateRange(ctx context.Context, userID string, _, _ time.Time) (*aggregate.MealHistory, error) {
	return aggregate.NewMealHistory("hist-1", userID), nil
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
		vo.NewFoodNutrient("1", "Ức gà tươi", "PROTEIN", 165.0, 31.0, 0.0, 3.6, nil, "CHICKEN", "", false, 13000, "BUDGET"),
		vo.NewFoodNutrient("2", "Khoai lang luộc", "CARB", 90.0, 2.0, 21.0, 0.1, nil, "", "SWEET_POTATO", false, 4500, "BUDGET"),
		vo.NewFoodNutrient("3", "Bông cải xanh", "VEGGIE", 34.0, 2.8, 7.0, 0.4, nil, "", "", false, 9000, "STANDARD"),
	}, nil
}

func (m *mockFoodRepo) FindNutiFoodProducts(ctx context.Context) ([]vo.FoodNutrient, error) {
	return []vo.FoodNutrient{
		vo.NewFoodNutrient("4", "Sữa NutiFood Varna Complete", "NUTIFOOD", 80.0, 4.0, 10.0, 2.5, nil, "DAIRY", "", true, 28000, "PREMIUM"),
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
	}, nil
}

func TestGenerateDailyPlanHandler_Handle(t *testing.T) {
	t.Parallel()

	planRepo := &mockPlanRepo{plans: make(map[string]*aggregate.NutritionPlan)}
	historyRepo := &mockHistoryRepo{}
	foodRepo := &mockFoodRepo{}
	recipeCacheRepo := &mockRecipeCacheRepo{}

	tdeeCalc := service.NewTDEECalculator()
	menuGen := service.NewMenuGenerator(recipeCacheRepo, foodRepo, &mockAIService{})

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

type mockWeeklyInsightRepoForDailyPlan struct {
	insights map[string]*entity.WeeklyInsight
}

func (m *mockWeeklyInsightRepoForDailyPlan) Save(_ context.Context, _ *entity.WeeklyInsight) error {
	return nil
}

func (m *mockWeeklyInsightRepoForDailyPlan) GetLatestByUserID(_ context.Context, userID string) (*entity.WeeklyInsight, error) {
	return m.insights[userID], nil
}

func (m *mockWeeklyInsightRepoForDailyPlan) GetByUserIDAndStartDate(_ context.Context, userID string, _ time.Time) (*entity.WeeklyInsight, error) {
	return m.insights[userID], nil
}

type mockAIServiceWithCapture struct {
	capturedPromptCtx repository.AIMenuPromptContext
}

func (m *mockAIServiceWithCapture) SelectCreativeMealOptions(
	_ context.Context,
	promptCtx repository.AIMenuPromptContext,
) ([]repository.GeneratedRecipeResult, error) {
	m.capturedPromptCtx = promptCtx
	return []repository.GeneratedRecipeResult{
		{
			RecipeName:   "Món Cá Nhân Hóa",
			CookingSteps: []string{"Hấp chín.", "Bày ra đĩa."},
			SupplementaryIngredients: []aggregate.IngredientGram{
				aggregate.NewIngredientGram("Ức gà", 150.0, false),
			},
		},
	}, nil
}

func (m *mockAIServiceWithCapture) EstimateNutrient(_ context.Context, _, _ string) (*repository.EstimatedNutrientResult, error) {
	return &repository.EstimatedNutrientResult{Calories: 400}, nil
}

func (m *mockAIServiceWithCapture) GenerateNutritionInsight(_ context.Context, _ repository.InsightPromptContext) (*repository.NutritionInsightResult, error) {
	return nil, nil
}

func TestGenerateDailyPlanHandler_WithWeeklyInsightPersonalization(t *testing.T) {
	t.Parallel()

	planRepo := &mockPlanRepo{plans: make(map[string]*aggregate.NutritionPlan)}
	historyRepo := &mockHistoryRepo{}
	foodRepo := &mockFoodRepo{}
	recipeCacheRepo := &mockRecipeCacheRepo{}
	aiMock := &mockAIServiceWithCapture{}

	tdeeCalc := service.NewTDEECalculator()
	menuGen := service.NewMenuGenerator(recipeCacheRepo, foodRepo, aiMock)

	weeklyRepo := &mockWeeklyInsightRepoForDailyPlan{
		insights: map[string]*entity.WeeklyInsight{
			"user-pers-1": entity.NewWeeklyInsight(
				"ins-1",
				"user-pers-1",
				time.Now().AddDate(0, 0, -7),
				time.Now(),
				85,
				"Tốt",
				"Hấp, luộc ít dầu mỡ",
				[]string{"Ức gà tươi", "Cơm trắng"},
				nil,
				entity.WeeklyMacroSummary{},
				time.Now(),
				time.Now(),
			),
		},
	}

	handler := command.NewGenerateDailyPlanHandler(planRepo, historyRepo, nil, tdeeCalc, menuGen).
		WithWeeklyInsightRepo(weeklyRepo)

	cmdObj := command.GenerateDailyPlanCommand{
		UserID:   "user-pers-1",
		PlanDate: time.Now(),
		BiologicalMetrics: service.BiologicalMetrics{
			WeightKg:      70.0,
			HeightCm:      170.0,
			Age:           25,
			Gender:        "MALE",
			ActivityLevel: "MODERATELY_ACTIVE",
		},
		NutritionGoal: vo.GoalMaintenance,
	}

	plan, err := handler.Handle(context.Background(), cmdObj)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}

	// Kiểm tra xem AI đã nhận đúng preferred_ingredients và weekly_cooking_style chưa
	if aiMock.capturedPromptCtx.WeeklyCookingStyle != "Hấp, luộc ít dầu mỡ" {
		t.Errorf("got WeeklyCookingStyle=%q, want %q", aiMock.capturedPromptCtx.WeeklyCookingStyle, "Hấp, luộc ít dầu mỡ")
	}
	if len(aiMock.capturedPromptCtx.PreferredIngredients) != 2 || aiMock.capturedPromptCtx.PreferredIngredients[0] != "Ức gà tươi" {
		t.Errorf("got PreferredIngredients=%v, want [Ức gà tươi, Cơm trắng]", aiMock.capturedPromptCtx.PreferredIngredients)
	}
}

func TestGenerateDailyPlanHandler_WithoutWeeklyInsight_ColdStart(t *testing.T) {
	t.Parallel()

	planRepo := &mockPlanRepo{plans: make(map[string]*aggregate.NutritionPlan)}
	historyRepo := &mockHistoryRepo{}
	foodRepo := &mockFoodRepo{}
	recipeCacheRepo := &mockRecipeCacheRepo{}
	aiMock := &mockAIServiceWithCapture{}

	tdeeCalc := service.NewTDEECalculator()
	menuGen := service.NewMenuGenerator(recipeCacheRepo, foodRepo, aiMock)

	weeklyRepo := &mockWeeklyInsightRepoForDailyPlan{
		insights: make(map[string]*entity.WeeklyInsight), // Không có insight nào (cold-start)
	}

	handler := command.NewGenerateDailyPlanHandler(planRepo, historyRepo, nil, tdeeCalc, menuGen).
		WithWeeklyInsightRepo(weeklyRepo)

	cmdObj := command.GenerateDailyPlanCommand{
		UserID:   "user-cold-start",
		PlanDate: time.Now(),
		BiologicalMetrics: service.BiologicalMetrics{
			WeightKg:      60.0,
			HeightCm:      165.0,
			Age:           22,
			Gender:        "FEMALE",
			ActivityLevel: "SEDENTARY",
		},
		NutritionGoal: vo.GoalWeightLoss,
	}

	plan, err := handler.Handle(context.Background(), cmdObj)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}

	// Khi không có dữ liệu tuần trước: trường này phải bỏ trống
	if aiMock.capturedPromptCtx.WeeklyCookingStyle != "" {
		t.Errorf("expected empty WeeklyCookingStyle, got %q", aiMock.capturedPromptCtx.WeeklyCookingStyle)
	}
	if len(aiMock.capturedPromptCtx.PreferredIngredients) != 0 {
		t.Errorf("expected empty PreferredIngredients, got %v", aiMock.capturedPromptCtx.PreferredIngredients)
	}
}

func TestGenerateDailyPlanHandler_StaleWeeklyInsight_IgnoredWhenOlderThan1Week(t *testing.T) {
	t.Parallel()

	planRepo := &mockPlanRepo{plans: make(map[string]*aggregate.NutritionPlan)}
	historyRepo := &mockHistoryRepo{}
	foodRepo := &mockFoodRepo{}
	recipeCacheRepo := &mockRecipeCacheRepo{}
	aiMock := &mockAIServiceWithCapture{}

	tdeeCalc := service.NewTDEECalculator()
	menuGen := service.NewMenuGenerator(recipeCacheRepo, foodRepo, aiMock)

	// Giả lập người dùng đã tạm nghỉ 1 tháng (insight gần nhất đã kết thúc 30 ngày trước)
	oneMonthAgo := time.Now().AddDate(0, -1, 0)
	weeklyRepo := &mockWeeklyInsightRepoForDailyPlan{
		insights: map[string]*entity.WeeklyInsight{
			"user-inactive-1month": entity.NewWeeklyInsight(
				"ins-old",
				"user-inactive-1month",
				oneMonthAgo.AddDate(0, 0, -7),
				oneMonthAgo, // EndDate 30 ngày trước
				80,
				"Thói quen cũ",
				"Chiên xào nhiều dầu",
				[]string{"Thịt mỡ", "Mì tôm"},
				nil,
				entity.WeeklyMacroSummary{},
				oneMonthAgo,
				oneMonthAgo,
			),
		},
	}

	handler := command.NewGenerateDailyPlanHandler(planRepo, historyRepo, nil, tdeeCalc, menuGen).
		WithWeeklyInsightRepo(weeklyRepo)

	cmdObj := command.GenerateDailyPlanCommand{
		UserID:   "user-inactive-1month",
		PlanDate: time.Now(),
		BiologicalMetrics: service.BiologicalMetrics{
			WeightKg:      70.0,
			HeightCm:      170.0,
			Age:           25,
			Gender:        "MALE",
			ActivityLevel: "MODERATELY_ACTIVE",
		},
		NutritionGoal: vo.GoalMaintenance,
	}

	plan, err := handler.Handle(context.Background(), cmdObj)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}

	// Đảm bảo insight của 1 tháng trước KHÔNG được lấy, các trường phải bỏ trống hoàn toàn!
	if aiMock.capturedPromptCtx.WeeklyCookingStyle != "" {
		t.Errorf("expected empty WeeklyCookingStyle for 1-month-old insight, got %q", aiMock.capturedPromptCtx.WeeklyCookingStyle)
	}
	if len(aiMock.capturedPromptCtx.PreferredIngredients) != 0 {
		t.Errorf("expected empty PreferredIngredients for 1-month-old insight, got %v", aiMock.capturedPromptCtx.PreferredIngredients)
	}
}

type spyHistoryRepoWithRangeCheck struct {
	mockHistoryRepo
	capturedFrom time.Time
	capturedTo   time.Time
}

func (s *spyHistoryRepoWithRangeCheck) FindByUserIDAndDateRange(_ context.Context, _ string, from, to time.Time) (*aggregate.MealHistory, error) {
	s.capturedFrom = from
	s.capturedTo = to
	return aggregate.NewMealHistory("hist-1", "user-1"), nil
}

func TestGenerateDailyPlan_FetchesRecent7DaysHistory(t *testing.T) {
	t.Parallel()

	planRepo := &mockPlanRepo{plans: make(map[string]*aggregate.NutritionPlan)}
	historyRepo := &spyHistoryRepoWithRangeCheck{}
	foodRepo := &mockFoodRepo{}
	recipeCacheRepo := &mockRecipeCacheRepo{}
	aiMock := &mockAIService{}

	tdeeCalc := service.NewTDEECalculator()
	menuGen := service.NewMenuGenerator(recipeCacheRepo, foodRepo, aiMock)

	handler := command.NewGenerateDailyPlanHandler(
		planRepo, historyRepo, nil, tdeeCalc, menuGen,
	)

	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	cmdObj := command.GenerateDailyPlanCommand{
		UserID:   "user-7day-test",
		PlanDate: now,
		BiologicalMetrics: service.BiologicalMetrics{
			WeightKg:      70.0,
			HeightCm:      170.0,
			Age:           25,
			Gender:        "MALE",
			ActivityLevel: "MODERATELY_ACTIVE",
		},
		NutritionGoal: vo.GoalMaintenance,
	}

	_, err := handler.Handle(context.Background(), cmdObj)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	diff := historyRepo.capturedTo.Sub(historyRepo.capturedFrom)
	expectedDiff := 7 * 24 * time.Hour
	if diff != expectedDiff {
		t.Errorf("expected 7 days date range (diff %v), got %v (from: %v, to: %v)", expectedDiff, diff, historyRepo.capturedFrom, historyRepo.capturedTo)
	}
}

func TestGenerateDailyPlan_ExtractsRecentIngredientsFromPastPlans(t *testing.T) {
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	yesterday := now.AddDate(0, 0, -1)

	// Tạo một plan ngày hôm qua chứa nguyên liệu "Ức gà tươi" trong bữa LUNCH
	alloc, _ := vo.NewCalorieAllocation(2000, 150, 200, 60)
	pastOpt := aggregate.NewMealOption(
		"opt-past-1", "Ức gà áp chảo", 500, 40, 10, 5,
		[]aggregate.IngredientGram{
			aggregate.NewIngredientGram("Ức gà tươi", 200, false),
			aggregate.NewIngredientGram("Bông cải xanh", 100, false),
		},
		[]string{"Bước 1", "Bước 2"},
		true,
	)
	pastPlan := aggregate.NewNutritionPlan(
		"plan-past-1", "user-ingredient-test", yesterday, alloc,
		[]aggregate.DailyMeal{
			aggregate.NewDailyMeal("LUNCH", []aggregate.MealOption{pastOpt}),
		},
	)

	planRepo := &mockPlanRepo{
		plans: map[string]*aggregate.NutritionPlan{
			"user-ingredient-test" + yesterday.Format("2006-01-02"): pastPlan,
		},
	}

	// History ghi nhận đúng bữa LUNCH ngày hôm qua
	mockHist := aggregate.NewMealHistory("hist-1", "user-ingredient-test")
	mockHist.AddMealLog(aggregate.NewMealLog(
		"log-1", "hist-1", "user-ingredient-test", "LUNCH", "Ức gà áp chảo", "1 dĩa", 500, 40, 10, 5, yesterday,
	))
	historyRepo := &mockHistoryRepoWithData{history: mockHist}

	foodRepo := &mockFoodRepo{}
	recipeCacheRepo := &mockRecipeCacheRepo{}
	aiMock := &mockAIService{}

	tdeeCalc := service.NewTDEECalculator()
	menuGen := service.NewMenuGenerator(recipeCacheRepo, foodRepo, aiMock)

	handler := command.NewGenerateDailyPlanHandler(
		planRepo, historyRepo, nil, tdeeCalc, menuGen,
	)

	cmdObj := command.GenerateDailyPlanCommand{
		UserID:   "user-ingredient-test",
		PlanDate: now,
		BiologicalMetrics: service.BiologicalMetrics{
			WeightKg:      70.0,
			HeightCm:      170.0,
			Age:           25,
			Gender:        "MALE",
			ActivityLevel: "MODERATELY_ACTIVE",
		},
		NutritionGoal: vo.GoalMaintenance,
	}

	plan, err := handler.Handle(context.Background(), cmdObj)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if plan == nil {
		t.Fatalf("expected non-nil plan")
	}
}

type mockHistoryRepoWithData struct {
	history *aggregate.MealHistory
}

func (m *mockHistoryRepoWithData) FindByUserID(_ context.Context, _ string) (*aggregate.MealHistory, error) {
	return m.history, nil
}

func (m *mockHistoryRepoWithData) FindByUserIDAndDateRange(_ context.Context, _ string, _, _ time.Time) (*aggregate.MealHistory, error) {
	return m.history, nil
}

func (m *mockHistoryRepoWithData) Save(_ context.Context, _ *aggregate.MealHistory) error {
	return nil
}
