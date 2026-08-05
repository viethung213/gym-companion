package service_test

import (
	"testing"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

func TestTDEECalculator_CalculateBaseTDEE(t *testing.T) {
	t.Parallel()

	calc := service.NewTDEECalculator()

	tests := []struct {
		name    string
		metrics service.BiologicalMetrics
		wantCal float64
	}{
		{
			name: "Male Moderately Active",
			metrics: service.BiologicalMetrics{
				WeightKg:      70.0,
				HeightCm:      170.0,
				Age:           25,
				Gender:        "MALE",
				ActivityLevel: "MODERATELY_ACTIVE",
			},
			// BMR = (10*70) + (6.25*170) - (5*25) + 5 = 700 + 1062.5 - 125 + 5 = 1642.5
			// TDEE = 1642.5 * 1.55 = 2545.875
			wantCal: 2545.875,
		},
		{
			name: "Female Lightly Active",
			metrics: service.BiologicalMetrics{
				WeightKg:      50.0,
				HeightCm:      160.0,
				Age:           30,
				Gender:        "FEMALE",
				ActivityLevel: "LIGHTLY_ACTIVE",
			},
			// BMR = (10*50) + (6.25*160) - (5*30) - 161 = 500 + 1000 - 150 - 161 = 1189
			// TDEE = 1189 * 1.375 = 1634.875
			wantCal: 1634.875,
		},
		{
			name: "Low Weight Minimum Safety Enforced (BR-NU-01)",
			metrics: service.BiologicalMetrics{
				WeightKg:      30.0,
				HeightCm:      140.0,
				Age:           20,
				Gender:        "FEMALE",
				ActivityLevel: "SEDENTARY",
			},
			// Lower than 1200 -> minimum 1200
			wantCal: 1200.0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			alloc, err := calc.CalculateBaseTDEE(tt.metrics, vo.GoalMaintenance)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if diff := alloc.TargetCalories() - tt.wantCal; diff > 0.01 || diff < -0.01 {
				t.Fatalf("got TDEE calories %f, want %f", alloc.TargetCalories(), tt.wantCal)
			}
		})
	}
}
