package persistence

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

func TestSessionPlanRecord_MappingRoundTrip(t *testing.T) {
	now := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	info := roadmap.SessionPlanInfo{
		SessionPlanID:            "sp-solo-1",
		DayPlanID:                "dp-1",
		WeekPlanID:               "wp-1",
		RoadmapID:                "rm-1",
		UserID:                   "user-1",
		ScheduledDate:            time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		SlotTime:                 "08:00-09:00",
		EstimatedDurationMinutes: 45,
		Status:                   roadmap.SessionPlanStatusPending,
		Source:                   roadmap.SessionPlanSourceSoloMatch,
		TargetMuscleGroups:       []string{"Chest", "Triceps"},
		Reasoning:                "Solo match vs player 2",
		GeneratedAt:              now,
	}

	sp, err := roadmap.RehydrateSessionPlan(&info)
	if err != nil {
		t.Fatalf("unexpected rehydrate error: %v", err)
	}

	rec, err := toSessionPlanRecord(sp)
	if err != nil {
		t.Fatalf("toSessionPlanRecord failed: %v", err)
	}

	if got, want := rec.Source, "SOLO_MATCH"; got != want {
		t.Errorf("record Source = %q, want %q", got, want)
	}

	restored, err := fromSessionPlanRecord(&rec)
	if err != nil {
		t.Fatalf("fromSessionPlanRecord failed: %v", err)
	}

	if got, want := restored.Info().Source, roadmap.SessionPlanSourceSoloMatch; got != want {
		t.Errorf("restored Source = %q, want %q", got, want)
	}
}
