package grpc_test

import (
	"context"
	"testing"
	"time"

	nutritionv1msg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/nutrition/v1/message"
	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/application/query"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/entity"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
	"github.com/viethung213/gym-companion/internal/nutrition/test/testutil"
	"github.com/viethung213/gym-companion/internal/nutrition/transport/grpc"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type mockPlanRepoForTransport struct {
	plans map[string]*aggregate.NutritionPlan
}

func (m *mockPlanRepoForTransport) FindByUserIDAndDate(_ context.Context, userID string, date time.Time) (*aggregate.NutritionPlan, error) {
	key := userID + date.Format("2006-01-02")
	return m.plans[key], nil
}

func (m *mockPlanRepoForTransport) FindByUserIDAndDates(_ context.Context, userID string, dates []time.Time) ([]*aggregate.NutritionPlan, error) {
	var result []*aggregate.NutritionPlan
	for _, d := range dates {
		key := userID + d.Format("2006-01-02")
		if p, ok := m.plans[key]; ok {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockPlanRepoForTransport) Save(_ context.Context, plan *aggregate.NutritionPlan) error {
	key := plan.UserID() + plan.PlanDate().Format("2006-01-02")
	m.plans[key] = plan
	return nil
}

func (m *mockPlanRepoForTransport) Update(_ context.Context, plan *aggregate.NutritionPlan) error {
	return m.Save(context.Background(), plan)
}

type mockUserAllergyRepoForTransport struct {
	allergies map[string][]*entity.UserAllergy
}

func (m *mockUserAllergyRepoForTransport) FindAllergiesByUserID(_ context.Context, userID string) ([]*entity.UserAllergy, error) {
	if m.allergies == nil {
		return nil, nil
	}
	return m.allergies[userID], nil
}

func (m *mockUserAllergyRepoForTransport) BatchFindAllergiesByUserIDs(_ context.Context, userIDs []string) (map[string][]string, error) {
	res := make(map[string][]string, len(userIDs))
	for _, uid := range userIDs {
		if m.allergies != nil {
			names := make([]string, 0, len(m.allergies[uid]))
			for _, a := range m.allergies[uid] {
				names = append(names, a.AllergenName())
			}
			res[uid] = names
		}
	}
	return res, nil
}

func (m *mockUserAllergyRepoForTransport) SetAllergies(_ context.Context, userID string, allergies []*entity.UserAllergy) error {
	if m.allergies == nil {
		m.allergies = make(map[string][]*entity.UserAllergy)
	}
	m.allergies[userID] = allergies
	return nil
}

func (m *mockPlanRepoForTransport) FindActiveUserIDs(_ context.Context, _ int) ([]string, error) {
	return nil, nil
}

func (m *mockPlanRepoForTransport) FindPlansForDate(_ context.Context, _ time.Time) ([]*aggregate.NutritionPlan, error) {
	return nil, nil
}

func (m *mockPlanRepoForTransport) GetUserMealSchedules(_ context.Context, _ string) (map[string]string, error) {
	return nil, nil
}

func (m *mockPlanRepoForTransport) SaveUserMealSchedules(_ context.Context, _ string, _ map[string]string) error {
	return nil
}

type mockHistoryRepoForTransport struct {
	history *aggregate.MealHistory
}

func (m *mockHistoryRepoForTransport) FindByUserID(_ context.Context, _ string) (*aggregate.MealHistory, error) {
	return m.history, nil
}

func (m *mockHistoryRepoForTransport) FindByUserIDAndDateRange(_ context.Context, _ string, _, _ time.Time) (*aggregate.MealHistory, error) {
	return m.history, nil
}

func (m *mockHistoryRepoForTransport) Save(_ context.Context, history *aggregate.MealHistory) error {
	m.history = history
	return nil
}

type mockFoodRepoForTransport struct {
	items map[string]*aggregate.FoodItem
}

func (m *mockFoodRepoForTransport) FindByID(_ context.Context, id string) (*aggregate.FoodItem, error) {
	return m.items[id], nil
}

func (m *mockFoodRepoForTransport) FindByName(_ context.Context, _ string) (*aggregate.FoodItem, error) {
	return nil, nil
}

func (m *mockFoodRepoForTransport) FindActiveCatalog(_ context.Context) ([]vo.FoodNutrient, error) {
	return []vo.FoodNutrient{
		vo.NewFoodNutrient("f1", "Ức gà", "PROTEIN", 165, 31, 0, 3.6, nil, "", "", false, 13000, "BUDGET"),
	}, nil
}

func (m *mockFoodRepoForTransport) FindNutiFoodProducts(_ context.Context) ([]vo.FoodNutrient, error) {
	return nil, nil
}

func (m *mockFoodRepoForTransport) Save(_ context.Context, item *aggregate.FoodItem) error {
	m.items[item.ID()] = item
	return nil
}

func (m *mockFoodRepoForTransport) Update(_ context.Context, item *aggregate.FoodItem) error {
	m.items[item.ID()] = item
	return nil
}

type mockRecipeCacheRepoForTransport struct{}

func (m *mockRecipeCacheRepoForTransport) FindByHash(_ context.Context, _ string) (*repository.CachedRecipe, error) {
	return nil, nil
}

func (m *mockRecipeCacheRepoForTransport) Save(_ context.Context, _ *repository.CachedRecipe) error {
	return nil
}

type mockAIServiceForTransport struct{}

func (m *mockAIServiceForTransport) SelectCreativeMealOptions(
	_ context.Context,
	_ repository.AIMenuPromptContext,
) ([]repository.GeneratedRecipeResult, error) {
	return []repository.GeneratedRecipeResult{
		{
			RecipeName:   "Ức gà áp chảo",
			CookingSteps: []string{"Áp chảo 15 phút"},
			SupplementaryIngredients: []aggregate.IngredientGram{
				aggregate.NewIngredientGram("Ức gà", 150, false),
				aggregate.NewIngredientGram("Khoai lang", 200, false),
				aggregate.NewIngredientGram("Bông cải", 120, false),
			},
		},
	}, nil
}

func (m *mockAIServiceForTransport) EstimateNutrient(_ context.Context, _, _ string) (*repository.EstimatedNutrientResult, error) {
	return &repository.EstimatedNutrientResult{Calories: 450, Protein: 25, Carbs: 50, Fat: 12}, nil
}

func (m *mockAIServiceForTransport) GenerateNutritionInsight(_ context.Context, _ repository.InsightPromptContext) (*repository.NutritionInsightResult, error) {
	return &repository.NutritionInsightResult{Summary: "stub insight"}, nil
}

func TestGRPCHandler_Integration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	planRepo := &mockPlanRepoForTransport{plans: make(map[string]*aggregate.NutritionPlan)}
	historyRepo := &mockHistoryRepoForTransport{}
	foodRepo := &mockFoodRepoForTransport{items: make(map[string]*aggregate.FoodItem)}
	cacheRepo := &mockRecipeCacheRepoForTransport{}
	aiStub := &mockAIServiceForTransport{}

	tdeeCalc := service.NewTDEECalculator()
	menuGen := service.NewMenuGenerator(cacheRepo, foodRepo, aiStub)

	genPlanHdlr := command.NewGenerateDailyPlanHandler(planRepo, historyRepo, nil, tdeeCalc, menuGen)
	recalPlanHdlr := command.NewRecalibratePlanWithPantryHandler(planRepo, historyRepo, nil, menuGen)
	logMealHdlr := command.NewLogMealHandler(planRepo, historyRepo, nil, aiStub)
	createFoodItemHdlr := command.NewCreateFoodItemHandler(foodRepo)
	approveFoodItemHdlr := command.NewApproveFoodItemHandler(foodRepo)
	updateMealScheduleHdlr := command.NewUpdateMealScheduleHandler(planRepo)

	getTodayMenuHdlr := query.NewGetTodayMenuHandler(planRepo)
	getNutritionHistoryHdlr := query.NewGetNutritionHistoryHandler(historyRepo)
	getNutritionSummaryHdlr := query.NewGetNutritionSummaryHandler(planRepo, historyRepo)

	handler := grpc.NewGRPCHandler(
		genPlanHdlr, recalPlanHdlr, logMealHdlr,
		createFoodItemHdlr, approveFoodItemHdlr, updateMealScheduleHdlr,
		getTodayMenuHdlr, getNutritionHistoryHdlr, getNutritionSummaryHdlr,
		query.NewGetNutritionInsightHandler(nil),
	)

	// 1. Validation error test
	_, err := handler.GetTodayMenu(ctx, &nutritionv1msg.GetTodayMenuRequest{UserId: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got status code %v, want %v", status.Code(err), codes.InvalidArgument)
	}

	// 2. GetTodayMenu (Auto-generates daily plan)
	menuResp, err := handler.GetTodayMenu(ctx, &nutritionv1msg.GetTodayMenuRequest{UserId: "grpc-user-1"})
	if err != nil {
		t.Fatalf("unexpected error GetTodayMenu: %v", err)
	}
	if menuResp.TargetCalories <= 0 {
		t.Fatalf("got target calories %f, want > 0", menuResp.TargetCalories)
	}

	// 3. CreateFoodItem
	createResp, err := handler.CreateFoodItem(ctx, &nutritionv1msg.CreateFoodItemRequest{
		Name:              "Thịt lợn",
		Category:          "PROTEIN",
		CaloriesPer_100G:  240,
		ProteinPer_100G:   26,
		IsNutifoodProduct: false,
	})
	if err != nil {
		t.Fatalf("unexpected error CreateFoodItem: %v", err)
	}
	if createResp.FoodItemId == "" {
		t.Fatalf("expected non-empty FoodItemId")
	}

	// 4. ApproveFoodItem
	approveResp, err := handler.ApproveFoodItem(ctx, &nutritionv1msg.ApproveFoodItemRequest{
		FoodItemId: createResp.FoodItemId,
	})
	if err != nil {
		t.Fatalf("unexpected error ApproveFoodItem: %v", err)
	}
	if !approveResp.Success {
		t.Fatalf("expected approve success true")
	}

	// 5. LogMeal
	logResp, err := handler.LogMeal(ctx, &nutritionv1msg.LogMealRequest{
		UserId:   "grpc-user-1",
		MealType: "Lunch",
		MealName: "Ức gà áp chảo",
		Calories: 450,
		Protein:  35,
		Carbs:    20,
		Fat:      10,
	})
	if err != nil {
		t.Fatalf("unexpected error LogMeal: %v", err)
	}
	if !logResp.Success {
		t.Fatalf("expected log meal success true")
	}
}

func TestGRPCHandler_GetNutritionHistory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	planRepo := &mockPlanRepoForTransport{plans: make(map[string]*aggregate.NutritionPlan)}
	historyRepo := &mockHistoryRepoForTransport{}
	foodRepo := &mockFoodRepoForTransport{items: make(map[string]*aggregate.FoodItem)}
	cacheRepo := &mockRecipeCacheRepoForTransport{}
	aiStub := &mockAIServiceForTransport{}

	tdeeCalc := service.NewTDEECalculator()
	menuGen := service.NewMenuGenerator(cacheRepo, foodRepo, aiStub)

	handler := grpc.NewGRPCHandler(
		command.NewGenerateDailyPlanHandler(planRepo, historyRepo, nil, tdeeCalc, menuGen),
		command.NewRecalibratePlanWithPantryHandler(planRepo, historyRepo, nil, menuGen),
		command.NewLogMealHandler(planRepo, historyRepo, nil, aiStub),
		command.NewCreateFoodItemHandler(foodRepo),
		command.NewApproveFoodItemHandler(foodRepo),
		command.NewUpdateMealScheduleHandler(planRepo),
		query.NewGetTodayMenuHandler(planRepo),
		query.NewGetNutritionHistoryHandler(historyRepo),
		query.NewGetNutritionSummaryHandler(planRepo, historyRepo),
		query.NewGetNutritionInsightHandler(nil),
	)

	// Validation: empty user_id
	_, err := handler.GetNutritionHistory(ctx, &nutritionv1msg.GetNutritionHistoryRequest{UserId: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got status code %v, want %v", status.Code(err), codes.InvalidArgument)
	}

	// Happy path: no history yet → returns empty meals list
	resp, err := handler.GetNutritionHistory(ctx, &nutritionv1msg.GetNutritionHistoryRequest{UserId: "hist-user-1"})
	if err != nil {
		t.Fatalf("unexpected error GetNutritionHistory: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	// Happy path: with logged meals
	now := time.Now()
	history := aggregate.NewMealHistory("h-1", "hist-user-1")
	history.AddMealLog(aggregate.NewMealLog("ml-1", "h-1", "hist-user-1", "Lunch", "Cơm gà", "1 dĩa", 500, 30, 60, 10, now))
	historyRepo.history = history

	resp, err = handler.GetNutritionHistory(ctx, &nutritionv1msg.GetNutritionHistoryRequest{UserId: "hist-user-1"})
	if err != nil {
		t.Fatalf("unexpected error GetNutritionHistory with meals: %v", err)
	}
	if len(resp.GetMeals()) != 1 {
		t.Fatalf("got %d meals, want 1", len(resp.GetMeals()))
	}
	if got := resp.GetMeals()[0].GetMealName(); got != "Cơm gà" {
		t.Fatalf("got meal name %q, want %q", got, "Cơm gà")
	}
}

func TestGRPCHandler_GetNutritionSummary(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Now()
	planRepo := &mockPlanRepoForTransport{plans: make(map[string]*aggregate.NutritionPlan)}
	historyRepo := &mockHistoryRepoForTransport{}
	foodRepo := &mockFoodRepoForTransport{items: make(map[string]*aggregate.FoodItem)}
	cacheRepo := &mockRecipeCacheRepoForTransport{}
	aiStub := &mockAIServiceForTransport{}

	tdeeCalc := service.NewTDEECalculator()
	menuGen := service.NewMenuGenerator(cacheRepo, foodRepo, aiStub)

	handler := grpc.NewGRPCHandler(
		command.NewGenerateDailyPlanHandler(planRepo, historyRepo, nil, tdeeCalc, menuGen),
		command.NewRecalibratePlanWithPantryHandler(planRepo, historyRepo, nil, menuGen),
		command.NewLogMealHandler(planRepo, historyRepo, nil, aiStub),
		command.NewCreateFoodItemHandler(foodRepo),
		command.NewApproveFoodItemHandler(foodRepo),
		command.NewUpdateMealScheduleHandler(planRepo),
		query.NewGetTodayMenuHandler(planRepo),
		query.NewGetNutritionHistoryHandler(historyRepo),
		query.NewGetNutritionSummaryHandler(planRepo, historyRepo),
		query.NewGetNutritionInsightHandler(nil),
	)

	// Validation: empty user_id
	_, err := handler.GetNutritionSummary(ctx, &nutritionv1msg.GetNutritionSummaryRequest{UserId: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got status code %v, want %v", status.Code(err), codes.InvalidArgument)
	}

	// Seed a plan so the summary handler can find it
	alloc, _ := vo.NewCalorieAllocation(2000, 150, 200, 66)
	plan := aggregate.NewNutritionPlan("p-summary-1", "summary-user-1", now, alloc, nil)
	_ = planRepo.Save(ctx, plan)

	// Happy path
	resp, err := handler.GetNutritionSummary(ctx, &nutritionv1msg.GetNutritionSummaryRequest{UserId: "summary-user-1"})
	if err != nil {
		t.Fatalf("unexpected error GetNutritionSummary: %v", err)
	}
	if resp.GetTargetCalories() <= 0 {
		t.Fatalf("got target calories %f, want > 0", resp.GetTargetCalories())
	}
}

func TestGetNutritionInsight_Handler(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t, "")
	repos := testutil.NewRepos(db)

	insightHdlr := query.NewGetNutritionInsightHandler(repos.WeeklyInsight)
	handler := grpc.NewGRPCHandler(
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil,
		insightHdlr,
	)

	// 1. Validation error: empty user_id
	_, err := handler.GetNutritionInsight(ctx, &nutritionv1msg.GetNutritionInsightRequest{UserId: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got status code %v, want %v", status.Code(err), codes.InvalidArgument)
	}

	// 2. Not found
	_, err = handler.GetNutritionInsight(ctx, &nutritionv1msg.GetNutritionInsightRequest{UserId: "unknown-user"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("got status code %v, want %v", status.Code(err), codes.NotFound)
	}

	// 3. Seed weekly insight
	now := time.Now()
	startDate := now.AddDate(0, 0, -6)
	seedInsight := testutil.MustNewWeeklyInsight(
		"insight-test-1",
		"insight-user-1",
		startDate,
		now,
		85,
		"Đạt mục tiêu rất tốt",
		"Luộc, hấp thanh đạm",
		[]string{"Ức gà", "Bông cải xanh"},
		[]entity.WeeklyImprovementArea{
			{
				Nutrient:    "PROTEIN",
				Action:      "INCREASE",
				DeltaAmount: 15,
				Suggestion:  "Bổ sung thêm đạm buổi sáng",
			},
		},
		&entity.WeeklyMacroSummary{
			ConsumedCalories: 14000,
			TargetCalories:   14000,
			ConsumedProtein:  800,
			TargetProtein:    850,
			ConsumedCarbs:    1200,
			TargetCarbs:      1200,
			ConsumedFat:      400,
			TargetFat:        400,
			LoggedMealsCount: 28,
			TotalMealsCount:  28,
		},
	)
	if err := repos.WeeklyInsight.Save(ctx, seedInsight); err != nil {
		t.Fatalf("failed to save seed insight: %v", err)
	}

	// 4. Successful retrieval without LLM call
	resp, err := handler.GetNutritionInsight(ctx, &nutritionv1msg.GetNutritionInsightRequest{UserId: "insight-user-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetOverallScore() != 85 {
		t.Errorf("got overall score %d, want 85", resp.GetOverallScore())
	}
	if resp.GetPredictedCookingStyle() != "Luộc, hấp thanh đạm" {
		t.Errorf("got style %s, want Luộc, hấp thanh đạm", resp.GetPredictedCookingStyle())
	}
	if len(resp.GetImprovementAreas()) != 1 {
		t.Fatalf("got %d improvement areas, want 1", len(resp.GetImprovementAreas()))
	}
	if resp.GetImprovementAreas()[0].GetNutrient() != "PROTEIN" {
		t.Errorf("got nutrient %s, want PROTEIN", resp.GetImprovementAreas()[0].GetNutrient())
	}
	if resp.GetWeeklySummary().GetConsumedCalories() != 14000 {
		t.Errorf("got consumed calories %f, want 14000", resp.GetWeeklySummary().GetConsumedCalories())
	}
}

func TestGRPCHandler_RecalibratePlanWithPantry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	planRepo := &mockPlanRepoForTransport{plans: make(map[string]*aggregate.NutritionPlan)}
	historyRepo := &mockHistoryRepoForTransport{}
	foodRepo := &mockFoodRepoForTransport{items: make(map[string]*aggregate.FoodItem)}
	cacheRepo := &mockRecipeCacheRepoForTransport{}
	aiStub := &mockAIServiceForTransport{}

	tdeeCalc := service.NewTDEECalculator()
	menuGen := service.NewMenuGenerator(cacheRepo, foodRepo, aiStub)

	eTom, _ := entity.NewUserAllergy("tom-id", "user-pantry-grpc", "TÔM", time.Now())
	allergyRepo := &mockUserAllergyRepoForTransport{
		allergies: map[string][]*entity.UserAllergy{
			"user-pantry-grpc": {eTom},
		},
	}

	handler := grpc.NewGRPCHandler(
		command.NewGenerateDailyPlanHandler(planRepo, historyRepo, nil, tdeeCalc, menuGen),
		command.NewRecalibratePlanWithPantryHandler(planRepo, historyRepo, nil, menuGen),
		command.NewLogMealHandler(planRepo, historyRepo, nil, aiStub),
		command.NewCreateFoodItemHandler(foodRepo),
		command.NewApproveFoodItemHandler(foodRepo),
		command.NewUpdateMealScheduleHandler(planRepo),
		query.NewGetTodayMenuHandler(planRepo),
		query.NewGetNutritionHistoryHandler(historyRepo),
		query.NewGetNutritionSummaryHandler(planRepo, historyRepo),
		query.NewGetNutritionInsightHandler(nil),
	).WithAllergyHandlers(
		query.NewGetUserAllergiesHandler(allergyRepo),
		command.NewUpdateUserAllergiesHandler(allergyRepo),
	)

	// 1. Validation: empty user_id
	_, err := handler.RecalibratePlanWithPantry(ctx, &nutritionv1msg.RecalibratePlanWithPantryRequest{
		UserId:               "",
		MealType:             "LUNCH",
		AvailableIngredients: []string{"Ức gà"},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got status code %v, want %v", status.Code(err), codes.InvalidArgument)
	}

	// 1b. Validation: empty meal_type
	_, err = handler.RecalibratePlanWithPantry(ctx, &nutritionv1msg.RecalibratePlanWithPantryRequest{
		UserId:               "user-pantry-grpc",
		MealType:             "",
		AvailableIngredients: []string{"Ức gà"},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument on empty meal_type, got code %v, err: %v", status.Code(err), err)
	}

	// 2. Setup existing plan với các bữa ăn
	now := time.Now()
	alloc, _ := vo.NewCalorieAllocation(2000, 150, 200, 66)
	optBf := aggregate.NewMealOption("opt-bf-grpc", "Bánh mì", 400, 20, 50, 10, nil, nil, false)
	mealBf := aggregate.NewDailyMeal("Breakfast", []aggregate.MealOption{optBf})
	optLunch := aggregate.NewMealOption("opt-lunch-grpc", "Cơm sườn", 700, 40, 70, 20, nil, nil, false)
	mealLunch := aggregate.NewDailyMeal("Lunch", []aggregate.MealOption{optLunch})
	existingPlan := aggregate.NewNutritionPlan("plan-pantry-grpc-1", "user-pantry-grpc", now, alloc, []aggregate.DailyMeal{mealBf, mealLunch})
	_ = planRepo.Save(ctx, existingPlan)

	// 3. Allergen conflict: user sends shrimp while having restriction on TÔM in profile
	_, err = handler.RecalibratePlanWithPantry(ctx, &nutritionv1msg.RecalibratePlanWithPantryRequest{
		UserId:               "user-pantry-grpc",
		MealType:             "LUNCH",
		AvailableIngredients: []string{"Tôm sú", "Cơm trắng"},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument on allergen conflict, got code %v, err: %v", status.Code(err), err)
	}

	// 4. Happy path: user changes allergy to SEAFOOD, sends deer meat and brown rice (safe)
	eSeafood, _ := entity.NewUserAllergy("seafood-id", "user-pantry-grpc", "SEAFOOD", time.Now())
	_ = allergyRepo.SetAllergies(ctx, "user-pantry-grpc", []*entity.UserAllergy{eSeafood})
	resp, err := handler.RecalibratePlanWithPantry(ctx, &nutritionv1msg.RecalibratePlanWithPantryRequest{
		UserId:               "user-pantry-grpc",
		MealType:             "LUNCH",
		AvailableIngredients: []string{"Thịt nai", "Cơm gạo lứt"},
	})
	if err != nil {
		t.Fatalf("unexpected error on safe recalibrate: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	// 5. Invariant: Meal already logged -> FailedPrecondition
	updatedPlan, _ := planRepo.FindByUserIDAndDate(ctx, "user-pantry-grpc", now)
	lunchMeal := updatedPlan.DailyMeals()[1]
	_, _ = updatedPlan.MarkOptionLogged(lunchMeal.Options()[0].OptionID())
	_ = planRepo.Update(ctx, updatedPlan)

	_, err = handler.RecalibratePlanWithPantry(ctx, &nutritionv1msg.RecalibratePlanWithPantryRequest{
		UserId:               "user-pantry-grpc",
		MealType:             "LUNCH",
		AvailableIngredients: []string{"Thịt nai"},
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition on already logged meal, got code %v, err: %v", status.Code(err), err)
	}
}

func TestGRPCHandler_UserAllergiesCRUD(t *testing.T) {
	ctx := context.Background()
	allergyRepo := &mockUserAllergyRepoForTransport{
		allergies: make(map[string][]*entity.UserAllergy),
	}

	handler := grpc.NewGRPCHandler(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	).WithAllergyHandlers(
		query.NewGetUserAllergiesHandler(allergyRepo),
		command.NewUpdateUserAllergiesHandler(allergyRepo),
	)

	userID := "test-allergy-user"
	ctx = middleware.WithActor(ctx, userID, "User")

	// 1. Initial Get should be empty
	getResp, err := handler.GetUserAllergies(ctx, &nutritionv1msg.GetUserAllergiesRequest{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(getResp.GetAllergies()) != 0 {
		t.Fatalf("expected 0 allergies, got %d", len(getResp.GetAllergies()))
	}

	// 2. Update allergies (Set) with deduplication & normalization
	updResp, err := handler.UpdateUserAllergies(ctx, &nutritionv1msg.UpdateUserAllergiesRequest{
		Allergies: []string{"dairy", "SEAFOOD", " dairy ", "  peanut  "},
	})
	if err != nil {
		t.Fatalf("expected no error on Update, got %v", err)
	}
	if len(updResp.GetAllergies()) != 3 {
		t.Fatalf("expected 3 deduplicated allergies, got %v", updResp.GetAllergies())
	}

	// 3. Get allergies again to verify updated state
	getResp2, err := handler.GetUserAllergies(ctx, &nutritionv1msg.GetUserAllergiesRequest{})
	if err != nil {
		t.Fatalf("expected no error on Get, got %v", err)
	}
	if len(getResp2.GetAllergies()) != 3 {
		t.Fatalf("expected 3 allergies, got %d", len(getResp2.GetAllergies()))
	}

	// 4. Update with empty list (clearing all allergies)
	updResp2, err := handler.UpdateUserAllergies(ctx, &nutritionv1msg.UpdateUserAllergiesRequest{
		Allergies: []string{},
	})
	if err != nil {
		t.Fatalf("expected no error on clear, got %v", err)
	}
	if len(updResp2.GetAllergies()) != 0 {
		t.Fatalf("expected 0 allergies after clear, got %v", updResp2.GetAllergies())
	}

	// 5. Unauthenticated request should return codes.Unauthenticated
	unauthCtx := context.Background()
	_, err = handler.GetUserAllergies(unauthCtx, &nutritionv1msg.GetUserAllergiesRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}

	// 6. Test extracting user_id directly from incoming metadata header (attached by API gateway)
	mdCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", "gateway-user-123"))
	getRespMD, err := handler.GetUserAllergies(mdCtx, &nutritionv1msg.GetUserAllergiesRequest{})
	if err != nil {
		t.Fatalf("expected no error when reading from gateway metadata header, got %v", err)
	}
	if len(getRespMD.GetAllergies()) != 0 {
		t.Fatalf("expected 0 allergies for new gateway user, got %d", len(getRespMD.GetAllergies()))
	}
}
