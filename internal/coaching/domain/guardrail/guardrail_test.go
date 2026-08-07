package guardrail

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// validRPE is mid-band for each phase, so the baseline satisfies BR-AC-10.
func validRPE() [4]float32 { return [4]float32{6.5, 7.5, 8.5, 5.5} }

// validSets unloads the DELOAD week (6 sets against PEAK's 9), satisfying
// BR-AC-11.
func validSets() [4]int32 { return [4]int32{3, 3, 3, 2} }

func buildValidRoadmap(t *testing.T) *roadmap.Roadmap {
	t.Helper()

	return buildRoadmap(t, validRPE(), validSets())
}

// buildRoadmap builds a 4-week roadmap with per-week main-work RPE and sets,
// so a test can break one periodization rule without disturbing the others.
func buildRoadmap(t *testing.T, weekRPE [4]float32, weekSets [4]int32) *roadmap.Roadmap {
	t.Helper()

	base := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC) // Monday

	weeks := make([]*roadmap.WeekPlan, 0, 4)

	for w := 0; w < 4; w++ {
		weekStart := base.AddDate(0, 0, w*7)

		phase := []roadmap.Phase{roadmap.PhaseAccumulation, roadmap.PhaseOverload, roadmap.PhasePeak, roadmap.PhaseDeload}[w]

		rpe := weekRPE[w]

		sets := weekSets[w]

		wp, err := roadmap.NewWeekPlan(&roadmap.WeekPlanInfo{
			WeekPlanID: "wp-" + itoa(w+1),
			RoadmapID:  "rm-1",
			UserID:     "user-1",
			WeekNumber: int32(w + 1),
			Phase:      phase,
			TargetRPE:  7.0,
			StartDate:  weekStart,
			EndDate:    weekStart.AddDate(0, 0, 6),
		})

		if err != nil {
			t.Fatalf("wp: %v", err)
		}

		for d := 0; d < 3; d++ {
			dp, _ := roadmap.NewDayPlan(&roadmap.DayPlanInfo{
				DayPlanID:     "dp-" + itoa(w) + "-" + itoa(d),
				WeekPlanID:    wp.ID(),
				RoadmapID:     "rm-1",
				UserID:        "user-1",
				ScheduledDate: weekStart.AddDate(0, 0, d),
			})

			sp, _ := roadmap.NewSessionPlan(&roadmap.SessionPlanInfo{
				SessionPlanID:      "sp-" + itoa(w) + "-" + itoa(d),
				DayPlanID:          dp.ID(),
				WeekPlanID:         wp.ID(),
				RoadmapID:          "rm-1",
				UserID:             "user-1",
				ScheduledDate:      weekStart.AddDate(0, 0, d),
				TargetMuscleGroups: []string{"chest"},
				Prescription: roadmap.WorkoutPrescription{
					MainExercises: []roadmap.PrescribedExercise{
						{ExerciseID: "ex-bench", ExerciseName: "Bench", TargetSets: sets, TargetReps: 8, TargetWeight: 60, TargetRPE: rpe},
					},
				},
			}, time.Now())

			_ = dp.AddSession(sp)

			_ = wp.AddDay(dp)
		}

		weeks = append(weeks, wp)
	}

	r, err := roadmap.NewRoadmap(&roadmap.Info{
		RoadmapID: "rm-1",
		UserID:    "user-1",
		StartDate: base,
		EndDate:   base.AddDate(0, 0, 28),
	}, weeks, time.Now())

	if err != nil {
		t.Fatalf("roadmap: %v", err)
	}

	return r
}

func TestGuardrail_Approved_Baseline(t *testing.T) {
	r := buildValidRoadmap(t)

	e := NewEngine(nil, func(id string) float64 {
		if id == "ex-bench" {
			return 60.0
		}

		return 0
	}, nil)

	got := e.Check(r)

	if got.Status != StatusApproved {
		t.Errorf("status = %s, violations = %+v", got.Status, got.Violations)
	}
}

