package guardrail

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
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

	e := NewEngine(nil, nil, func(id string) float64 {
		if id == "ex-bench" {
			return 60.0
		}

		return 0
	})

	got := e.Check(r, nil, time.Time{})

	if got.Status != StatusApproved {
		t.Errorf("status = %s, violations = %+v", got.Status, got.Violations)
	}
}

func TestGuardrail_RejectsWeightOverBand(t *testing.T) {
	r := buildValidRoadmap(t)

	// Baseline PR 40 → 60kg exceeds +30% (52kg cap).

	e := NewEngine(nil, nil, func(id string) float64 {
		if id == "ex-bench" {
			return 40.0
		}

		return 0
	})

	got := e.Check(r, nil, time.Time{})

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

// benchCatalog knows ex-bench: CHEST primary, TRICEPS and SHOULDERS secondary, barbell strength.
func benchCatalog() *fakeCatalogReader {
	return &fakeCatalogReader{exercises: map[string]port.Exercise{
		"ex-bench": {
			ExerciseID:       "ex-bench",
			Name:             "Bench",
			PrimaryMuscle:    taxonomy.MuscleGroupChest,
			SecondaryMuscles: []taxonomy.MuscleGroup{taxonomy.MuscleGroupTriceps, taxonomy.MuscleGroupShoulders},
			Equipment:        taxonomy.EquipmentBarbell,
			Category:         taxonomy.CategoryStrength,
			Level:            taxonomy.LevelBeginner,
		},
	}}
}

// TestGuardrail_InjuryExclusion checks BR-AC-09 against each exercise using
// the docs/05 §8.2 table (the baseline plan prescribes ex-bench at RPE 6.5,
// 7.5, 8.5, 5.5 per week).
func TestGuardrail_InjuryExclusion(t *testing.T) {
	tests := []struct {
		name     string
		injuries []service.Injury
		want     string
	}{
		{name: "no injuries", injuries: nil},
		{
			name:     "moderate chest: primary load excluded",
			injuries: []service.Injury{{Region: taxonomy.JointRegionChest, Severity: taxonomy.InjurySeverityModerate}},
			want:     "BR-AC-09",
		},
		{
			name:     "severe elbow: secondary triceps load excluded",
			injuries: []service.Injury{{Region: taxonomy.JointRegionElbow, Severity: taxonomy.InjurySeveritySevere}},
			want:     "BR-AC-09",
		},
		{
			name:     "moderate elbow: secondary load above the RPE 6 cap",
			injuries: []service.Injury{{Region: taxonomy.JointRegionElbow, Severity: taxonomy.InjurySeverityModerate}},
			want:     "BR-AC-09",
		},
		{
			name:     "unrelated knee injury",
			injuries: []service.Injury{{Region: taxonomy.JointRegionKnee, Severity: taxonomy.InjurySeveritySevere}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewEngine(nil, benchCatalog(), nil).Check(buildValidRoadmap(t), tt.injuries, time.Time{})
			assertViolation(t, got, tt.want)
		})
	}
}

// TestGuardrail_InjuryCapLowersPhaseBand: a capped exercise within its cap
// passes both BR-AC-09 and BR-AC-10 even where the phase band is higher.
func TestGuardrail_InjuryCapLowersPhaseBand(t *testing.T) {
	mildChest := []service.Injury{{Region: taxonomy.JointRegionChest, Severity: taxonomy.InjurySeverityMild}}
	tests := []struct {
		name string
		give [4]float32
		want string
	}{
		{name: "within the RPE 7 cap every week", give: [4]float32{6.5, 7.0, 7.0, 5.5}},
		{name: "peak at RPE 8 breaks the cap", give: [4]float32{6.5, 7.0, 8.0, 5.5}, want: "BR-AC-09"},
		{name: "overload below its floor still breaks the band", give: [4]float32{6.5, 6.5, 7.0, 5.5}, want: "BR-AC-10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewEngine(nil, benchCatalog(), nil).Check(buildRoadmap(t, tt.give, validSets()), mildChest, time.Time{})
			assertViolation(t, got, tt.want)
		})
	}
}

