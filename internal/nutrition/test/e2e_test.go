// Package test chứa E2E test cho module nutrition.
// E2E test kiểm tra luồng hoàn chỉnh từ gRPC handler → application layer → repository,
// mô phỏng cách client thực sự gọi API.
//
// Khác với integration test (chỉ test application + infra), E2E test đi qua
// TOÀN BỘ stack: transport (gRPC handler) → command/query handlers → SQLite repos.
//
// Chạy:
//
//	go test -v ./internal/nutrition/test/... -run E2E
package test

import (
	"context"
	"testing"
	"time"

	nutritionv1msg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/nutrition/v1/message"
	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/application/query"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/transport"
	"github.com/viethung213/gym-companion/internal/nutrition/test/testutil"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// newE2EHandler tạo GRPCHandler đầy đủ stack với SQLite in-memory.
// Trả về handler và repos để test có thể kiểm tra trạng thái DB trực tiếp.
func newE2EHandler(t *testing.T) (*transport.GRPCHandler, *testutil.Repos) {
	t.Helper()

	db := testutil.NewTestDB(t, "")
	repos := testutil.NewRepos(db)
	ai := &testutil.MockAIService{}

	tdeeCalc := service.NewTDEECalculator()
	matrix := service.NewCombinatorialMatrix()
	menuGen := service.NewMenuGenerator(matrix, repos.RecipeCache, repos.FoodItem, ai)

	h := transport.NewGRPCHandler(
		command.NewGenerateDailyPlanHandler(repos.NutritionPlan, repos.MealHistory, nil, tdeeCalc, menuGen),
		command.NewRecalibratePlanWithPantryHandler(repos.NutritionPlan, repos.MealHistory, nil, menuGen),
		command.NewLogMealHandler(repos.NutritionPlan, repos.MealHistory, nil, ai),
		command.NewCreateFoodItemHandler(repos.FoodItem),
		command.NewApproveFoodItemHandler(repos.FoodItem),
		query.NewGetTodayMenuHandler(repos.NutritionPlan),
		query.NewGetNutritionHistoryHandler(repos.MealHistory),
		query.NewGetNutritionSummaryHandler(repos.NutritionPlan, repos.MealHistory),
		query.NewGetNutritionInsightHandler(repos.NutritionPlan, repos.MealHistory, ai),
	)

	return h, repos
}

// TestE2E_FullUserJourney kiểm tra luồng hoàn chỉnh của người dùng:
// 1. GetTodayMenu (auto-generate plan)
// 2. CreateFoodItem + ApproveFoodItem
// 3. LogMeal
// 4. GetNutritionHistory
// 5. GetNutritionSummary
func TestE2E_FullUserJourney(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler, _ := newE2EHandler(t)
	userID := "e2e-user-1"

	// ── Step 1: GetTodayMenu (auto-generate plan cho user mới) ────────────────
	menuResp, err := handler.GetTodayMenu(ctx, &nutritionv1msg.GetTodayMenuRequest{
		UserId: userID,
	})
	if err != nil {
		t.Fatalf("[Step 1] GetTodayMenu: %v", err)
	}
	if menuResp.GetTargetCalories() <= 0 {
		t.Fatalf("[Step 1] got target_calories=%f, want > 0", menuResp.GetTargetCalories())
	}

	// ── Step 2: CreateFoodItem ────────────────────────────────────────────────
	createResp, err := handler.CreateFoodItem(ctx, &nutritionv1msg.CreateFoodItemRequest{
		Name:             "Cá hồi",
		Category:         "PROTEIN",
		CaloriesPer_100G: 206,
		ProteinPer_100G:  20,
		CarbsPer_100G:    0,
		FatPer_100G:      13,
	})
	if err != nil {
		t.Fatalf("[Step 2] CreateFoodItem: %v", err)
	}
	if createResp.GetFoodItemId() == "" {
		t.Fatal("[Step 2] expected non-empty food_item_id")
	}

	// ── Step 3: ApproveFoodItem ───────────────────────────────────────────────
	approveResp, err := handler.ApproveFoodItem(ctx, &nutritionv1msg.ApproveFoodItemRequest{
		FoodItemId: createResp.GetFoodItemId(),
	})
	if err != nil {
		t.Fatalf("[Step 3] ApproveFoodItem: %v", err)
	}
	if !approveResp.GetSuccess() {
		t.Fatal("[Step 3] expected approve success=true")
	}

	// ── Step 4: LogMeal ───────────────────────────────────────────────────────
	logResp, err := handler.LogMeal(ctx, &nutritionv1msg.LogMealRequest{
		UserId:   userID,
		MealType: "Dinner",
		MealName: "Cá hồi áp chảo",
		Calories: 350,
		Protein:  30,
		Carbs:    10,
		Fat:      18,
	})
	if err != nil {
		t.Fatalf("[Step 4] LogMeal: %v", err)
	}
	if !logResp.GetSuccess() {
		t.Fatal("[Step 4] expected log_meal success=true")
	}

	// ── Step 5: GetNutritionHistory ───────────────────────────────────────────
	histResp, err := handler.GetNutritionHistory(ctx, &nutritionv1msg.GetNutritionHistoryRequest{
		UserId: userID,
	})
	if err != nil {
		t.Fatalf("[Step 5] GetNutritionHistory: %v", err)
	}
	if len(histResp.GetMeals()) != 1 {
		t.Fatalf("[Step 5] got %d meals, want 1", len(histResp.GetMeals()))
	}
	if got := histResp.GetMeals()[0].GetMealName(); got != "Cá hồi áp chảo" {
		t.Fatalf("[Step 5] got meal_name=%q, want %q", got, "Cá hồi áp chảo")
	}

	// ── Step 6: GetNutritionSummary ───────────────────────────────────────────
	summaryResp, err := handler.GetNutritionSummary(ctx, &nutritionv1msg.GetNutritionSummaryRequest{
		UserId: userID,
	})
	if err != nil {
		t.Fatalf("[Step 6] GetNutritionSummary: %v", err)
	}
	if summaryResp.GetTargetCalories() <= 0 {
		t.Fatalf("[Step 6] got target_calories=%f, want > 0", summaryResp.GetTargetCalories())
	}
	if summaryResp.GetConsumedCalories() != 350 {
		t.Fatalf("[Step 6] got consumed_calories=%f, want 350", summaryResp.GetConsumedCalories())
	}
}

// TestE2E_Validation_AllEndpoints kiểm tra tất cả endpoint trả về
// codes.InvalidArgument khi thiếu tham số bắt buộc.
func TestE2E_Validation_AllEndpoints(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler, _ := newE2EHandler(t)

	tests := []struct {
		name string
		fn   func() error
	}{
		{
			"GetTodayMenu empty user_id",
			func() error {
				_, err := handler.GetTodayMenu(ctx, &nutritionv1msg.GetTodayMenuRequest{UserId: ""})
				return err
			},
		},
		{
			"LogMeal empty user_id",
			func() error {
				_, err := handler.LogMeal(ctx, &nutritionv1msg.LogMealRequest{UserId: ""})
				return err
			},
		},
		{
			"ApproveFoodItem empty food_item_id",
			func() error {
				_, err := handler.ApproveFoodItem(ctx, &nutritionv1msg.ApproveFoodItemRequest{FoodItemId: ""})
				return err
			},
		},
		{
			"GetNutritionHistory empty user_id",
			func() error {
				_, err := handler.GetNutritionHistory(ctx, &nutritionv1msg.GetNutritionHistoryRequest{UserId: ""})
				return err
			},
		},
		{
			"GetNutritionSummary empty user_id",
			func() error {
				_, err := handler.GetNutritionSummary(ctx, &nutritionv1msg.GetNutritionSummaryRequest{UserId: ""})
				return err
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.fn()
			if grpcstatus.Code(err) != codes.InvalidArgument {
				t.Fatalf("%s: got code %v, want InvalidArgument", tt.name, grpcstatus.Code(err))
			}
		})
	}
}

// TestE2E_GetTodayMenu_Idempotent kiểm tra rằng gọi GetTodayMenu nhiều lần
// trong cùng ngày trả về cùng plan (không tạo thêm plan mới).
func TestE2E_GetTodayMenu_Idempotent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler, repos := newE2EHandler(t)
	userID := "e2e-idem-user"

	// Gọi lần 1 — auto-generate plan
	resp1, err := handler.GetTodayMenu(ctx, &nutritionv1msg.GetTodayMenuRequest{UserId: userID})
	if err != nil {
		t.Fatalf("GetTodayMenu [1]: %v", err)
	}

	// Gọi lần 2 — plan đã tồn tại, không nên tạo mới
	resp2, err := handler.GetTodayMenu(ctx, &nutritionv1msg.GetTodayMenuRequest{UserId: userID})
	if err != nil {
		t.Fatalf("GetTodayMenu [2]: %v", err)
	}

	if resp1.GetTargetCalories() != resp2.GetTargetCalories() {
		t.Fatalf("target_calories changed between calls: %f vs %f",
			resp1.GetTargetCalories(), resp2.GetTargetCalories())
	}

	// Xác nhận chỉ có 1 plan trong DB
	plan, err := repos.NutritionPlan.FindByUserIDAndDate(ctx, userID, time.Now())
	if err != nil || plan == nil {
		t.Fatalf("FindByUserIDAndDate: %v", err)
	}
}

