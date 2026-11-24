package adk

import (
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// rescheduleRoadmap is an ICT roadmap from Mon 2026-11-23 holding the given
// PENDING sessions, keyed "YYYY-MM-DD HH:MM-HH:MM", plus one completed Monday.
func rescheduleRoadmap(t *testing.T, pending ...string) *roadmap.Roadmap {
	t.Helper()
	start := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	weeks := make([]*roadmap.WeekPlan, 0, roadmap.WeeksPerRoadmap)
	for n := range roadmap.WeeksPerRoadmap {
		ws := start.AddDate(0, 0, n*7)
		w, err := roadmap.NewWeekPlan(&roadmap.WeekPlanInfo{
			WeekPlanID: "w" + string(rune('1'+n)), RoadmapID: "rm", UserID: "u", WeekNumber: int32(n + 1),
			Phase: roadmap.PhaseAccumulation, TargetRPE: 7, StartDate: ws, EndDate: ws.AddDate(0, 0, 7),
		})
		if err != nil {
			t.Fatal(err)
		}
		weeks = append(weeks, w)
	}
	add := func(key string, status roadmap.SessionPlanStatus) {
		d, err := time.Parse(time.DateOnly, key[:10])
		if err != nil {
			t.Fatal(err)
		}
		w := weeks[int(d.Sub(start).Hours()/24)/7]
		dp, err := roadmap.NewDayPlan(&roadmap.DayPlanInfo{DayPlanID: "d" + key, WeekPlanID: w.ID(), RoadmapID: "rm", UserID: "u", ScheduledDate: d})
		if err != nil {
			t.Fatal(err)
		}
		s, err := roadmap.RehydrateSessionPlan(&roadmap.SessionPlanInfo{
			SessionPlanID: "s" + key, DayPlanID: dp.ID(), WeekPlanID: w.ID(), RoadmapID: "rm", UserID: "u",
			ScheduledDate: d, SlotTime: key[11:], Status: status, Source: roadmap.SessionPlanSourceScheduled,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := dp.AddSession(s); err != nil {
			t.Fatal(err)
		}
		if err := w.AddDay(dp); err != nil {
			t.Fatal(err)
		}
	}
	add("2026-11-23 17:30-19:00", roadmap.SessionPlanStatusCompleted)
	for _, k := range pending {
		add(k, roadmap.SessionPlanStatusPending)
	}
	rm, err := roadmap.RehydrateRoadmap(&roadmap.Info{
		RoadmapID: "rm", UserID: "u", Status: roadmap.StatusActive, Timezone: "Asia/Ho_Chi_Minh",
		StartDate: start, EndDate: start.AddDate(0, 0, 28),
	}, weeks)
	if err != nil {
		t.Fatal(err)
	}
	return rm
}

// rescheduleNow is Tue 2026-11-24 09:00 ICT.
func rescheduleNow() time.Time { return time.Date(2026, 11, 24, 2, 0, 0, 0, time.UTC) }

func mondayThursday() []WorkoutSlot {
	return []WorkoutSlot{
		{DayOfWeek: int(time.Monday), StartTime: "17:30", EndTime: "19:00"},
		{DayOfWeek: int(time.Thursday), StartTime: "17:30", EndTime: "19:00"},
	}
}

func remainingMonThu() []string {
	return []string{
		"2026-11-26 17:30-19:00",
		"2026-11-30 17:30-19:00", "2026-12-03 17:30-19:00",
		"2026-12-07 17:30-19:00", "2026-12-10 17:30-19:00",
		"2026-12-14 17:30-19:00", "2026-12-17 17:30-19:00",
	}
}

func TestRescheduleTargets_NilWhenSessionsAlreadyOnSlots(t *testing.T) {
	rm := rescheduleRoadmap(t, remainingMonThu()...)
	if got := rescheduleTargets(rm, mondayThursday(), rescheduleNow()); got != nil {
		t.Fatalf("got %d targets, want nil: sessions already sit on the selected slots", len(got))
	}
}

func TestRescheduleTargets_MovesRemainingOntoNewSlots(t *testing.T) {
	rm := rescheduleRoadmap(t, remainingMonThu()...)
	// The athlete switched to Tuesday 06:00 and Friday 18:00.
	slots := []WorkoutSlot{
		{DayOfWeek: int(time.Tuesday), StartTime: "06:00", EndTime: "07:00"},
		{DayOfWeek: int(time.Friday), StartTime: "18:00", EndTime: "19:30"},
	}
	got := rescheduleTargets(rm, slots, rescheduleNow())

	// Tue 24 06:00 ICT has already started at 09:00, so week 1 keeps only Friday.
	want := []string{
		"2026-11-27 18:00-19:30",
		"2026-12-01 06:00-07:00", "2026-12-04 18:00-19:30",
		"2026-12-08 06:00-07:00", "2026-12-11 18:00-19:30",
		"2026-12-15 06:00-07:00", "2026-12-18 18:00-19:30",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d targets %+v, want %d", len(got), got, len(want))
	}
	for i, w := range want {
		if g := got[i].ScheduledDate + " " + got[i].SlotTime; g != w {
			t.Errorf("target %d = %s, want %s", i, g, w)
		}
	}

	sessions := rescheduledSessions(got)
	if sessions[0].EstimatedDurationMinutes != 90 || sessions[1].EstimatedDurationMinutes != 60 {
		t.Errorf("durations = %d, %d; want 90 and 60 from the slot lengths",
			sessions[0].EstimatedDurationMinutes, sessions[1].EstimatedDurationMinutes)
	}
}

func TestRescheduleTargets_RespectsWeeklyCaps(t *testing.T) {
	rm := rescheduleRoadmap(t, "2026-11-26 17:30-19:00")
	daily := make([]WorkoutSlot, 0, 7)
	for d := range 7 {
		daily = append(daily, WorkoutSlot{DayOfWeek: d, StartTime: "20:00", EndTime: "21:00"})
	}
	got := rescheduleTargets(rm, daily, rescheduleNow())

	type count struct{ all, training int }
	perWeek := map[int]*count{}
	start := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	for _, ts := range got {
		d, _ := time.Parse(time.DateOnly, ts.ScheduledDate)
		w := int(d.Sub(start).Hours()/24) / 7
		if perWeek[w] == nil {
			perWeek[w] = &count{}
		}
		perWeek[w].all++
		if ts.SessionKind == kindTraining {
			perWeek[w].training++
		}
	}
	// Week 1: Tue 24 .. Sun 29 = 6 open slots; the completed Monday leaves room
	// for 5 more training sessions, so the sixth slot becomes recovery.
	if c := perWeek[0]; c == nil || c.all != 6 || c.training != roadmap.MaxSessionsPerWeek-1 {
		t.Errorf("week 1 = %+v, want 6 sessions / %d training", c, roadmap.MaxSessionsPerWeek-1)
	}
	for w := 1; w < roadmap.WeeksPerRoadmap; w++ {
		if c := perWeek[w]; c == nil || c.all != 7 || c.training != roadmap.MaxSessionsPerWeek {
			t.Errorf("week %d = %+v, want 7 sessions / %d training", w+1, c, roadmap.MaxSessionsPerWeek)
		}
	}
}

func TestPinRewriteSlots(t *testing.T) {
	plan := &GeneratedPlan{Weeks: []WeekPlan{{Sessions: []SessionPlan{
		{ScheduledDate: "2030-01-01", SlotTime: "99:99"},
		{ScheduledDate: "bad", SlotTime: "", EstimatedDurationMinutes: 30},
	}}}}
	pinRewriteSlots(plan, []SessionToRevise{
		{ScheduledDate: "2026-11-27", SlotTime: "18:00-19:30", EstimatedDurationMinutes: 90},
		{ScheduledDate: "2026-12-01", SlotTime: "06:00-07:00", EstimatedDurationMinutes: 60},
	})
	s := plan.Weeks[0].Sessions
	if s[0].ScheduledDate != "2026-11-27" || s[0].SlotTime != "18:00-19:30" || s[0].EstimatedDurationMinutes != 90 {
		t.Errorf("session 0 not pinned: %+v", s[0])
	}
	if s[1].ScheduledDate != "2026-12-01" || s[1].EstimatedDurationMinutes != 30 {
		t.Errorf("session 1 should be pinned but keep its own duration: %+v", s[1])
	}
}
