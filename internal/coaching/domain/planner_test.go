package domain

import (
	"testing"
	"time"
)

func TestSchedulePlanner_PlanWeek(t *testing.T) {
	tests := []struct {
		name string
		give PlanningInput
		want int
	}{
		{
			name: "creates requested training days with a rest day",
			give: planningInputForTest(),
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			planner := SchedulePlanner{}
			days, err := planner.PlanWeek(tt.give, 1)
			if err != nil {
				t.Fatalf("PlanWeek() error = %v", err)
			}
			if got := len(days); got != 7 {
				t.Fatalf("len(days) = %d, want 7", got)
			}

			var trainingDays int
			for _, day := range days {
				if day.Status == ScheduleDayStatusTraining {
					trainingDays++
				}
			}
			if got := trainingDays; got != tt.want {
				t.Fatalf("training days = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPlannedVolumeValidator_Validate(t *testing.T) {
	tests := []struct {
		name         string
		givePrevious int
		giveNext     int
		want         bool
	}{
		{name: "accepts ten percent increase", givePrevious: 100, giveNext: 110, want: true},
		{name: "rejects increase above ten percent", givePrevious: 100, giveNext: 111, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := PlannedVolumeValidator{}
			if got := validator.Validate(tt.givePrevious, tt.giveNext); got != tt.want {
				t.Fatalf("Validate() = %t, want %t", got, tt.want)
			}
		})
	}
}

func planningInputForTest() PlanningInput {
	return PlanningInput{
		Goal:                PlanningGoalMuscleGain,
		ExperienceLevel:     ExperienceLevelBeginner,
		TrainingDaysPerWeek: 3,
		PreferredWeekdays:   []time.Weekday{time.Monday, time.Wednesday, time.Friday},
		MaxSessionMinutes:   45,
		EquipmentIDs:        []string{"dumbbell"},
		Timezone:            "Asia/Ho_Chi_Minh",
		StartDate:           time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC),
	}
}
