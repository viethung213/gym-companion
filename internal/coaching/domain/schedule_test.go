package domain_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

func TestNewWeeklySchedule(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	startDate := now
	endDate := now.AddDate(0, 0, 7)

	createValidDays := func() []domain.ScheduleDay {
		days := make([]domain.ScheduleDay, 7)
		daysOfWeek := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
		for i := 0; i < 7; i++ {
			status := domain.WorkoutDayStatusTraining
			if i == 6 {
				status = domain.WorkoutDayStatusRest // 1 Rest day on Sunday
			}
			days[i] = domain.NewScheduleDay(startDate.AddDate(0, 0, i), daysOfWeek[i], status, []string{"Chest", "Triceps"}, "")
		}
		return days
	}

	t.Run("successful creation with 1 rest day", func(t *testing.T) {
		t.Parallel()
		days := createValidDays()
		ws, err := domain.NewWeeklySchedule("ws_1", "rdp_1", "usr_100", 1, startDate, endDate, "Push/Pull/Legs", days)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ws.WeekNumber() != 1 {
			t.Errorf("expected week number 1, got %d", ws.WeekNumber())
		}
	})

	t.Run("violation of BR-AC-01 (7 training days, 0 rest days)", func(t *testing.T) {
		t.Parallel()
		days := createValidDays()
		days[6] = domain.NewScheduleDay(startDate.AddDate(0, 0, 6), "Sunday", domain.WorkoutDayStatusTraining, []string{"Legs"}, "")
		_, err := domain.NewWeeklySchedule("ws_1", "rdp_1", "usr_100", 1, startDate, endDate, "Push/Pull/Legs", days)
		if err != domain.ErrViolationBRAC01 {
			t.Errorf("expected ErrViolationBRAC01, got %v", err)
		}
	})

	t.Run("invalid week number", func(t *testing.T) {
		t.Parallel()
		days := createValidDays()
		_, err := domain.NewWeeklySchedule("ws_1", "rdp_1", "usr_100", 5, startDate, endDate, "Push/Pull/Legs", days)
		if err != domain.ErrInvalidWeekNumber {
			t.Errorf("expected ErrInvalidWeekNumber, got %v", err)
		}
	})

	t.Run("mark day skipped per BR-AC-03 decision 1.1", func(t *testing.T) {
		t.Parallel()
		days := createValidDays()
		ws, _ := domain.NewWeeklySchedule("ws_1", "rdp_1", "usr_100", 1, startDate, endDate, "Push/Pull/Legs", days)
		targetDate := startDate.AddDate(0, 0, 0)
		ok := ws.MarkDaySkipped(targetDate)
		if !ok {
			t.Fatalf("expected MarkDaySkipped to succeed")
		}
		if ws.ScheduleDays()[0].Status() != domain.WorkoutDayStatusSkipped {
			t.Errorf("expected status WorkoutDayStatusSkipped, got %v", ws.ScheduleDays()[0].Status())
		}
	})
}
