package roadmap

import (
	"errors"
	"testing"
	"time"
)

// blockFrom is a one-session roadmap from start (a Monday) to start+28 days.
func blockFrom(t *testing.T, id string, start time.Time) *Roadmap {
	t.Helper()
	s, err := NewSessionPlan(&SessionPlanInfo{
		SessionPlanID: id + "-s", DayPlanID: id + "-d", WeekPlanID: id + "-w", RoadmapID: id,
		UserID: "u", ScheduledDate: start, SlotTime: "18:00-19:00",
	}, start)
	if err != nil {
		t.Fatal(err)
	}
	d, err := RehydrateDayPlan(&DayPlanInfo{DayPlanID: id + "-d", WeekPlanID: id + "-w", RoadmapID: id, UserID: "u", ScheduledDate: start}, []*SessionPlan{s})
	if err != nil {
		t.Fatal(err)
	}
	w, err := RehydrateWeekPlan(&WeekPlanInfo{
		WeekPlanID: id + "-w", RoadmapID: id, UserID: "u", WeekNumber: 1, Phase: PhaseAccumulation,
		StartDate: start, EndDate: start.AddDate(0, 0, 7),
	}, []*DayPlan{d})
	if err != nil {
		t.Fatal(err)
	}
	rm, err := NewRoadmap(&Info{RoadmapID: id, UserID: "u", StartDate: start, EndDate: start.AddDate(0, 0, 28)}, []*WeekPlan{w}, start)
	if err != nil {
		t.Fatal(err)
	}
	rm.SetTimezone("Asia/Ho_Chi_Minh")
	return rm
}

func TestRoadmap_BlockBoundariesUseTheAthletesZone(t *testing.T) {
	rm := blockFrom(t, "rm", time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC))
	hcm := rm.Location()

	if want := time.Date(2026, 11, 30, 0, 0, 0, 0, hcm); !rm.EndsAt().Equal(want) {
		t.Errorf("EndsAt = %v, want %v", rm.EndsAt(), want)
	}
	if want := time.Date(2026, 11, 23, 0, 0, 0, 0, hcm); !rm.FinalWeekStartsAt().Equal(want) {
		t.Errorf("FinalWeekStartsAt = %v, want %v", rm.FinalWeekStartsAt(), want)
	}
}

func TestRoadmap_PlannedLifecycle(t *testing.T) {
	now := time.Date(2026, 11, 23, 9, 0, 0, 0, time.UTC)
	current := blockFrom(t, "a", time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC))
	next := blockFrom(t, "b", time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC))
	unrelated := blockFrom(t, "c", time.Date(2026, 12, 7, 0, 0, 0, 0, time.UTC))

	if !next.Follows(current) || unrelated.Follows(current) {
		t.Fatalf("Follows: next %v, unrelated %v; want true, false", next.Follows(current), unrelated.Follows(current))
	}

	if err := next.HoldUntilStart(); err != nil || next.Status() != StatusPlanned {
		t.Fatalf("HoldUntilStart: err %v, status %s", err, next.Status())
	}
	if err := next.Activate(now); err != nil || next.Status() != StatusActive {
		t.Fatalf("Activate: err %v, status %s", err, next.Status())
	}
	if err := next.Activate(now); err != nil {
		t.Errorf("Activate is not idempotent: %v", err)
	}

	if err := current.MarkCompleted(now); err != nil {
		t.Fatal(err)
	}
	if err := current.Activate(now); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("activating a COMPLETED roadmap: err %v, want ErrInvalidTransition", err)
	}
}

// Only an untouched draft may be planned ahead.
func TestRoadmap_HoldUntilStartNeedsAFreshDraft(t *testing.T) {
	start := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	rm := blockFrom(t, "a", start)
	s, _ := rm.FindSession("a-s")
	if err := s.MarkCompleted(100, 0, start); err != nil {
		t.Fatal(err)
	}
	if err := rm.HoldUntilStart(); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("err %v, want ErrInvalidTransition for a roadmap already trained", err)
	}
}

// A draft is dated by when planning started; a saved roadmap keeps its date.
func TestRoadmap_StampPlannedAt(t *testing.T) {
	start := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	rm := blockFrom(t, "a", start)
	planned := start.Add(-2 * time.Minute)

	if err := rm.StampPlannedAt(planned); err != nil || !rm.Info().CreatedAt.Equal(planned) {
		t.Fatalf("stamp fresh draft: err %v, created %v", err, rm.Info().CreatedAt)
	}
	rm.MarkSaved(1)
	if err := rm.StampPlannedAt(start); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("stamping a saved roadmap: err %v, want ErrInvalidTransition", err)
	}
}
