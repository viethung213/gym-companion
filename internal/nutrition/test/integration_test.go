// Package test chứa integration test cho module nutrition.
// Integration test kiểm tra luồng qua nhiều layer (application → infrastructure)
// nhưng KHÔNG thông qua mạng/gRPC thực sự — dùng SQLite in-memory thay PostgreSQL.
//
// Chạy:
//
//	go test -v ./internal/nutrition/test/... -run Integration
package test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/application/query"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
	"github.com/viethung213/gym-companion/internal/nutrition/test/testutil"
)

// TestIntegration_GenerateDailyPlan_And_LogMeal kiểm tra luồng hoàn chỉnh:
// GenerateDailyPlan → lưu plan vào DB → LogMeal → kiểm tra history.
func TestIntegration_GenerateDailyPlan_And_LogMeal(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.NewTestDB(t, "")
	repos := testutil.NewRepos(db)
	ai := &testutil.MockAIService{}

	tdeeCalc := service.NewTDEECalculator()
	matrix := service.NewCombinatorialMatrix()
	menuGen := service.NewMenuGenerator(matrix, repos.RecipeCache, repos.FoodItem, ai)

	// Seed một FoodItem để menu generator có dữ liệu
	food := testutil.NewApprovedFoodItem("fi-int-1", "Ức gà", "PROTEIN", 165, 31, 0, 3.6)
	if err := repos.FoodItem.Save(ctx, food); err != nil {
		t.Fatalf("seed food item: %v", err)
	}

	// 1. GenerateDailyPlan
	genHdlr := command.NewGenerateDailyPlanHandler(
		repos.NutritionPlan, repos.MealHistory, nil, tdeeCalc, menuGen,
	)
	plan, err := genHdlr.Handle(ctx, command.GenerateDailyPlanCommand{
		UserID:   "int-user-1",
		PlanDate: time.Now(),
		BiologicalMetrics: service.BiologicalMetrics{
			WeightKg:      70,
			HeightCm:      170,
			Age:           28,
			Gender:        "MALE",
			ActivityLevel: "MODERATELY_ACTIVE",
		},
	})
	if err != nil {
		t.Fatalf("GenerateDailyPlan: %v", err)
	}
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}

	// 2. Xác nhận plan đã được lưu vào DB
	saved, err := repos.NutritionPlan.FindByUserIDAndDate(ctx, "int-user-1", time.Now())
	if err != nil || saved == nil {
		t.Fatalf("FindByUserIDAndDate after generate: %v", err)
	}
	if saved.UserID() != "int-user-1" {
		t.Fatalf("got userID %q, want %q", saved.UserID(), "int-user-1")
	}

	// 3. LogMeal
	logHdlr := command.NewLogMealHandler(repos.NutritionPlan, repos.MealHistory, nil, ai)
	_, err = logHdlr.Handle(ctx, command.LogMealCommand{
		UserID:   "int-user-1",
		MealType: "Lunch",
		MealName: "Ức gà áp chảo",
		Calories: 450,
		Protein:  35,
		Carbs:    20,
		Fat:      10,
	})
	if err != nil {
		t.Fatalf("LogMeal: %v", err)
	}

	// 4. Xác nhận meal history được lưu
	history, err := repos.MealHistory.FindByUserID(ctx, "int-user-1")
	if err != nil || history == nil {
		t.Fatalf("FindByUserID history: %v", err)
	}
	if len(history.MealLogs()) != 1 {
		t.Fatalf("got %d meal logs, want 1", len(history.MealLogs()))
	}
	if got := history.MealLogs()[0].MealName(); got != "Ức gà áp chảo" {
		t.Fatalf("got meal name %q, want %q", got, "Ức gà áp chảo")
	}
}

