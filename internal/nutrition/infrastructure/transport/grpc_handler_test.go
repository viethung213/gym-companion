package transport_test

import (
	"context"
	"testing"
	"time"

	nutritionv1msg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/nutrition/v1/message"
	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/application/query"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/transport"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockPlanRepoForTransport struct {
	plans map[string]*aggregate.NutritionPlan
}

func (m *mockPlanRepoForTransport) FindByUserIDAndDate(_ context.Context, userID string, date time.Time) (*aggregate.NutritionPlan, error) {
	key := userID + date.Format("2006-01-02")
	return m.plans[key], nil
}

func (m *mockPlanRepoForTransport) Save(_ context.Context, plan *aggregate.NutritionPlan) error {
	key := plan.UserID() + plan.PlanDate().Format("2006-01-02")
	m.plans[key] = plan
	return nil
}

func (m *mockPlanRepoForTransport) Update(_ context.Context, plan *aggregate.NutritionPlan) error {
	return m.Save(context.Background(), plan)
}

func (m *mockPlanRepoForTransport) FindActiveUserIDs(_ context.Context, _ int) ([]string, error) {
	return nil, nil
}

type mockHistoryRepoForTransport struct {
	history *aggregate.MealHistory
}

func (m *mockHistoryRepoForTransport) FindByUserID(_ context.Context, _ string) (*aggregate.MealHistory, error) {
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
		vo.NewFoodNutrient("f1", "Ức gà", "PROTEIN", 165, 31, 0, 3.6, nil, "", "", false),
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
	_ vo.LockoutRegistry,
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
	matrix := service.NewCombinatorialMatrix()
	menuGen := service.NewMenuGenerator(matrix, cacheRepo, foodRepo, aiStub)

	genPlanHdlr := command.NewGenerateDailyPlanHandler(planRepo, historyRepo, nil, tdeeCalc, menuGen)
	recalPlanHdlr := command.NewRecalibratePlanWithPantryHandler(planRepo, historyRepo, nil, menuGen)
	logMealHdlr := command.NewLogMealHandler(planRepo, historyRepo, nil, aiStub)
	createFoodItemHdlr := command.NewCreateFoodItemHandler(foodRepo)
	approveFoodItemHdlr := command.NewApproveFoodItemHandler(foodRepo)

	getTodayMenuHdlr := query.NewGetTodayMenuHandler(planRepo)
	getNutritionHistoryHdlr := query.NewGetNutritionHistoryHandler(historyRepo)
	getNutritionSummaryHdlr := query.NewGetNutritionSummaryHandler(planRepo, historyRepo)

	getNutritionInsightHdlr := query.NewGetNutritionInsightHandler(planRepo, historyRepo, aiStub)

	handler := transport.NewGRPCHandler(
		genPlanHdlr, recalPlanHdlr, logMealHdlr,
		createFoodItemHdlr, approveFoodItemHdlr,
		getTodayMenuHdlr, getNutritionHistoryHdlr, getNutritionSummaryHdlr,
		getNutritionInsightHdlr,
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
	matrix := service.NewCombinatorialMatrix()
	menuGen := service.NewMenuGenerator(matrix, cacheRepo, foodRepo, aiStub)

	handler := transport.NewGRPCHandler(
		command.NewGenerateDailyPlanHandler(planRepo, historyRepo, nil, tdeeCalc, menuGen),
		command.NewRecalibratePlanWithPantryHandler(planRepo, historyRepo, nil, menuGen),
		command.NewLogMealHandler(planRepo, historyRepo, nil, aiStub),
		command.NewCreateFoodItemHandler(foodRepo),
		command.NewApproveFoodItemHandler(foodRepo),
		query.NewGetTodayMenuHandler(planRepo),
		query.NewGetNutritionHistoryHandler(historyRepo),
		query.NewGetNutritionSummaryHandler(planRepo, historyRepo),
		query.NewGetNutritionInsightHandler(planRepo, historyRepo, aiStub),
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
	lockout := vo.NewLockoutRegistry(nil)
	history := aggregate.NewMealHistory("h-1", "hist-user-1", lockout)
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
	matrix := service.NewCombinatorialMatrix()
	menuGen := service.NewMenuGenerator(matrix, cacheRepo, foodRepo, aiStub)

	handler := transport.NewGRPCHandler(
		command.NewGenerateDailyPlanHandler(planRepo, historyRepo, nil, tdeeCalc, menuGen),
		command.NewRecalibratePlanWithPantryHandler(planRepo, historyRepo, nil, menuGen),
		command.NewLogMealHandler(planRepo, historyRepo, nil, aiStub),
		command.NewCreateFoodItemHandler(foodRepo),
		command.NewApproveFoodItemHandler(foodRepo),
		query.NewGetTodayMenuHandler(planRepo),
		query.NewGetNutritionHistoryHandler(historyRepo),
		query.NewGetNutritionSummaryHandler(planRepo, historyRepo),
		query.NewGetNutritionInsightHandler(planRepo, historyRepo, aiStub),
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

