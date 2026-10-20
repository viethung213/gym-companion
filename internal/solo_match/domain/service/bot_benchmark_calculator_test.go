package service_test

import (
	"testing"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/service"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

func TestBotBenchmarkCalculator(t *testing.T) {
	calc := service.NewBotBenchmarkCalculator()

	prVal := 35.0
	tests := []struct {
		name   string
		cType  valueobject.ChallengeType
		userPR *float64
		want   float64
	}{
		{
			name:   "has PR reps",
			cType:  valueobject.ChallengeTypeReps,
			userPR: &prVal,
			want:   35.0,
		},
		{
			name:   "no PR reps fallback",
			cType:  valueobject.ChallengeTypeReps,
			userPR: nil,
			want:   5.0,
		},
		{
			name:   "no PR duration fallback",
			cType:  valueobject.ChallengeTypeDuration,
			userPR: nil,
			want:   300.0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := calc.CalculateBotScore(tt.cType, tt.userPR)
			if got != tt.want {
				t.Errorf("CalculateBotScore() got %v, want %v", got, tt.want)
			}
		})
	}
}
