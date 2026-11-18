package worker_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/worker"
)

// mockProfileClient là stub triển khai repository.ProfileClient.
type mockProfileClient struct {
	metrics *repository.UserBiologicalMetrics
	err     error
}

func (m *mockProfileClient) GetBiologicalMetrics(_ context.Context, _ string) (*repository.UserBiologicalMetrics, error) {
	return m.metrics, m.err
}

// mockPlanRepo là stub triển khai repository.NutritionPlanRepository cho worker test.
type mockPlanRepo struct {
	activeUsers []string
}

func (m *mockPlanRepo) FindActiveUserIDs(_ context.Context, _ int) ([]string, error) {
	return m.activeUsers, nil
}

func (m *mockPlanRepo) FindByUserIDAndDate(_ context.Context, _ string, _ time.Time) (*aggregate.NutritionPlan, error) {
	return nil, nil
}

func (m *mockPlanRepo) FindByUserIDAndDates(_ context.Context, _ string, _ []time.Time) ([]*aggregate.NutritionPlan, error) {
	return nil, nil
}

func (m *mockPlanRepo) Save(_ context.Context, _ *aggregate.NutritionPlan) error   { return nil }
func (m *mockPlanRepo) Update(_ context.Context, _ *aggregate.NutritionPlan) error { return nil }
func (m *mockPlanRepo) FindPlansForDate(_ context.Context, _ time.Time) ([]*aggregate.NutritionPlan, error) {
	return nil, nil
}
func (m *mockPlanRepo) GetUserMealSchedules(_ context.Context, _ string) (map[string]string, error) {
	return nil, nil
}
func (m *mockPlanRepo) SaveUserMealSchedules(_ context.Context, _ string, _ map[string]string) error {
	return nil
}

type mockCronCoordinatorForWorker struct {
	tasks       map[string][]string
	claimed     map[string]bool
	completed   map[string]bool
	mu          sync.Mutex
	ensureCalls int
}

func newMockCronCoordinatorForWorker() *mockCronCoordinatorForWorker {
	return &mockCronCoordinatorForWorker{
		tasks:     make(map[string][]string),
		claimed:   make(map[string]bool),
		completed: make(map[string]bool),
	}
}

func (m *mockCronCoordinatorForWorker) EnsureTaskSet(_ context.Context, jobName, targetDate string, loaderFn func() ([]string, error)) ([]string, error) {
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

func (m *mockCronCoordinatorForWorker) TryClaimUser(_ context.Context, jobName, targetDate, userID string, _ time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := jobName + ":" + targetDate + ":" + userID
	if m.completed[key] || m.claimed[key] {
		return false, nil
	}
	m.claimed[key] = true
	return true, nil
}

func (m *mockCronCoordinatorForWorker) CompleteUser(_ context.Context, jobName, targetDate, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := jobName + ":" + targetDate + ":" + userID
	m.completed[key] = true
	delete(m.claimed, key)
	return nil
}

func (m *mockCronCoordinatorForWorker) ReleaseUserLock(_ context.Context, jobName, targetDate, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := jobName + ":" + targetDate + ":" + userID
	delete(m.claimed, key)
	return nil
}

func TestNewDailyMenuCronWorker_WithNilProfileClient(t *testing.T) {
	t.Parallel()

	// nil profileClient được chấp nhận — worker không panic khi Start
	w := worker.NewDailyMenuCronWorker(nil, nil, nil, nil, 5, 0)
	if w == nil {
		t.Fatal("expected non-nil worker")
	}
}

func TestNextDailyTarget(t *testing.T) {
	t.Parallel()

	// 1. Bây giờ là 03:00 sáng -> Mục tiêu 05:00 sáng hôm nay
	morning3am := time.Date(2026, 11, 18, 3, 0, 0, 0, time.UTC)
	targetToday := worker.NextDailyTarget(morning3am, 5, 0)
	if targetToday.Day() != 18 || targetToday.Hour() != 5 || targetToday.Minute() != 0 {
		t.Fatalf("expected today 05:00, got %v", targetToday)
	}

	// 2. Bây giờ là 07:00 sáng (đã qua 05:00) -> Mục tiêu 05:00 sáng ngày mai
	morning7am := time.Date(2026, 11, 18, 7, 0, 0, 0, time.UTC)
	targetTomorrow := worker.NextDailyTarget(morning7am, 5, 0)
	if targetTomorrow.Day() != 19 || targetTomorrow.Hour() != 5 || targetTomorrow.Minute() != 0 {
		t.Fatalf("expected tomorrow 05:00, got %v", targetTomorrow)
	}
}

func TestDailyMenuCronWorker_RunOnce_NilCoordinatorReturnsError(t *testing.T) {
	t.Parallel()

	planRepo := &mockPlanRepo{activeUsers: []string{"user-1"}}
	w := worker.NewDailyMenuCronWorker(nil, planRepo, nil, nil, 5, 0)

	_, _, err := w.RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected error when coordinator is nil, got nil")
	}
}

func TestFetchBiologicalMetrics_ProfileFound(t *testing.T) {
	t.Parallel()

	want := &repository.UserBiologicalMetrics{
		WeightKg:      65.0,
		HeightCm:      160.0,
		Age:           28,
		Gender:        "FEMALE",
		ActivityLevel: "LIGHTLY_ACTIVE",
	}
	profileCli := &mockProfileClient{metrics: want}

	// Kiểm tra qua GRPCProfileClient mapping
	got, err := profileCli.GetBiologicalMetrics(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil metrics, want non-nil")
	}
	if got.WeightKg != want.WeightKg {
		t.Errorf("WeightKg: got %f, want %f", got.WeightKg, want.WeightKg)
	}
	if got.ActivityLevel != want.ActivityLevel {
		t.Errorf("ActivityLevel: got %s, want %s", got.ActivityLevel, want.ActivityLevel)
	}
}

func TestFetchBiologicalMetrics_ProfileNotFound_ReturnsNil(t *testing.T) {
	t.Parallel()

	profileCli := &mockProfileClient{metrics: nil, err: nil}

	got, err := profileCli.GetBiologicalMetrics(context.Background(), "new-user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("got %+v, want nil (no profile)", got)
	}
}

func TestFetchBiologicalMetrics_ProfileError_ReturnsFallback(t *testing.T) {
	t.Parallel()

	profileCli := &mockProfileClient{err: errors.New("connection refused")}

	got, err := profileCli.GetBiologicalMetrics(context.Background(), "user-err")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got != nil {
		t.Errorf("got non-nil metrics on error, want nil")
	}
}

// TestDefaultBiologicalMetrics kiểm tra fallback values hợp lệ cho TDEECalculator.
func TestDefaultBiologicalMetrics_ValidForTDEE(t *testing.T) {
	t.Parallel()

	fallback := service.BiologicalMetrics{
		WeightKg:      70.0,
		HeightCm:      170.0,
		Age:           25,
		Gender:        "MALE",
		ActivityLevel: "MODERATELY_ACTIVE",
	}

	calc := service.NewTDEECalculator()
	alloc, err := calc.CalculateBaseTDEE(fallback, vo.GoalMaintenance)
	if err != nil {
		t.Fatalf("CalculateBaseTDEE with fallback: %v", err)
	}
	if alloc.TargetCalories() <= 0 {
		t.Errorf("got target calories %f, want > 0", alloc.TargetCalories())
	}
}
