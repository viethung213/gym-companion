package adk

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Created Tue 2026-11-24 21:00 UTC with Tue and Wed slots: Tuesday has passed,
// so week 1 is Tue 24 .. Mon 30 holding only Wed 25, and week 2 starts Tue 1.
func anchorSchedule() *slotSchedule {
	return newSlotSchedule([]WorkoutSlot{
		{DayOfWeek: int(time.Tuesday), StartTime: "17:30", EndTime: "19:00"},
		{DayOfWeek: int(time.Wednesday), StartTime: "17:30", EndTime: "19:00"},
	}, time.Date(2026, 11, 24, 21, 0, 0, 0, time.UTC), time.UTC)
}

func TestMapToDomainRoadmap_WeeksStartOnTheScheduleAnchor(t *testing.T) {
	c := mapperFor(t, "bench-press")
	week1 := sessionOf("2026-11-25", "bench-press")
	week2 := sessionOf("2026-12-01", "bench-press")
	plan := &GeneratedPlan{Weeks: []WeekPlan{
		{WeekNumber: 1, Phase: "ACCUMULATION", Sessions: []SessionPlan{week1}},
		{WeekNumber: 2, Phase: "OVERLOAD", Sessions: []SessionPlan{week2}},
		{WeekNumber: 3, Phase: "PEAK"},
		{WeekNumber: 4, Phase: "DELOAD"},
	}}

	rm, err := c.mapToDomainRoadmap(context.Background(), plan, nil, testPrimaries(plan), "user-1", getMapNow(),
		anchorSchedule().weekStart(1))
	if err != nil {
		t.Fatalf("mapToDomainRoadmap: %v", err)
	}
	w2 := rm.Weeks()[1].Info()
	if got := w2.StartDate.Format(time.DateOnly); got != "2026-12-01" {
		t.Fatalf("week 2 starts %s, want 2026-12-01 (the schedule's week 2, holding its Tuesday session)", got)
	}
}

func TestValidator_SessionListedUnderTheWrongWeekIsAnIssue(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press")).withSchedule(anchorSchedule())
	misplaced := sessionOf("2026-12-01", "bench-press") // week 2's Tuesday
	misplaced.SlotTime = "17:30-19:00"

	out, err := v.validate(context.Background(), planOf(misplaced), false)
	if err != nil {
		t.Fatal(err)
	}
	for _, is := range out.Issues {
		if strings.Contains(is.Reason, "is not in week 1") {
			return
		}
	}
	t.Fatalf("issues = %+v, want a wrong-week issue", out.Issues)
}