func TestGuardrail_RejectsWeightOverBand(t *testing.T) {
	r := buildValidRoadmap(t)

	// Baseline PR 40 → 60kg exceeds +30% (52kg cap).

	e := NewEngine(nil, func(id string) float64 {
		if id == "ex-bench" {
			return 40.0
		}

		return 0
	}, nil)

	got := e.Check(r)

	if got.Status != StatusRejected {
		t.Fatalf("expected rejected")
	}

	foundBRAC02 := false

	for _, v := range got.Violations {
		if v.Code == "BR-AC-02" {
			foundBRAC02 = true

			break
		}
	}

	if !foundBRAC02 {
		t.Errorf("expected BR-AC-02 violation, got: %+v", got.Violations)
	}
}

func TestGuardrail_RejectsInjuryTargeted(t *testing.T) {
	r := buildValidRoadmap(t)

	// All sessions target "chest" — injury on chest should reject them all.

	e := NewEngine(nil, nil, []port.InjuryStatus{
		{MuscleGroup: "chest"},
	})

	got := e.Check(r)

	if got.Status != StatusRejected {
		t.Fatalf("expected rejected")
	}

	foundBRAC09 := false

	for _, v := range got.Violations {
		if v.Code == "BR-AC-09" {
			foundBRAC09 = true

			break
		}
	}

	if !foundBRAC09 {
		t.Errorf("expected BR-AC-09 violation")
	}
}

func TestGuardrail_RecoveredInjury_NotFlagged(t *testing.T) {
	r := buildValidRoadmap(t)

	rec := time.Now()

	e := NewEngine(nil, nil, []port.InjuryStatus{
		{MuscleGroup: "chest", RecoveredAt: &rec},
	})

	got := e.Check(r)

	if got.Status != StatusApproved {
		t.Errorf("expected approved for recovered injury, got %s: %+v", got.Status, got.Violations)
	}
}

func TestGuardrail_NilRoadmapRejected(t *testing.T) {
	e := NewEngine(nil, nil, nil)

	got := e.Check(nil)

	if got.Status != StatusRejected {
		t.Errorf("nil roadmap should reject")
	}
}

func TestGuardrail_PhaseRPEBand(t *testing.T) {
	tests := []struct {
		name string
		give [4]float32
		want string // violation code expected, empty means approved
	}{
		{name: "mid band every phase", give: validRPE()},
		{name: "accumulation too hard", give: [4]float32{7.5, 7.5, 8.5, 5.5}, want: "BR-AC-10"},
		{name: "peak too easy", give: [4]float32{6.5, 7.5, 7.0, 5.5}, want: "BR-AC-10"},
		{name: "deload at peak intensity", give: [4]float32{6.5, 7.5, 8.5, 8.5}, want: "BR-AC-10"},
		{name: "band edges are inclusive", give: [4]float32{6.0, 8.0, 8.0, 6.0}},
		{name: "unset rpe is not this rule's defect", give: [4]float32{0, 0, 0, 0}},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := NewEngine(nil, nil, nil).Check(buildRoadmap(t, tt.give, validSets()))

			assertViolation(t, got, tt.want)
		})
	}
}

func TestGuardrail_DeloadVolume(t *testing.T) {
	tests := []struct {
		name string
		give [4]int32
		want string
	}{
		{name: "deload unloads", give: validSets()},
		{name: "deload holds peak volume", give: [4]int32{3, 3, 3, 3}, want: "BR-AC-11"},
		{name: "deload exceeds peak", give: [4]int32{3, 3, 2, 3}, want: "BR-AC-11"},
		// 3 sessions a week, so PEAK totals 30 sets and allows exactly 21.
		{name: "exactly at the ratio", give: [4]int32{3, 3, 10, 7}},
		{name: "one set over the ratio", give: [4]int32{3, 3, 10, 8}, want: "BR-AC-11"},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := NewEngine(nil, nil, nil).Check(buildRoadmap(t, validRPE(), tt.give))

			assertViolation(t, got, tt.want)
		})
	}
}

// assertViolation asserts the review carries code, or approves when code is "".
func assertViolation(t *testing.T, got ReviewResult, code string) {
	t.Helper()

	if code == "" {
		if got.Status != StatusApproved {
			t.Errorf("status = %s, want %s (violations: %+v)", got.Status, StatusApproved, got.Violations)
		}

		return
	}

	for _, v := range got.Violations {
		if v.Code == code {
			return
		}
	}

	t.Errorf("violations = %+v, want one with code %s", got.Violations, code)
}
