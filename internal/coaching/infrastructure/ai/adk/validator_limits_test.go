package adk

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// mainOf returns the first main exercise of the first session.
func mainOf(p *GeneratedPlan) PrescribedExercise {
	return p.Weeks[0].Sessions[0].Prescription.MainExercises[0]
}

// The numeric limits are retryable issues, and salvage ships the line clamped
// instead of the absurd value.
func TestPlanValidator_NumericLimits(t *testing.T) {
	tests := []struct {
		name  string
		edit  func(*PrescribedExercise)
		check func(PrescribedExercise) bool
	}{
		{"30 sets", func(e *PrescribedExercise) { e.TargetSets = 30 },
			func(e PrescribedExercise) bool { return e.TargetSets == roadmap.MaxTargetSets }},
		{"zero sets", func(e *PrescribedExercise) { e.TargetSets = 0 },
			func(e PrescribedExercise) bool { return e.TargetSets == 1 }},
		{"200 reps", func(e *PrescribedExercise) { e.TargetReps = 200 },
			func(e PrescribedExercise) bool { return e.TargetReps == roadmap.MaxTargetReps }},
		{"rest of an hour", func(e *PrescribedExercise) { e.RestSetSec = 3600 },
			func(e PrescribedExercise) bool { return e.RestSetSec == roadmap.MaxRestSetSec }},
		{"rpe 12", func(e *PrescribedExercise) { e.TargetRPE = 12 },
			func(e PrescribedExercise) bool { return e.TargetRPE <= roadmap.MaxTargetRPE }},
		{"rpe missing", func(e *PrescribedExercise) { e.TargetRPE = 0 },
			func(e PrescribedExercise) bool { return e.TargetRPE == defaultMainRPE }},
		{"negative load", func(e *PrescribedExercise) { e.TargetWeightKg = -20 },
			func(e PrescribedExercise) bool { return e.TargetWeightKg == 0 }},
		{"half a tonne", func(e *PrescribedExercise) { e.TargetWeightKg = 500 },
			func(e PrescribedExercise) bool { return e.TargetWeightKg == roadmap.MaxTargetWeightKg }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			build := func() *GeneratedPlan {
				p := planOf(sessionOf("2026-08-03", "bench-press"))
				tt.edit(&p.Weeks[0].Sessions[0].Prescription.MainExercises[0])
				return p
			}
			v := newPlanValidator(newFakeCatalog("bench-press"))

			strict, err := v.validate(context.Background(), build(), false)
			if err != nil {
				t.Fatal(err)
			}
			if len(strict.Issues) != 1 {
				t.Fatalf("got issues %v, want exactly one", strict.Issues)
			}

			salvaged, err := v.validate(context.Background(), build(), true)
			if err != nil {
				t.Fatal(err)
			}
			if got := mainOf(salvaged.Plan); !tt.check(got) {
				t.Errorf("salvage shipped %+v", got)
			}
		})
	}
}

// A weight outside ±30% of the athlete's record is sent back, and salvage
// clamps it to the band edge (BR-AC-02 before the guardrail).
func TestPlanValidator_LoadAgainstRecord(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press")).
		withLoads(&loadLimits{baselines: map[string]float64{"bench-press": 60}, bodyWeightKg: 70})

	build := func(kg float64) *GeneratedPlan {
		p := planOf(sessionOf("2026-08-03", "bench-press"))
		p.Weeks[0].Sessions[0].Prescription.MainExercises[0].TargetWeightKg = kg
		return p
	}

	ok, _ := v.validate(context.Background(), build(70), false) // 60 * 1.3 = 78
	if len(ok.Issues) != 0 {
		t.Errorf("70 kg against a 60 kg record: issues %v, want none", ok.Issues)
	}

	strict, _ := v.validate(context.Background(), build(100), false)
	if n := issuesContaining(strict.Issues, "30% above"); n != 1 {
		t.Errorf("100 kg against a 60 kg record: issues %v, want the band issue", strict.Issues)
	}
	salvaged, _ := v.validate(context.Background(), build(100), true)
	if got := mainOf(salvaged.Plan).TargetWeightKg; got != 78 {
		t.Errorf("salvaged load = %v, want 78 (60 + 30%%)", got)
	}
}

