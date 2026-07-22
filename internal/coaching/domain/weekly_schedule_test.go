package domain

import (
	"testing"
	"time"
)

func TestWeeklySchedule(t *testing.T) {
	now := time.Now()
	
	day1 := NewScheduleDay(now, 1, DayStatusTraining, []string{"Chest", "Triceps"}, "plan-1", "MORNING", 60)
	day2 := NewScheduleDay(now.Add(24*time.Hour), 2, DayStatusTraining, []string{"Chest"}, "plan-2", "MORNING", 60)

	ws, err := NewWeeklySchedule("ws-1", "rm-1", "user-1", 1, now, []ScheduleDay{day1, day2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test Muscle Recovery Violation
	err = ws.ValidateMuscleRecovery(48)
	if err == nil {
		t.Errorf("expected muscle recovery violation")
	}

	// Reschedule Day2 to be 48 hours away
	err = ws.RescheduleDay(now.Add(24*time.Hour), now.Add(48*time.Hour), now)
	if err != nil {
		t.Fatalf("unexpected error rescheduling day: %v", err)
	}

	err = ws.ValidateMuscleRecovery(48)
	if err != nil {
		t.Errorf("unexpected error after fixing recovery: %v", err)
	}

	// Test SkipDay
	if err := ws.SkipDay(now, now); err != nil {
		t.Fatalf("unexpected error skipping day: %v", err)
	}
	if ws.Days()[0].Status() != DayStatusSkipped {
		t.Errorf("day not skipped")
	}

	// Test Reschedule
	future := now.Add(72 * time.Hour)
	if err := ws.RescheduleDay(now.Add(48*time.Hour), future, now); err != nil {
		t.Fatalf("unexpected error rescheduling day: %v", err)
	}
	if !ws.Days()[1].ScheduledDate().Equal(future) || ws.Days()[1].Status() != DayStatusRescheduled {
		t.Errorf("day not rescheduled")
	}
}

