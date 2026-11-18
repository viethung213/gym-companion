package worker_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/entity"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/worker"
)

type mockWeeklyInsightRepoForWorker struct {
	saved map[string]*entity.WeeklyInsight
}

func (m *mockWeeklyInsightRepoForWorker) Save(_ context.Context, insight *entity.WeeklyInsight) error {
	m.saved[insight.UserID()] = insight
	return nil
}

func (m *mockWeeklyInsightRepoForWorker) GetLatestByUserID(_ context.Context, userID string) (*entity.WeeklyInsight, error) {
	return m.saved[userID], nil
}

func (m *mockWeeklyInsightRepoForWorker) GetByUserIDAndStartDate(_ context.Context, userID string, _ time.Time) (*entity.WeeklyInsight, error) {
	return m.saved[userID], nil
}

type mockAIServiceForWorker struct{}

func (m *mockAIServiceForWorker) SelectCreativeMealOptions(_ context.Context, _ repository.AIMenuPromptContext) ([]repository.GeneratedRecipeResult, error) {
	return nil, nil
}

func (m *mockAIServiceForWorker) EstimateNutrient(_ context.Context, _, _ string) (*repository.EstimatedNutrientResult, error) {
	return nil, nil
}

func (m *mockAIServiceForWorker) GenerateNutritionInsight(_ context.Context, _ repository.InsightPromptContext) (*repository.NutritionInsightResult, error) {
	return &repository.NutritionInsightResult{
		OverallScore:          85,
		Summary:               "Tốt",
		PredictedCookingStyle: "Hấp luộc",
		PreferredIngredients:  []string{"Ức gà"},
		ImprovementAreas:      nil,
	}, nil
}

type mockPlanRepoForWeeklyWorker struct {
	activeUsers []string
}

func (m *mockPlanRepoForWeeklyWorker) FindActiveUserIDs(_ context.Context, _ int) ([]string, error) {
	return m.activeUsers, nil
}

func (m *mockPlanRepoForWeeklyWorker) FindByUserIDAndDate(_ context.Context, _ string, _ time.Time) (*aggregate.NutritionPlan, error) {
	return nil, nil
}

func (m *mockPlanRepoForWeeklyWorker) FindByUserIDAndDates(_ context.Context, _ string, _ []time.Time) ([]*aggregate.NutritionPlan, error) {
	return nil, nil
}

func (m *mockPlanRepoForWeeklyWorker) Save(_ context.Context, _ *aggregate.NutritionPlan) error {
	return nil
}
func (m *mockPlanRepoForWeeklyWorker) Update(_ context.Context, _ *aggregate.NutritionPlan) error {
	return nil
}
func (m *mockPlanRepoForWeeklyWorker) FindPlansForDate(_ context.Context, _ time.Time) ([]*aggregate.NutritionPlan, error) {
	return nil, nil
}
func (m *mockPlanRepoForWeeklyWorker) GetUserMealSchedules(_ context.Context, _ string) (map[string]string, error) {
	return nil, nil
}
func (m *mockPlanRepoForWeeklyWorker) SaveUserMealSchedules(_ context.Context, _ string, _ map[string]string) error {
	return nil
}

type mockCronCoordinatorForWeeklyWorker struct {
	tasks       map[string][]string
	claimed     map[string]bool
	completed   map[string]bool
	mu          sync.Mutex
	ensureCalls int
}

func newMockCronCoordinatorForWeeklyWorker() *mockCronCoordinatorForWeeklyWorker {
	return &mockCronCoordinatorForWeeklyWorker{
		tasks:     make(map[string][]string),
		claimed:   make(map[string]bool),
		completed: make(map[string]bool),
	}
}

func (m *mockCronCoordinatorForWeeklyWorker) EnsureTaskSet(_ context.Context, jobName, targetDate string, loaderFn func() ([]string, error)) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureCalls++
	key := jobName + ":" + targetDate
	if list, ok := m.tasks[key]; ok {
		return list, nil
	}
	users, err := loaderFn()
	if err != nil {
		return nil, err
	}
	m.tasks[key] = users
	return users, nil
}