// Without a record the model is guessing: the guess may not exceed 1.5x
// bodyweight, so a beginner is never handed a 120 kg squat.
func TestPlanValidator_LoadWithoutRecord(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("squat")).withLoads(&loadLimits{bodyWeightKg: 60})

	p := planOf(sessionOf("2026-08-03", "squat"))
	p.Weeks[0].Sessions[0].Prescription.MainExercises[0].TargetWeightKg = 120

	salvaged, _ := v.validate(context.Background(), p, true)
	if n := issuesContaining(salvaged.Issues, "no record"); n != 1 {
		t.Errorf("issues %v, want the bodyweight issue", salvaged.Issues)
	}
	if got := mainOf(salvaged.Plan).TargetWeightKg; got != 90 {
		t.Errorf("salvaged load = %v, want 90 (1.5 x 60 kg)", got)
	}
}

// The model cannot relabel weeks: the backend pins week 3 to PEAK, so PEAK
// intensity cannot be skipped, and its main work is held to RPE 8-9.
func TestPlanValidator_PhasePinnedByWeekNumber(t *testing.T) {
	plan := planOf(fullWeekOne()...)
	plan.Weeks[0].Phase = "PEAK" // forged: week 1 is ACCUMULATION

	got, err := scheduledValidator().validate(context.Background(), plan, false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Weeks[0].Phase != string(roadmap.PhaseAccumulation) {
		t.Errorf("week 1 phase = %s, want ACCUMULATION pinned", plan.Weeks[0].Phase)
	}
	if plan.Weeks[0].TargetRPEMin != 6 || plan.Weeks[0].TargetRPEMax != 7 {
		t.Errorf("week 1 window = %v-%v, want 6-7", plan.Weeks[0].TargetRPEMin, plan.Weeks[0].TargetRPEMax)
	}
	if n := issuesContaining(got.Issues, "window"); n != 0 {
		t.Errorf("RPE 6.5 in ACCUMULATION flagged: %v", got.Issues)
	}
}

func TestPlanValidator_PhaseRPEWindow(t *testing.T) {
	plan := planOf(fullWeekOne()...)
	plan.Weeks[0].Sessions[0].Prescription.MainExercises[0].TargetRPE = 9 // ACCUMULATION is 6-7

	strict, _ := scheduledValidator().validate(context.Background(), plan, false)
	if n := issuesContaining(strict.Issues, "ACCUMULATION window"); n != 1 {
		t.Fatalf("issues %v, want one window issue", strict.Issues)
	}

	plan = planOf(fullWeekOne()...)
	plan.Weeks[0].Sessions[0].Prescription.MainExercises[0].TargetRPE = 9
	salvaged, _ := scheduledValidator().validate(context.Background(), plan, true)
	if got := mainOf(salvaged.Plan).TargetRPE; got != 7 {
		t.Errorf("salvaged RPE = %v, want 7 (top of ACCUMULATION)", got)
	}
}

// Rewrites span weeks: each revised session is held to its own week's phase,
// not to the phase of the first pending session.
func TestPlanValidator_RewriteUsesEachSessionsPhase(t *testing.T) {
	revise := []SessionToRevise{
		{ScheduledDate: "2026-08-17", SlotTime: "06:00-07:30", SessionKind: kindTraining, Phase: string(roadmap.PhasePeak)},
		{ScheduledDate: "2026-08-24", SlotTime: "06:00-07:30", SessionKind: kindTraining, Phase: string(roadmap.PhaseDeload)},
	}
	build := func(rpe ...float64) *GeneratedPlan {
		a, b := sessionOf("2026-08-17", "bench-press"), sessionOf("2026-08-24", "bench-press")
		a.Prescription.MainExercises[0].TargetRPE = rpe[0]
		b.Prescription.MainExercises[0].TargetRPE = rpe[1]
		return &GeneratedPlan{Weeks: []WeekPlan{{WeekNumber: 1, Sessions: []SessionPlan{a, b}}}}
	}
	v := newPlanValidator(newFakeCatalog("bench-press")).expecting(2).withRevise(revise)

	ok, _ := v.validate(context.Background(), build(8.5, 5.5), false)
	if len(ok.Issues) != 0 {
		t.Errorf("PEAK 8.5 then DELOAD 5.5: issues %v, want none", ok.Issues)
	}

	bad, _ := v.validate(context.Background(), build(8.5, 8.5), false)
	if n := issuesContaining(bad.Issues, "DELOAD window"); n != 1 {
		t.Errorf("DELOAD session at 8.5: issues %v, want the DELOAD window issue", bad.Issues)
	}
}

// Seen live: an adaptation listed 18 training sessions over three weeks under
// one entry, as the prompt asks, and was rejected for "18 training sessions in
// week 1". A rewrite keeps the roadmap's own dates, so the caps do not apply.
func TestPlanValidator_RewriteSpanningWeeksIsNotOneWeek(t *testing.T) {
	start := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	revise := make([]SessionToRevise, 0, 18)
	sessions := make([]SessionPlan, 0, 18)
	for w := range 3 {
		for d := range 6 {
			date := start.AddDate(0, 0, 7*w+d).Format(scheduledDateISO)
			revise = append(revise, SessionToRevise{ScheduledDate: date, SlotTime: "18:00-19:30",
				SessionKind: kindTraining, Phase: string(roadmap.PhaseOverload)})
			s := sessionOf(date, "bench-press")
			s.Prescription.MainExercises[0].TargetRPE = 7.5
			sessions = append(sessions, s)
		}
	}
	plan := &GeneratedPlan{Weeks: []WeekPlan{{WeekNumber: 1, Sessions: sessions}}}

	got, err := newPlanValidator(newFakeCatalog("bench-press")).expecting(18).withRevise(revise).
		validate(context.Background(), plan, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Issues) != 0 {
		t.Errorf("issues %v, want none for a rewrite spanning three weeks", got.Issues)
	}
}

// Each line prescribes what its exercise is measured in; a missing required
// field is reported to the model and filled only in salvage.
func TestClampMeasure(t *testing.T) {
	tests := []struct {
		name      string
		give      PrescribedExercise
		main      bool
		measure   string
		wantIssue bool
		want      PrescribedExercise
	}{
		{"timed with a duration keeps it, drops reps and load", PrescribedExercise{DurationSeconds: 40, TargetReps: 8, TargetWeightKg: 10},
			true, measurementDuration, false, PrescribedExercise{DurationSeconds: 40}},
		{"timed given reps converts them to a hold", PrescribedExercise{TargetReps: 15},
			false, measurementDuration, true, PrescribedExercise{DurationSeconds: 45}},
		{"timed main work without anything", PrescribedExercise{},
			true, measurementDuration, true, PrescribedExercise{DurationSeconds: salvageIntervalSeconds}},
		{"timed stretch without anything", PrescribedExercise{},
			false, measurementDuration, true, PrescribedExercise{DurationSeconds: salvageStretchSeconds}},
		{"reps line drops a duration", PrescribedExercise{TargetReps: 8, DurationSeconds: 30, TargetWeightKg: 60},
			true, "REPS", false, PrescribedExercise{TargetReps: 8, TargetWeightKg: 60}},
		{"reps line without reps", PrescribedExercise{TargetWeightKg: 60},
			true, "REPS", true, PrescribedExercise{TargetReps: salvageReps, TargetWeightKg: 60}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ex := tt.give
			reasons := clampMeasure(&ex, tt.main, tt.measure)
			if (len(reasons) > 0) != tt.wantIssue {
				t.Errorf("reasons %v, want issue %v", reasons, tt.wantIssue)
			}
			if ex != tt.want {
				t.Errorf("line = %+v, want %+v", ex, tt.want)
			}
		})
	}
}

