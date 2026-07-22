package domain

import (
	"testing"
	"time"
)

func TestWeeklySchedule(t *testing.T) {
	now := time.Now()
	
	ws, err := NewWeeklySchedule("ws-1", "rm-1", "user-1", 1, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Test Adding Days
	day1 := ScheduleDay{
		ScheduledDate: now,
		Status:        DayStatusTraining,
		TargetMuscleGroups: []string{"Chest", "Triceps"},
	}
	ws.AddDay(day1)
	
	day2 := ScheduleDay{
		ScheduledDate: now.Add(24 * time.Hour),
		Status:        DayStatusTraining,
		TargetMuscleGroups: []string{"Chest"},
	}
	ws.AddDay(day2)
	
	// Test Muscle Recovery Violation
	err = ws.ValidateMuscleRecovery(48)
	if err == nil {
		t.Errorf("expected muscle recovery violation")
	}
	
	// Fix Day2 to be 48 hours away
	ws.scheduleDays[1].ScheduledDate = now.Add(48 * time.Hour)
	err = ws.ValidateMuscleRecovery(48)
	if err != nil {
		t.Errorf("unexpected error after fixing recovery: %v", err)
	}
	
	// Test SkipDay
	if err := ws.SkipDay(now, now); err != nil {
		t.Fatalf("unexpected error skipping day: %v", err)
	}
	if ws.Days()[0].Status != DayStatusSkipped {
		t.Errorf("day not skipped")
	}
	
	// Test Reschedule
	future := now.Add(72 * time.Hour)
	if err := ws.RescheduleDay(now.Add(48*time.Hour), future, now); err != nil {
		t.Fatalf("unexpected error rescheduling day: %v", err)
	}
	if ws.Days()[1].ScheduledDate != future || ws.Days()[1].Status != DayStatusRescheduled {
		t.Errorf("day not rescheduled")
	}
}
