package query_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/application/query"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/entity"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
	"github.com/viethung213/gym-companion/internal/nutrition/test/testutil"
)

type mockPlanRepo struct {
	plans map[string]*aggregate.NutritionPlan
}

func (m *mockPlanRepo) FindByUserIDAndDate(_ context.Context, userID string, date time.Time) (*aggregate.NutritionPlan, error) {
	key := userID + date.Format("2006-01-02")
	return m.plans[key], nil
}

func (m *mockPlanRepo) FindByUserIDAndDates(_ context.Context, userID string, dates []time.Time) ([]*aggregate.NutritionPlan, error) {
	var result []*aggregate.NutritionPlan
	for _, d := range dates {
		key := userID + d.Format("2006-01-02")
		if p, ok := m.plans[key]; ok {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockPlanRepo) Save(_ context.Context, plan *aggregate.NutritionPlan) error {
	key := plan.UserID() + plan.PlanDate().Format("2006-01-02")
	m.plans[key] = plan
	return nil
}

func (m *mockPlanRepo) Update(_ context.Context, plan *aggregate.NutritionPlan) error {
	return m.Save(context.Background(), plan)
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

type mockHistoryRepo struct {
	history *aggregate.MealHistory
}

func (m *mockHistoryRepo) FindByUserID(_ context.Context, _ string) (*aggregate.MealHistory, error) {
	return m.history, nil
}

func (m *mockHistoryRepo) FindByUserIDAndDateRange(_ context.Context, _ string, _, _ time.Time) (*aggregate.MealHistory, error) {
	return m.history, nil
}

func (m *mockHistoryRepo) Save(_ context.Context, history *aggregate.MealHistory) error {
	m.history = history
	return nil
}

type mockWeeklyInsightRepo struct {
	insights map[string]*entity.WeeklyInsight
}

func (m *mockWeeklyInsightRepo) Save(_ context.Context, insight *entity.WeeklyInsight) error {
	m.insights[insight.UserID()] = insight
	return nil
}

func (m *mockWeeklyInsightRepo) GetLatestByUserID(_ context.Context, userID string) (*entity.WeeklyInsight, error) {
	return m.insights[userID], nil
}

func (m *mockWeeklyInsightRepo) GetByUserIDAndStartDate(_ context.Context, userID string, _ time.Time) (*entity.WeeklyInsight, error) {
	return m.insights[userID], nil
}

func TestQueryHandlers(t *testing.T) {
	t.Parallel()

	now := time.Now()
	alloc, _ := vo.NewCalorieAllocation(2000, 150, 200, 66)
	plan := aggregate.NewNutritionPlan("plan-q-1", "user-q-1", now, alloc, nil)

	planRepo := &mockPlanRepo{plans: make(map[string]*aggregate.NutritionPlan)}
	_ = planRepo.Save(context.Background(), plan)

	history := aggregate.NewMealHistory("hist-q-1", "user-q-1")
	logItem := aggregate.NewMealLog("log-q-1", "hist-q-1", "user-q-1", "Lunch", "Ức gà", "1 dĩa", 400, 35, 10, 5, now)
	history.AddMealLog(logItem)

	historyRepo := &mockHistoryRepo{history: history}

	// 1. GetTodayMenuHandler
	todayMenuHdlr := query.NewGetTodayMenuHandler(planRepo)
	planResult, err := todayMenuHdlr.Handle(context.Background(), query.GetTodayMenuQuery{UserID: "user-q-1", PlanDate: now})
	if err != nil {
		t.Fatalf("unexpected error getting today menu: %v", err)
	}
	if planResult == nil || planResult.ID() != "plan-q-1" {
		t.Fatalf("got plan %v, want plan ID plan-q-1", planResult)
	}

	// 2. GetNutritionHistoryHandler
	historyHdlr := query.NewGetNutritionHistoryHandler(historyRepo)
	histResult, err := historyHdlr.Handle(context.Background(), query.GetNutritionHistoryQuery{UserID: "user-q-1"})
	if err != nil {
		t.Fatalf("unexpected error getting nutrition history: %v", err)
	}
	if got := len(histResult.MealLogs()); got != 1 {
		t.Fatalf("got meal logs len %d, want 1", got)
	}

	// 3. GetNutritionSummaryHandler
	summaryHdlr := query.NewGetNutritionSummaryHandler(planRepo, historyRepo)
	summary, err := summaryHdlr.Handle(context.Background(), query.GetNutritionSummaryQuery{UserID: "user-q-1", PlanDate: now})
	if err != nil {
		t.Fatalf("unexpected error getting nutrition summary: %v", err)
	}
	if summary.TargetCalories != 2000 {
		t.Fatalf("got target calories %f, want 2000", summary.TargetCalories)
	}
	if summary.ConsumedCalories != 400 {
		t.Fatalf("got consumed calories %f, want 400", summary.ConsumedCalories)
	}

	// 4. GetNutritionInsightHandler (Đọc từ DB, không gọi LLM)
	weeklyRepo := &mockWeeklyInsightRepo{insights: make(map[string]*entity.WeeklyInsight)}
	seedInsight := testutil.MustNewWeeklyInsight(
		"insight-q-1",
		"user-q-1",
		now.AddDate(0, 0, -6),
		now,
		88,
		"Chế độ ăn rất tốt.",
		"Hấp, luộc thanh đạm",
		[]string{"Ức gà", "Cơm trắng"},
		[]entity.WeeklyImprovementArea{
			{Nutrient: "PROTEIN", Action: "INCREASE", DeltaAmount: 15.0, Suggestion: "Tăng 15g đạm"},
		},
		&entity.WeeklyMacroSummary{
			ConsumedCalories: 14000,
			TargetCalories:   14000,
		},
	)
	_ = weeklyRepo.Save(context.Background(), seedInsight)

	insightHdlr := query.NewGetNutritionInsightHandler(weeklyRepo)
	insightRes, err := insightHdlr.Handle(context.Background(), query.GetNutritionInsightQuery{UserID: "user-q-1"})
	if err != nil {
		t.Fatalf("unexpected error getting nutrition insight: %v", err)
	}
	if insightRes == nil {
		t.Fatalf("expected insightRes not nil")
	}
	if insightRes.OverallScore() != 88 {
		t.Fatalf("got overall score %d, want 88", insightRes.OverallScore())
	}
	if insightRes.PredictedCookingStyle() != "Hấp, luộc thanh đạm" {
		t.Fatalf("got predicted cooking style %q, want 'Hấp, luộc thanh đạm'", insightRes.PredictedCookingStyle())
	}
}
