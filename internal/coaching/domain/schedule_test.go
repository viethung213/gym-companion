package domain_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

func TestGenerateWeeklySchedule(t *testing.T) {
	t.Parallel()

	now := time.Now()
	startDate := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name                 string
		experienceLevel      string
		wantTrainingDaysCount int
		wantSplitType        string
		wantErr              bool
	}{
		{
			name:                 "beginner gets 3 days full body",
			experienceLevel:      "beginner",
			wantTrainingDaysCount: 3,
			wantSplitType:        "FullBody",
			wantErr:              false,
		},
		{
			name:                 "intermediate gets 4 days upper lower",
			experienceLevel:      "intermediate",
			wantTrainingDaysCount: 4,
			wantSplitType:        "Upper/Lower",
			wantErr:              false,
		},
		{
			name:                 "advanced gets 5 days push pull legs",
			experienceLevel:      "advanced",
			wantTrainingDaysCount: 5,
			wantSplitType:        "Push/Pull/Legs",
			wantErr:              false,
		},
		{
			name:                 "invalid experience level returns error",
			experienceLevel:      "expert",
			wantTrainingDaysCount: 0,
			wantSplitType:        "",
			wantErr:              true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			schedule, err := domain.GenerateWeeklySchedule(
				"s-1", "r-1", "u-1", 1, startDate, tt.experienceLevel, now,
			)

			if (err != nil) != tt.wantErr {
				t.Fatalf("got err = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if got, want := schedule.MuscleSplitType(), tt.wantSplitType; got != want {
				t.Errorf("MuscleSplitType got = %s, want = %s", got, want)
			}

			trainingDays := schedule.TrainingDays()
			if got, want := len(trainingDays), tt.wantTrainingDaysCount; got != want {
				t.Errorf("TrainingDays count got = %d, want = %d", got, want)
			}

			// Invariant check: total days is 7, rest days >= 1 (BR-AC-01)
			allDays := schedule.ScheduleDays()
			if got, want := len(allDays), 7; got != want {
				t.Errorf("Total schedule days got = %d, want = %d", got, want)
			}
			restDays := 7 - len(trainingDays)
			if restDays < 1 {
				t.Errorf("BR-AC-01 violated: rest days count = %d, want >= 1", restDays)
			}
		})
	}
}

func TestWeeklySchedule_SetDailyPlanID(t *testing.T) {
	t.Parallel()

	now := time.Now()
	startDate := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	schedule, err := domain.GenerateWeeklySchedule("s-1", "r-1", "u-1", 1, startDate, "beginner", now)
	if err != nil {
		t.Fatalf("generate schedule failed: %v", err)
	}

	trainingDays := schedule.TrainingDays()
	if len(trainingDays) == 0 {
		t.Fatalf("expected training days")
	}

	targetDate := trainingDays[0].Date
	planID := "plan-999"

	if err := schedule.SetDailyPlanID(targetDate, planID); err != nil {
		t.Fatalf("set daily plan id failed: %v", err)
	}

	// Verify plan ID linked
	found := false
	for _, d := range schedule.ScheduleDays() {
		if d.Date.Equal(targetDate) {
			if got, want := d.DailyPlanID, planID; got != want {
				t.Errorf("DailyPlanID got = %s, want = %s", got, want)
			}
			found = true
			break
		}
	}
	if !found {
		t.Errorf("target date not found in schedule")
	}
}