// TestIntegration_GetTodayMenu_From_ExistingPlan kiểm tra query GetTodayMenu
// khi plan đã tồn tại trong DB (không cần generate mới).
func TestIntegration_GetTodayMenu_From_ExistingPlan(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.NewTestDB(t, "")
	repos := testutil.NewRepos(db)

	// Seed plan trực tiếp vào DB
	now := time.Now()
	plan := testutil.NewTestNutritionPlan("plan-int-1", "int-user-2", now)
	if err := repos.NutritionPlan.Save(ctx, plan); err != nil {
		t.Fatalf("seed plan: %v", err)
	}

	// Query GetTodayMenu — phải trả về plan đã tồn tại, không generate mới
	getMenuHdlr := query.NewGetTodayMenuHandler(repos.NutritionPlan)
	result, err := getMenuHdlr.Handle(ctx, query.GetTodayMenuQuery{
		UserID:   "int-user-2",
		PlanDate: now,
	})

	if err != nil {
		t.Fatalf("GetTodayMenu: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ID() != "plan-int-1" {
		t.Fatalf("got plan ID %q, want %q", result.ID(), "plan-int-1")
	}
}

// TestIntegration_NutritionSummary kiểm tra GetNutritionSummary
// phản ánh đúng lượng calo đã tiêu thụ sau khi log meal.
func TestIntegration_NutritionSummary(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.NewTestDB(t, "")
	repos := testutil.NewRepos(db)
	ai := &testutil.MockAIService{}

	now := time.Now()

	// Seed plan
	plan := testutil.NewTestNutritionPlan("plan-sum-1", "sum-user-1", now)
	if err := repos.NutritionPlan.Save(ctx, plan); err != nil {
		t.Fatalf("seed plan: %v", err)
	}

	// Log 2 bữa ăn
	logHdlr := command.NewLogMealHandler(repos.NutritionPlan, repos.MealHistory, nil, ai)
	meals := []struct {
		name string
		cal  float64
	}{
		{"Bữa sáng", 400},
		{"Bữa trưa", 600},
	}
	for i, meal := range meals {
		if _, err := logHdlr.Handle(ctx, command.LogMealCommand{
			UserID:   "sum-user-1",
			MealType: "Meal",
			MealName: meal.name,
			Calories: meal.cal,
			Protein:  30,
			Carbs:    50,
			Fat:      10,
		}); err != nil {
			t.Fatalf("LogMeal[%d]: %v", i, err)
		}
	}

	// GetNutritionSummary
	summaryHdlr := query.NewGetNutritionSummaryHandler(repos.NutritionPlan, repos.MealHistory)
	summary, err := summaryHdlr.Handle(ctx, query.GetNutritionSummaryQuery{
		UserID:   "sum-user-1",
		PlanDate: now,
	})
	if err != nil {
		t.Fatalf("GetNutritionSummary: %v", err)
	}
	if summary.TargetCalories <= 0 {
		t.Fatalf("got target calories %v, want > 0", summary.TargetCalories)
	}
	if summary.ConsumedCalories != 1000 {
		t.Fatalf("got consumed calories %v, want 1000", summary.ConsumedCalories)
	}
}

// TestIntegration_CreateAndApprove_FoodItem kiểm tra vòng đời FoodItem:
// Create (PENDING) → SubmitForApproval → Approve (ACTIVE) → FindActiveCatalog.
func TestIntegration_CreateAndApprove_FoodItem(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.NewTestDB(t, "")
	repos := testutil.NewRepos(db)

	createHdlr := command.NewCreateFoodItemHandler(repos.FoodItem)
	approveHdlr := command.NewApproveFoodItemHandler(repos.FoodItem)

	// 1. Create — trả về *aggregate.FoodItem
	created, err := createHdlr.Handle(ctx, command.CreateFoodItemCommand{
		Name:              "Thịt bò nạc",
		Category:          "PROTEIN",
		CaloriesPer100g:   250,
		ProteinPer100g:    26,
		CarbsPer100g:      0,
		FatPer100g:        15,
		IsNutiFoodProduct: false,
	})
	if err != nil || created == nil {
		t.Fatalf("CreateFoodItem: item=%v, err=%v", created, err)
	}

	// 2. Approve
	if _, err = approveHdlr.Handle(ctx, command.ApproveFoodItemCommand{
		FoodItemID: created.ID(),
	}); err != nil {
		t.Fatalf("ApproveFoodItem: %v", err)
	}

	// 3. Xác nhận trong catalog
	catalog, err := repos.FoodItem.FindActiveCatalog(ctx)
	if err != nil {
		t.Fatalf("FindActiveCatalog: %v", err)
	}
	found := false
	for _, item := range catalog {
		if item.Name() == "Thịt bò nạc" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("approved food item not found in active catalog")
	}
}
