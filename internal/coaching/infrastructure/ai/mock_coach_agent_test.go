package ai

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

type fixedClock struct{ t time.Time }

func (c *fixedClock) Now() time.Time { return c.t }

type counterIDs struct{ n int }

func (c *counterIDs) NewID() string {
	c.n++
	return "id-" + strconvItoa(c.n)
}

func strconvItoa(n int) string { return strconv.Itoa(n) }

func TestMockCoachAgent_GenerateRoadmap_4Weeks(t *testing.T) {
	clock := &fixedClock{t: time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)} // Tuesday
	agent := NewMockCoachAgent(&counterIDs{}, clock)

	cc := port.CoachContext{
		Flow:   port.FlowInitiate4Week,
		UserID: "user-alpha",
		Profile: port.Profile{
			UserID:                "user-alpha",
			PrimaryGoal:           "MUSCLE_GAIN",
			PreferredMuscleGroups: []string{"chest", "back"},
			AvailableSlots: []port.Slot{
				{DayOfWeek: time.Monday, StartTime: "06:00", EndTime: "07:00"},
				{DayOfWeek: time.Wednesday, StartTime: "06:00", EndTime: "07:00"},
				{DayOfWeek: time.Friday, StartTime: "06:00", EndTime: "07:00"},
				{DayOfWeek: time.Saturday, StartTime: "09:00", EndTime: "10:00"},
			},
		},
	}

	r, err := agent.GenerateRoadmap(context.Background(), cc, nil)
	if err != nil {
		t.Fatalf("GenerateRoadmap: %v", err)
	}
	if err := r.ValidateFullStructure(); err != nil {
		t.Fatalf("ValidateFullStructure: %v", err)
	}
	if len(r.Weeks()) != roadmap.WeeksPerRoadmap {
		t.Fatalf("want %d weeks, got %d", roadmap.WeeksPerRoadmap, len(r.Weeks()))
	}
	// Phase progression must match spec.
	wants := []roadmap.Phase{roadmap.PhaseAccumulation, roadmap.PhaseOverload, roadmap.PhasePeak, roadmap.PhaseDeload}
	for i, w := range r.Weeks() {
		if w.Phase() != wants[i] {
			t.Errorf("week %d phase=%s, want %s", i+1, w.Phase(), wants[i])
		}
		if w.TotalSessions() > roadmap.MaxSessionsPerWeek {
			t.Errorf("week %d has %d sessions (cap %d)", i+1, w.TotalSessions(), roadmap.MaxSessionsPerWeek)
		}
	}
}

func TestMockCoachAgent_GenerateRoadmap_Deterministic(t *testing.T) {
	clock := &fixedClock{t: time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)}
	cc := port.CoachContext{
		UserID: "user-beta",
		Profile: port.Profile{
			UserID:                "user-beta",
			PreferredMuscleGroups: []string{"chest"},
		},
	}

	a1 := NewMockCoachAgent(&counterIDs{}, clock)
	r1, err := a1.GenerateRoadmap(context.Background(), cc, nil)
	if err != nil {
		t.Fatalf("gen1: %v", err)
	}
	a2 := NewMockCoachAgent(&counterIDs{}, clock)
	r2, err := a2.GenerateRoadmap(context.Background(), cc, nil)
	if err != nil {
		t.Fatalf("gen2: %v", err)
	}
	// Same phase pattern and same rest-day pattern (structural determinism).
	for i, w := range r1.Weeks() {
		if w.Phase() != r2.Weeks()[i].Phase() {
			t.Errorf("week %d phase differs", i)
		}
		if w.TotalSessions() != r2.Weeks()[i].TotalSessions() {
			t.Errorf("week %d session count differs (%d vs %d)", i, w.TotalSessions(), r2.Weeks()[i].TotalSessions())
		}
	}
}