func TestGuardrail_NilRoadmapRejected(t *testing.T) {
	e := NewEngine(nil, nil, nil)

	got := e.Check(nil, nil, time.Time{})

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

			got := NewEngine(nil, nil, nil).Check(buildRoadmap(t, tt.give, validSets()), nil, time.Time{})

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

			got := NewEngine(nil, nil, nil).Check(buildRoadmap(t, validRPE(), tt.give), nil, time.Time{})

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

type fakeCatalogReader struct {
	validIDs  map[string]bool
	exercises map[string]port.Exercise
}

func (f *fakeCatalogReader) SearchByFilter(_ context.Context, _ *port.ExerciseFilter) ([]port.Exercise, error) {
	return nil, nil
}

func (f *fakeCatalogReader) GetByID(_ context.Context, id string) (port.Exercise, error) {
	if ex, ok := f.exercises[id]; ok {
		return ex, nil
	}
	if f.validIDs[id] {
		return port.Exercise{ExerciseID: id, Name: "Name of " + id}, nil
	}
	return port.Exercise{}, port.ErrExerciseNotFound
}

func TestGuardrail_RejectsInvalidCatalogExerciseID(t *testing.T) {
	r := buildValidRoadmap(t)
	cat := &fakeCatalogReader{validIDs: map[string]bool{}} // "ex-bench" is NOT in validIDs

	e := NewEngine(nil, cat, nil)
	got := e.Check(r, nil, time.Time{})

	if got.Status != StatusRejected {
		t.Fatalf("expected status REJECTED for non-existent exercise ID")
	}

	assertViolation(t, got, "BR-AC-12")
}

// TestGuardrail_InjuryRulesOnlyApplyToRewritableSessions: a regen only rewrites
// PENDING sessions from now, so completed or past sessions that load the
// injured area must not block it (they are history, not prescriptions).
func TestGuardrail_InjuryRulesOnlyApplyToRewritableSessions(t *testing.T) {
	moderateChest := []service.Injury{{Region: taxonomy.JointRegionChest, Severity: taxonomy.InjurySeverityModerate}}
	mildChest := []service.Injury{{Region: taxonomy.JointRegionChest, Severity: taxonomy.InjurySeverityMild}}
	week2 := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	afterRoadmap := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		injuries []service.Injury
		finish   func(t *testing.T, s *roadmap.SessionPlan) // applied to every session
		from     time.Time
		want     string
	}{
		{
			name:     "pending future session loading the injury blocks",
			injuries: moderateChest,
			from:     week2,
			want:     "BR-AC-09",
		},
		{
			name:     "completed sessions loading the injury are approved",
			injuries: moderateChest,
			finish: func(t *testing.T, s *roadmap.SessionPlan) {
				if err := s.MarkCompleted(80, 0, time.Now()); err != nil {
					t.Fatalf("complete: %v", err)
				}
			},
			from: week2,
		},
		{
			name:     "skipped sessions are approved",
			injuries: moderateChest,
			finish: func(t *testing.T, s *roadmap.SessionPlan) {
				if err := s.MarkSkipped(); err != nil {
					t.Fatalf("skip: %v", err)
				}
			},
			from: week2,
		},
		{
			name:     "past pending sessions are approved",
			injuries: moderateChest,
			from:     afterRoadmap,
		},
		{
			// PEAK sessions sit at RPE 8.5 above the MILD cap of 7: only a
			// rewritable session may be held to the cap.
			name:     "injury cap does not lower the band of past sessions",
			injuries: mildChest,
			from:     afterRoadmap,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := buildValidRoadmap(t)
			if tt.finish != nil {
				for _, w := range r.Weeks() {
					for _, d := range w.Days() {
						for _, s := range d.Sessions() {
							tt.finish(t, s)
						}
					}
				}
			}
			got := NewEngine(nil, benchCatalog(), nil).Check(r, tt.injuries, tt.from)
			assertViolation(t, got, tt.want)
		})
	}
}
