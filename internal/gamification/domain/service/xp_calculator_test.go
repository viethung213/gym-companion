package service_test

import (
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/gamification/domain/service"
)

func float32Ptr(v float32) *float32 {
	return &v
}

func float64Ptr(v float64) *float64 {
	return &v
}

func TestCalculateWorkoutXp(t *testing.T) {
	tests := []struct {
		name    string
		give    service.WorkoutXpInput
		wantXp  int32
		wantErr error
	}{
		{
			name: "insufficient sets (< 3 sets) fails proof of workout",
			give: service.WorkoutXpInput{
				TotalSets: 2,
			},
			wantXp:  0,
			wantErr: service.ErrInsufficientWorkout,
		},
		{
			name: "minimum valid workout: base 50 + vol 10 = 60 xp",
			give: service.WorkoutXpInput{
				TotalSets:        3,
				TotalVolumeKg:    500.0,
				AverageFormScore: nil,
				HasNewPR:         false,
				CurrentStreak:    0,
			},
			wantXp:  60,
			wantErr: nil,
		},
		{
			name: "medium volume (2000kg) + good form (75% / 0.75): 50 + 20 + 15 = 85 xp",
			give: service.WorkoutXpInput{
				TotalSets:        10,
				TotalVolumeKg:    2000.0,
				AverageFormScore: float32Ptr(0.75), // 20 * 0.75 = 15 form XP
				HasNewPR:         false,
				CurrentStreak:    0,
			},
			wantXp:  85, // 50 + 20 + 15 = 85
			wantErr: nil,
		},
		{
			name: "3-day streak multiplier 1.10x: (50 + 20 + 15) * 1.10 = 94 xp",
			give: service.WorkoutXpInput{
				TotalSets:        10,
				TotalVolumeKg:    2000.0,
				AverageFormScore: float32Ptr(75.0), // 75 on 100 scale -> 15 form XP
				HasNewPR:         false,
				CurrentStreak:    3,
			},
			wantXp:  94, // 85 * 1.10 = 93.5 -> 94
			wantErr: nil,
		},
		{
			name: "heavy volume (6000kg) + excellent form (90%) + PR + 7-day streak (1.20x): (50+30+18+25)*1.20 = 148 xp",
			give: service.WorkoutXpInput{
				TotalSets:        20,
				TotalVolumeKg:    6000.0,
				AverageFormScore: float32Ptr(0.90), // 20 * 0.90 = 18 form XP
				HasNewPR:         true,
				CurrentStreak:    7,
			},
			wantXp:  148, // 123 * 1.20 = 147.6 -> 148
			wantErr: nil,
		},
		{
			name: "volume ratio custom clamping: ratio 1.2 -> 25 * 1.2 = 30 clamped",
			give: service.WorkoutXpInput{
				TotalSets:     5,
				VolumeRatio:   float64Ptr(1.2),
				CurrentStreak: 0,
			},
			wantXp:  80, // 50 + 30 = 80
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			gotXp, err := service.CalculateWorkoutXp(tt.give)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("CalculateWorkoutXp() err = %v, want %v", err, tt.wantErr)
			}
			if gotXp != tt.wantXp {
				t.Errorf("CalculateWorkoutXp() xp = %d, want %d", gotXp, tt.wantXp)
			}
		})
	}
}
