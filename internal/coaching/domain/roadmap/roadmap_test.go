package roadmap

import (
	"errors"
	"testing"
	"time"
)

func newTestRoadmapInfo() Info {
	return Info{
		RoadmapID: "rm-1",
		UserID:    "user-1",
		StartDate: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC),
	}
}

func newDay(id string, date time.Time, weekPlanID string, sessionCount int) *DayPlan {
	dp, _ := NewDayPlan(&DayPlanInfo{
		DayPlanID:     id,
		WeekPlanID:    weekPlanID,
		RoadmapID:     "rm-1",
		UserID:        "user-1",
		ScheduledDate: date,
	})

	now := time.Now()

	for i := 0; i < sessionCount; i++ {
		sp, _ := NewSessionPlan(&SessionPlanInfo{
			SessionPlanID: id + "-s" + string(rune('0'+i)),
			DayPlanID:     id,
			WeekPlanID:    weekPlanID,
			RoadmapID:     "rm-1",
			UserID:        "user-1",
			ScheduledDate: date,
		}, now)

		_ = dp.AddSession(sp)
	}

	return dp
}

func newWeek(id string, num int32, phase Phase, weekStart time.Time, sessionsPerDay []int) *WeekPlan {
	wp, _ := NewWeekPlan(&WeekPlanInfo{
		WeekPlanID: id,
		RoadmapID:  "rm-1",
		UserID:     "user-1",
		WeekNumber: num,
		Phase:      phase,
		TargetRPE:  7.0,
		StartDate:  weekStart,
		EndDate:    weekStart.AddDate(0, 0, 6),
	})

	for i, n := range sessionsPerDay {
		day := newDay(id+"-d"+string(rune('0'+i)), weekStart.AddDate(0, 0, i), id, n)

		_ = wp.AddDay(day)
	}

	return wp
}

func TestNewRoadmap_DefaultsToActive(t *testing.T) {
	now := time.Now()

	info := newTestRoadmapInfo()

	r, err := NewRoadmap(&info, nil, now)

	if err != nil {
		t.Fatalf("NewRoadmap: %v", err)
	}

	if r.Status() != StatusActive {
		t.Errorf("status=%s, want ACTIVE", r.Status())
	}
}

func TestNewRoadmap_NilChecks(t *testing.T) {
	if _, err := NewRoadmap(nil, nil, time.Now()); err == nil {
		t.Errorf("expected error for nil Info")
	}

	if _, err := RehydrateRoadmap(nil, nil); err == nil {
		t.Errorf("expected error for nil Info")
	}
}

func TestNewRoadmap_ValidatesFields(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*Info)
	}{
		{"missing roadmap_id", func(i *Info) { i.RoadmapID = "" }},
		{"missing user_id", func(i *Info) { i.UserID = "" }},
		{"missing start_date", func(i *Info) { i.StartDate = time.Time{} }},
		{"end before start", func(i *Info) { i.EndDate = i.StartDate.AddDate(0, 0, -1) }},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			info := newTestRoadmapInfo()

			tt.mut(&info)

			_, err := NewRoadmap(&info, nil, time.Now())

			if !errors.Is(err, ErrInvalidRoadmap) {
				t.Errorf("expected ErrInvalidRoadmap, got %v", err)
			}
		})
	}
}

func TestRoadmap_WeeklyCap_BR_AC_01(t *testing.T) {
	// Try to add 7 sessions in one week (>6 = cap).

	wp, err := NewWeekPlan(&WeekPlanInfo{
		WeekPlanID: "wp-1",
		RoadmapID:  "rm-1",
		UserID:     "user-1",
		WeekNumber: 1,
		Phase:      PhaseAccumulation,
		TargetRPE:  7.0,
		StartDate:  time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		EndDate:    time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC),
	})

	if err != nil {
		t.Fatalf("NewWeekPlan: %v", err)
	}

	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	// Add 6 days each with 1 session — should succeed

	for i := 0; i < 6; i++ {
		day := newDay("wp-1-d"+string(rune('0'+i)), base.AddDate(0, 0, i), "wp-1", 1)

		if err := wp.AddDay(day); err != nil {
			t.Fatalf("AddDay %d: %v", i, err)
		}
	}

	// 7th day with a session should exceed the cap

	day := newDay("wp-1-d6", base.AddDate(0, 0, 6), "wp-1", 1)

	err = wp.AddDay(day)

	if !errors.Is(err, ErrWeeklyCapExceeded) {
		t.Errorf("expected ErrWeeklyCapExceeded, got %v", err)
	}
}