func (m *mockCronCoordinatorForWeeklyWorker) TryClaimUser(_ context.Context, jobName, targetDate, userID string, _ time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := jobName + ":" + targetDate + ":" + userID
	if m.completed[key] || m.claimed[key] {
		return false, nil
	}
	m.claimed[key] = true
	return true, nil
}

func (m *mockCronCoordinatorForWeeklyWorker) CompleteUser(_ context.Context, jobName, targetDate, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := jobName + ":" + targetDate + ":" + userID
	m.completed[key] = true
	delete(m.claimed, key)
	return nil
}

func (m *mockCronCoordinatorForWeeklyWorker) ReleaseUserLock(_ context.Context, jobName, targetDate, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := jobName + ":" + targetDate + ":" + userID
	delete(m.claimed, key)
	return nil
}

func TestNextSundayTarget(t *testing.T) {
	t.Parallel()

	// 1. Giả lập ngày Thứ Hai lúc 10:00 (2026-11-16 là Thứ Hai)
	monday := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	target := worker.NextSundayTarget(monday, 23, 0)
	if target.Weekday() != time.Sunday {
		t.Fatalf("expected weekday Sunday, got %v", target.Weekday())
	}
	if target.Day() != 22 {
		t.Fatalf("expected Sunday 2026-11-22, got %v", target)
	}
	if target.Hour() != 23 || target.Minute() != 0 {
		t.Fatalf("expected 23:00, got %02d:%02d", target.Hour(), target.Minute())
	}

	// 2. Giả lập ngày Chủ Nhật lúc 14:00 (2026-11-22 là Chủ Nhật) -> chạy tối nay 23:00
	sundayAfternoon := time.Date(2026, 11, 22, 14, 0, 0, 0, time.UTC)
	targetToday := worker.NextSundayTarget(sundayAfternoon, 23, 0)
	if targetToday.Day() != 22 || targetToday.Hour() != 23 {
		t.Fatalf("expected tonight 2026-11-22 23:00, got %v", targetToday)
	}

	// 3. Giả lập ngày Chủ Nhật lúc 23:30 (đã qua giờ chạy) -> chạy Chủ Nhật tuần sau
	sundayLate := time.Date(2026, 11, 22, 23, 30, 0, 0, time.UTC)
	targetNextWeek := worker.NextSundayTarget(sundayLate, 23, 0)
	if targetNextWeek.Day() != 29 {
		t.Fatalf("expected next Sunday 2026-11-29, got %v", targetNextWeek)
	}
}

func TestWeeklyInsightCronWorker_RunOnce(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Now()

	// Setup active user with meal history
	planRepo := &mockPlanRepoForWeeklyWorker{activeUsers: []string{"active-user-1", "active-user-2"}}
	historyRepo := &mockHistoryRepoForWorker{
		histories: make(map[string]*aggregate.MealHistory),
	}

	h1 := aggregate.NewMealHistory("h-1", "active-user-1")
	h1.AddMealLog(aggregate.NewMealLog("l-1", "h-1", "active-user-1", "Breakfast", "Cơm", "1 chén", 200, 5, 45, 1, now))
	historyRepo.histories["active-user-1"] = h1

	h2 := aggregate.NewMealHistory("h-2", "active-user-2")
	h2.AddMealLog(aggregate.NewMealLog("l-2", "h-2", "active-user-2", "Dinner", "Thịt bò", "100g", 250, 26, 0, 15, now))
	historyRepo.histories["active-user-2"] = h2

	weeklyRepo := &mockWeeklyInsightRepoForWorker{saved: make(map[string]*entity.WeeklyInsight)}
	aiService := &mockAIServiceForWorker{}

	coord := newMockCronCoordinatorForWeeklyWorker()
	genHandler := command.NewGenerateWeeklyInsightHandler(planRepo, historyRepo, weeklyRepo, aiService)
	workerInstance := worker.NewWeeklyInsightCronWorker(genHandler, planRepo, coord, 23, 0)

	// Chạy thử RunOnce
	success, fail, err := workerInstance.RunOnce(ctx)
	if err != nil {
		t.Fatalf("unexpected error running RunOnce: %v", err)
	}
	if success != 2 || fail != 0 {
		t.Errorf("got success=%d, fail=%d; want success=2, fail=0", success, fail)
	}
	if len(weeklyRepo.saved) != 2 {
		t.Errorf("got %d saved insights, want 2", len(weeklyRepo.saved))
	}
}

