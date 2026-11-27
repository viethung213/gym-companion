package guardrail

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// The hard gate refuses numbers no session may carry, whatever wrote them.
func TestGuardrail_PrescriptionLimits(t *testing.T) {
	tests := []struct {
		name string
		give [4]int32 // sets per week
		want string
	}{
		{name: "sane volume", give: validSets()},
		// Incomplete, not unsafe: a stored line like this must not lock rewrites.
		{name: "zero sets is not a safety breach", give: [4]int32{3, 3, 3, 0}},
		{name: "thirty sets", give: [4]int32{30, 3, 3, 2}, want: "BR-AC-13"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewEngine(nil, nil, nil).Check(context.Background(), buildRoadmap(t, validRPE(), tt.give), nil, time.Time{})
			assertViolation(t, got, tt.want)
		})
	}
}

func TestLimitIssue(t *testing.T) {
	ok := roadmap.PrescribedExercise{TargetSets: 3, TargetReps: 8, TargetWeight: 60, TargetRPE: 7, RestSetSec: 90}
	if got := ok.LimitIssue(); got != "" {
		t.Fatalf("sane line flagged: %s", got)
	}

	bad := map[string]roadmap.PrescribedExercise{
		"too heavy":       {TargetSets: 3, TargetReps: 5, TargetWeight: 400, TargetRPE: 8},
		"negative load":   {TargetSets: 3, TargetReps: 5, TargetWeight: -5, TargetRPE: 8},
		"rpe above 10":    {TargetSets: 3, TargetReps: 5, TargetRPE: 11},
		"100 reps":        {TargetSets: 3, TargetReps: 100, TargetRPE: 7},
		"20 minutes rest": {TargetSets: 3, TargetReps: 8, TargetRPE: 7, RestSetSec: 1200},
	}
	for name, ex := range bad {
		if ex.LimitIssue() == "" {
			t.Errorf("%s: not flagged", name)
		}
	}

	warmUp := roadmap.PrescribedExercise{TargetSets: 1, DurationSeconds: 30}
	if got := warmUp.LimitIssue(); got != "" {
		t.Errorf("warm-up without rpe flagged: %s", got)
	}
}

// BR-AC-02 now reads the user's records; it skips unloaded work and history.
func TestGuardrail_WeightBandUsesUserRecords(t *testing.T) {
	var askedFor string
	prs := func(_ context.Context, userID string) map[string]float64 {
		askedFor = userID
		return map[string]float64{"ex-bench": 40} // 60 kg is above the 52 kg cap
	}

	r := buildValidRoadmap(t)
	assertViolation(t, NewEngine(nil, nil, prs).Check(context.Background(), r, nil, time.Time{}), "BR-AC-02")
	if askedFor != "user-1" {
		t.Errorf("records looked up for %q, want the roadmap owner", askedFor)
	}

	// Every session is in the past at this instant: history is not re-judged.
	after := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	assertViolation(t, NewEngine(nil, nil, prs).Check(context.Background(), r, nil, after), "")
}

// Ad-hoc work in the DELOAD week is the athlete's own and is not program
// volume: counting it would lock every later rewrite with BR-AC-11.
func TestGuardrail_DeloadVolumeIgnoresAdHocSessions(t *testing.T) {
	r := buildValidRoadmap(t) // PEAK 9 sets, DELOAD 6
	deload := r.Weeks()[3]
	date := deload.Info().StartDate.AddDate(0, 0, 5)
	dp, err := roadmap.NewDayPlan(&roadmap.DayPlanInfo{
		DayPlanID: "dp-adhoc", WeekPlanID: deload.ID(), RoadmapID: "rm-1", UserID: "user-1", ScheduledDate: date,
	})
	if err != nil {
		t.Fatal(err)
	}
	sp, err := roadmap.NewSessionPlan(&roadmap.SessionPlanInfo{
		SessionPlanID: "sp-adhoc", DayPlanID: dp.ID(), WeekPlanID: deload.ID(), RoadmapID: "rm-1", UserID: "user-1",
		ScheduledDate: date, Source: roadmap.SessionPlanSourceAdHoc,
		Prescription: roadmap.WorkoutPrescription{MainExercises: []roadmap.PrescribedExercise{
			{ExerciseID: "ex-bench", ExerciseName: "Bench", TargetSets: 9, TargetReps: 8, TargetRPE: 7},
		}},
	}, date)
	if err != nil {
		t.Fatal(err)
	}
	if err := dp.AddSession(sp); err != nil {
		t.Fatal(err)
	}
	if err := deload.AddDay(dp); err != nil {
		t.Fatal(err)
	}
	assertViolation(t, NewEngine(nil, nil, nil).Check(context.Background(), r, nil, time.Time{}), "")
}