func TestRoadmap_MarkCompleted(t *testing.T) {
	info := newTestRoadmapInfo()

	r, _ := NewRoadmap(&info, nil, time.Now())

	now := time.Now().Add(28 * 24 * time.Hour)

	if err := r.MarkCompleted(now); err != nil {
		t.Fatalf("MarkCompleted: %v", err)
	}

	if r.Status() != StatusCompleted {
		t.Errorf("status=%s", r.Status())
	}

	// Idempotent

	if err := r.MarkCompleted(now.Add(time.Hour)); err != nil {
		t.Errorf("expected no-op, got %v", err)
	}
}

func TestRoadmap_PendingSessionsFrom(t *testing.T) {
	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	wp1 := newWeek("wp-1", 1, PhaseAccumulation, base, []int{1, 0, 1, 0, 1, 0, 0})

	info := newTestRoadmapInfo()

	r, err := NewRoadmap(&info, []*WeekPlan{wp1}, time.Now())

	if err != nil {
		t.Fatalf("NewRoadmap: %v", err)
	}

	// From the beginning: 3 PENDING sessions

	pending := r.PendingSessionsFrom(base)

	if len(pending) != 3 {
		t.Errorf("expected 3 pending, got %d", len(pending))
	}

	// Mark first as completed

	_ = pending[0].MarkCompleted(80, 0, time.Now())

	pending2 := r.PendingSessionsFrom(base)

	if len(pending2) != 2 {
		t.Errorf("expected 2 pending after complete, got %d", len(pending2))
	}

	// From day 3: skip earlier PENDING → only 2 sessions from day 3 onward (day 3 + day 5)

	from := base.AddDate(0, 0, 2)

	pending3 := r.PendingSessionsFrom(from)

	if len(pending3) != 2 {
		t.Errorf("expected 2 pending from day 3, got %d", len(pending3))
	}
}

func TestRoadmap_FindSession(t *testing.T) {
	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	wp1 := newWeek("wp-1", 1, PhaseAccumulation, base, []int{1, 0, 0, 0, 0, 0, 0})

	info := newTestRoadmapInfo()

	r, _ := NewRoadmap(&info, []*WeekPlan{wp1}, time.Now())

	if _, ok := r.FindSession("wp-1-d0-s0"); !ok {
		t.Errorf("expected to find session wp-1-d0-s0")
	}

	if _, ok := r.FindSession("no-such-id"); ok {
		t.Errorf("expected not to find bogus id")
	}
}

func TestRoadmap_ValidateFullStructure_Requires4Weeks(t *testing.T) {
	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	wp1 := newWeek("wp-1", 1, PhaseAccumulation, base, []int{0, 0, 0, 0, 0, 0, 0})

	info := newTestRoadmapInfo()

	r, _ := NewRoadmap(&info, []*WeekPlan{wp1}, time.Now())

	if err := r.ValidateFullStructure(); !errors.Is(err, ErrInvalidWeekCount) {
		t.Errorf("expected ErrInvalidWeekCount for 1 week, got %v", err)
	}

	weeks := []*WeekPlan{
		newWeek("wp-1", 1, PhaseAccumulation, base, []int{0, 0, 0, 0, 0, 0, 0}),
		newWeek("wp-2", 2, PhaseOverload, base.AddDate(0, 0, 7), []int{0, 0, 0, 0, 0, 0, 0}),
		newWeek("wp-3", 3, PhasePeak, base.AddDate(0, 0, 14), []int{0, 0, 0, 0, 0, 0, 0}),
		newWeek("wp-4", 4, PhaseDeload, base.AddDate(0, 0, 21), []int{0, 0, 0, 0, 0, 0, 0}),
	}

	info2 := newTestRoadmapInfo()

	r2, _ := NewRoadmap(&info2, weeks, time.Now())

	if err := r2.ValidateFullStructure(); err != nil {
		t.Errorf("expected no error for 4 weeks, got %v", err)
	}
}
