package adk

import (
	"context"
	"strings"
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

	// Strict mode fits the plan itself instead of asking for a retry.
	strict, _ := scheduledValidator().validate(context.Background(), plan, false)
	if n := issuesContaining(strict.Issues, "window"); n != 0 {
		t.Errorf("issues %v, want the window fitted, not reported", strict.Issues)
	}
	if len(strict.Fitted) != 1 || !strings.Contains(strict.Fitted[0].Reason, "ACCUMULATION window") {
		t.Errorf("fitted %v, want one ACCUMULATION window fit", strict.Fitted)
	}
	if got := mainOf(plan).TargetRPE; got != 7 {
		t.Errorf("strict RPE = %v, want 7 fitted on the plan", got)
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

	plan := build(8.5, 8.5)
	bad, _ := v.validate(context.Background(), plan, false)
	if n := issuesContaining(bad.Fitted, "DELOAD window"); n != 1 {
		t.Errorf("DELOAD session at 8.5: fitted %v, want the DELOAD window fit", bad.Fitted)
	}
	if got := plan.Weeks[0].Sessions[1].Prescription.MainExercises[0].TargetRPE; got != 6 {
		t.Errorf("DELOAD RPE = %v, want 6 (top of DELOAD)", got)
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

// Lowering the RPE lowers the load with it, 4% per point; raising it into the
// window keeps the load. An injury cap binds in every phase.
func TestFitRPE(t *testing.T) {
	peak := &SessionPlan{SessionKind: kindTraining, phase: roadmap.PhasePeak}
	tests := []struct {
		name             string
		rpe, weight, cap float64
		main             bool
		wantRPE, wantKg  float64
		wantReason       string
	}{
		{"inside the window", 8.5, 100, 0, true, 8.5, 100, ""},
		{"above the window lowers the load", 9.5, 100, 0, true, 9, 98, "PEAK window"},
		{"below the window keeps the load", 7, 100, 0, true, 8, 100, "PEAK window"},
		{"injury cap under the window", 8.5, 100, 7, true, 7, 94, "injury cap max RPE 7"},
		{"injury cap on a warm-up", 8, 0, 6, false, 6, 0, "injury cap"},
		{"no RPE is clampRPE's", 0, 100, 0, true, 0, 100, ""},
	}
	for _, tt := range tests {
		ex := PrescribedExercise{TargetRPE: tt.rpe, TargetWeightKg: tt.weight}
		reason := fitRPE(&ex, tt.main, peak, tt.cap)
		if ex.TargetRPE != tt.wantRPE || ex.TargetWeightKg != tt.wantKg {
			t.Errorf("%s: RPE %v load %v, want %v and %v", tt.name, ex.TargetRPE, ex.TargetWeightKg, tt.wantRPE, tt.wantKg)
		}
		if (tt.wantReason == "") != (reason == "") || !strings.Contains(reason, tt.wantReason) {
			t.Errorf("%s: reason %q, want one containing %q", tt.name, reason, tt.wantReason)
		}
	}
}

// A few fitted lines ship; a plan where most lines needed fitting goes back.
func TestSplitFitted(t *testing.T) {
	plan := planOf(sessionOf("2026-08-03", "a", "b", "c", "d"), sessionOf("2026-08-05", "e", "f", "g", "h"))
	fit := func(n int) []planIssue {
		out := []planIssue{{Reason: "is not in the exercise catalog"}}
		for range n {
			out = append(out, planIssue{Reason: "target_rpe fitted", Fitted: true})
		}
		return out
	}

	if open, fitted := splitFitted(plan, fit(2), false); len(open) != 1 || len(fitted) != 2 {
		t.Errorf("2 of 8 lines: open %d fitted %d, want 1 and 2", len(open), len(fitted))
	}
	if open, fitted := splitFitted(plan, fit(3), false); len(open) != 4 || fitted != nil {
		t.Errorf("3 of 8 lines: open %d fitted %d, want all 4 back to the model", len(open), len(fitted))
	}
	if open, fitted := splitFitted(plan, fit(3), true); len(open) != 1 || len(fitted) != 3 {
		t.Errorf("final round: open %d fitted %d, want the fits kept", len(open), len(fitted))
	}
}

// The 300 kg ceiling holds above every other cap: a 260 kg record allows
// 338 kg by the +30% band, and a 210 kg athlete 315 kg by bodyweight, but the
// guardrail rejects anything over 300, so the validator must clamp there too.
func TestPlanValidator_LoadNeverAboveTheAbsoluteCeiling(t *testing.T) {
	tests := []struct {
		name  string
		loads *loadLimits
	}{
		{"strong record", &loadLimits{baselines: map[string]float64{"squat": 260}, bodyWeightKg: 90}},
		{"heavy athlete without a record", &loadLimits{bodyWeightKg: 210}},
	}
	// A modest record still caps below the ceiling: 100 kg allows 130, not 300.
	modest := newPlanValidator(newFakeCatalog("squat")).withLoads(&loadLimits{baselines: map[string]float64{"squat": 100}})
	heavy := planOf(sessionOf("2026-08-03", "squat"))
	heavy.Weeks[0].Sessions[0].Prescription.MainExercises[0].TargetWeightKg = 330
	if got, _ := modest.validate(context.Background(), heavy, true); mainOf(got.Plan).TargetWeightKg != 130 {
		t.Errorf("100 kg record: salvaged load = %v, want 130", mainOf(got.Plan).TargetWeightKg)
	}
	for _, tt := range tests {
		v := newPlanValidator(newFakeCatalog("squat")).withLoads(tt.loads)
		plan := planOf(sessionOf("2026-08-03", "squat"))
		plan.Weeks[0].Sessions[0].Prescription.MainExercises[0].TargetWeightKg = 330

		strict, _ := v.validate(context.Background(), plan, false)
		if n := issuesContaining(strict.Issues, "300 kg ceiling"); n != 1 {
			t.Errorf("%s: issues %v, want the ceiling issue", tt.name, strict.Issues)
		}
		salvaged, _ := v.validate(context.Background(), plan, true)
		if got := mainOf(salvaged.Plan).TargetWeightKg; got != roadmap.MaxTargetWeightKg {
			t.Errorf("%s: salvaged load = %v, want %d", tt.name, got, roadmap.MaxTargetWeightKg)
		}
	}
}

// Reps the model gave a timed exercise are converted to a hold (3 s a rep,
// 30-60 s), not thrown away before the conversion can read them.
func TestPlanValidator_RepsOnATimedExerciseBecomeAHold(t *testing.T) {
	catalog := newFakeCatalog("plank")
	plank := catalog.byID["plank"]
	plank.MeasurementType = measurementDuration
	catalog.byID["plank"] = plank
	plan := planOf(sessionOf("2026-08-03", "plank"))
	line := &plan.Weeks[0].Sessions[0].Prescription.MainExercises[0]
	line.TargetReps, line.DurationSeconds = 20, 0

	salvaged, _ := newPlanValidator(catalog).validate(context.Background(), plan, true)
	got := mainOf(salvaged.Plan)
	if got.DurationSeconds != 60 || got.TargetReps != 0 {
		t.Errorf("salvaged plank = %d s, %d reps; want 60 s (20 reps x 3 s) and no reps", got.DurationSeconds, got.TargetReps)
	}
}