// A plan that passes on its first attempt ships as the model wrote it, so the
// stray fields must be cleared on the plan itself: a plank stored with 12 reps
// and 20 kg would mislead the workout screen.
func TestPlanValidator_StrayMeasureFieldsClearedOnPassingPlan(t *testing.T) {
	cat := newFakeCatalog("plank")
	p := cat.byID["plank"]
	p.MeasurementType = measurementDuration
	cat.byID["plank"] = p

	plan := planOf(sessionOf("2026-08-03", "plank"))
	line := &plan.Weeks[0].Sessions[0].Prescription.MainExercises[0]
	line.DurationSeconds, line.TargetReps, line.TargetWeightKg = 45, 12, 20

	got, err := newPlanValidator(cat).validate(context.Background(), plan, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Issues) != 0 {
		t.Fatalf("issues %v, want none: nothing is missing", got.Issues)
	}
	if shipped := mainOf(got.Plan); shipped.TargetReps != 0 || shipped.TargetWeightKg != 0 || shipped.DurationSeconds != 45 {
		t.Errorf("shipped %+v, want the 45 s hold with no reps or load", shipped)
	}
}

// Light work against a heavy record is fine: a 140 kg single says nothing
// against 15 reps at 80 kg in a deload.
func TestPlanValidator_LightLoadAgainstARecordPasses(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("squat")).
		withLoads(&loadLimits{baselines: map[string]float64{"squat": 140}, bodyWeightKg: 80})
	p := planOf(sessionOf("2026-08-03", "squat"))
	p.Weeks[0].Sessions[0].Prescription.MainExercises[0].TargetWeightKg = 80

	got, _ := v.validate(context.Background(), p, false)
	if len(got.Issues) != 0 {
		t.Errorf("issues %v, want none for a lighter load", got.Issues)
	}
}