func TestWeeklyInsightCronWorker_SkipsUserWhenApplicationReturnsNil(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Now()

	// user-1 có meal log, user-2 không có meal log nào
	planRepo := &mockPlanRepoForWeeklyWorker{activeUsers: []string{"user-with-meals", "user-no-meals"}}
	historyRepo := &mockHistoryRepoForWorker{
		histories: make(map[string]*aggregate.MealHistory),
	}

	h1 := aggregate.NewMealHistory("h-1", "user-with-meals")
	h1.AddMealLog(aggregate.NewMealLog("l-1", "h-1", "user-with-meals", "Lunch", "Phở", "1 tô", 500, 25, 60, 15, now))
	historyRepo.histories["user-with-meals"] = h1

	// user-no-meals không có log nào
	historyRepo.histories["user-no-meals"] = aggregate.NewMealHistory("h-2", "user-no-meals")

	weeklyRepo := &mockWeeklyInsightRepoForWorker{saved: make(map[string]*entity.WeeklyInsight)}
	aiService := &mockAIServiceForWorker{}

	coord := newMockCronCoordinatorForWeeklyWorker()
	genHandler := command.NewGenerateWeeklyInsightHandler(planRepo, historyRepo, weeklyRepo, aiService)
	workerInstance := worker.NewWeeklyInsightCronWorker(genHandler, planRepo, coord, 23, 0)

	success, fail, err := workerInstance.RunOnce(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if success != 1 || fail != 0 {
		t.Errorf("got success=%d, fail=%d; want success=1, fail=0", success, fail)
	}
	if len(weeklyRepo.saved) != 1 {
		t.Errorf("got %d saved insights, want 1", len(weeklyRepo.saved))
	}
	if _, ok := weeklyRepo.saved["user-with-meals"]; !ok {
		t.Errorf("expected user-with-meals to have saved insight")
	}
	if _, ok := weeklyRepo.saved["user-no-meals"]; ok {
		t.Errorf("expected user-no-meals NOT to have saved insight")
	}
}

func TestWeeklyInsightCronWorker_NilCoordinator_ReturnsError(t *testing.T) {
	t.Parallel()

	planRepo := &mockPlanRepoForWeeklyWorker{activeUsers: []string{"active-user-1"}}
	workerInstance := worker.NewWeeklyInsightCronWorker(nil, planRepo, nil, 23, 0)

	_, _, err := workerInstance.RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected error when coordinator is nil, got nil")
	}
}

func TestWeeklyInsightCronWorker_StartStop(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	workerInstance := worker.NewWeeklyInsightCronWorker(nil, nil, nil, 23, 0)
	if err := workerInstance.Start(ctx); err != nil {
		t.Fatalf("failed to start worker: %v", err)
	}

	// Dừng worker qua Stop
	workerInstance.Stop()
}

type mockHistoryRepoForWorker struct {
	histories map[string]*aggregate.MealHistory
}

func (m *mockHistoryRepoForWorker) FindByUserID(_ context.Context, userID string) (*aggregate.MealHistory, error) {
	return m.histories[userID], nil
}

func (m *mockHistoryRepoForWorker) FindByUserIDAndDateRange(_ context.Context, userID string, _, _ time.Time) (*aggregate.MealHistory, error) {
	return m.histories[userID], nil
}

func (m *mockHistoryRepoForWorker) Save(_ context.Context, history *aggregate.MealHistory) error {
	m.histories[history.UserID()] = history
	return nil
}
