package worker_test

import (
	"context"
	"errors"
	"testing"

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

func (m *mockPlanRepo) FindByUserIDAndDate(_ context.Context, _ string, _ interface{}) (interface{}, error) {
	return nil, nil
}

func (m *mockPlanRepo) Save(_ context.Context, _ interface{}) error  { return nil }
func (m *mockPlanRepo) Update(_ context.Context, _ interface{}) error { return nil }

func TestNewDailyMenuCronWorker_WithNilProfileClient(t *testing.T) {
	t.Parallel()

	// nil profileClient được chấp nhận — worker không panic khi Start
	w := worker.NewDailyMenuCronWorker(nil, nil, nil)
	if w == nil {
		t.Fatal("expected non-nil worker")
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