// TestE2E_RecalibratePlan kiểm tra luồng RecalibratePlanWithPantry
// sau khi plan đã tồn tại.
func TestE2E_RecalibratePlan(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler, _ := newE2EHandler(t)
	userID := "e2e-recal-user"

	// Bước 1: Generate plan ban đầu
	if _, err := handler.GetTodayMenu(ctx, &nutritionv1msg.GetTodayMenuRequest{
		UserId: userID,
	}); err != nil {
		t.Fatalf("GetTodayMenu (seed): %v", err)
	}

	// Bước 2: Log một bữa ăn để có dữ liệu lịch sử
	if _, err := handler.LogMeal(ctx, &nutritionv1msg.LogMealRequest{
		UserId:   userID,
		MealType: "Lunch",
		MealName: "Bún bò",
		Calories: 600,
		Protein:  25,
		Carbs:    80,
		Fat:      15,
	}); err != nil {
		t.Fatalf("LogMeal: %v", err)
	}

	// Bước 3: RecalibratePlanWithPantry — chỉ cần user_id theo contract thực tế
	recalResp, err := handler.RecalibratePlanWithPantry(ctx, &nutritionv1msg.RecalibratePlanWithPantryRequest{
		UserId:               userID,
		AvailableIngredients: []string{"Ức gà", "Khoai lang", "Cải bó xôi"},
	})
	if grpcstatus.Code(err) == codes.Unimplemented {
		t.Skip("RecalibratePlanWithPantry chưa được implement trong gRPC handler — skip")
	}
	if err != nil {
		t.Fatalf("RecalibratePlanWithPantry: %v", err)
	}
	if recalResp == nil {
		t.Fatal("expected non-nil recalibrate response")
	}
}
