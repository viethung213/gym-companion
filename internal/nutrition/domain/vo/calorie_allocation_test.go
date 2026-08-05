package vo_test

import (
	"testing"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

func TestNewCalorieAllocation_BR_NU_01_Invariant(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		targetCalories float64
		wantErr        bool
	}{
		{
			name:           "Valid target calories >= 1200 kcal",
			targetCalories: 2000.0,
			wantErr:        false,
		},
		{
			name:           "Minimum valid target calories 1200 kcal",
			targetCalories: 1200.0,
			wantErr:        false,
		},
		{
			name:           "Invalid target calories < 1200 kcal",
			targetCalories: 1199.0,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			alloc, err := vo.NewCalorieAllocation(tt.targetCalories, 150.0, 200.0, 50.0)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewCalorieAllocation() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && alloc.TargetCalories() != tt.targetCalories {
				t.Fatalf("got target calories %.2f, want %.2f", alloc.TargetCalories(), tt.targetCalories)
			}
		})
	}
}

func TestCalorieAllocation_RebalanceWithSurplus(t *testing.T) {
	t.Parallel()

	alloc, err := vo.NewCalorieAllocation(2000.0, 150.0, 200.0, 50.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rebalanced := alloc.RebalanceWithSurplus(300.0)
	if rebalanced.TargetCalories() != 2300.0 {
		t.Fatalf("got rebalanced calories %.2f, want 2300.0", rebalanced.TargetCalories())
	}
}
