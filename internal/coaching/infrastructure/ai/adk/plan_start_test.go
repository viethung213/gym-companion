package adk

import (
	"testing"
	"time"
)

// A block planned ahead is laid out from midnight of its first day in the
// athlete's zone, so the slot schedule starts that day and skips nothing.
func TestPlanStartInstant(t *testing.T) {
	got := planStartInstant(time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC), "Asia/Ho_Chi_Minh")
	if got.Format(time.RFC3339) != "2026-11-30T00:00:00+07:00" {
		t.Errorf("got %s, want midnight of 2026-11-30 in Ho Chi Minh City", got.Format(time.RFC3339))
	}

	in := &CoachInput{
		Flow:        FlowInitiate4Week,
		CurrentTime: got.Format(time.RFC3339),
		Timezone:    "Asia/Ho_Chi_Minh",
		Profile:     UserProfile{AvailableSlots: []WorkoutSlot{{DayOfWeek: int(time.Monday), StartTime: "06:00", EndTime: "07:00"}}},
	}
	schedule, err := initSlotSchedule(in)
	if err != nil {
		t.Fatal(err)
	}
	if ws := schedule.weekStart(1).Format(time.DateOnly); ws != "2026-11-30" {
		t.Errorf("week 1 starts %s, want 2026-11-30", ws)
	}
	if n := schedule.expectedSessions(1); n != 1 {
		t.Errorf("week 1 expects %d sessions, want the Monday 06:00 slot of its first day", n)
	}
}

func TestPlanStartIsPerRun(t *testing.T) {
	c := &CoachingContextAgent{planStarts: map[string]time.Time{}}
	start := time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC)
	c.putPlanStart("run-1", start)

	if _, ok := c.takePlanStart("run-2"); ok {
		t.Error("another run saw run-1's start")
	}
	if got, ok := c.takePlanStart("run-1"); !ok || !got.Equal(start) {
		t.Errorf("run-1 start = %v, %v", got, ok)
	}
	if _, ok := c.takePlanStart("run-1"); ok {
		t.Error("start not cleared after it was taken")
	}
}
