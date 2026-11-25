package adk

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
)

// fakeCatalog is a hand-rolled ExerciseCatalogReader that reports which IDs it
// was asked about, so tests can assert on memoization as well as on results.
type fakeCatalog struct {
	byID  map[string]port.Exercise
	calls map[string]int
	err   error // when set, every lookup fails with this error
}

func newFakeCatalog(ids ...string) *fakeCatalog {
	byID := make(map[string]port.Exercise, len(ids))
	for _, id := range ids {
		byID[id] = port.Exercise{
			ExerciseID:    id,
			Name:          "Name of " + id,
			PrimaryMuscle: taxonomy.MuscleGroupChest,
			Equipment:     taxonomy.EquipmentBarbell,
			Category:      taxonomy.CategoryStrength,
		}
	}
	return &fakeCatalog{byID: byID, calls: make(map[string]int)}
}

func (f *fakeCatalog) SearchByFilter(context.Context, *port.ExerciseFilter) ([]port.Exercise, error) {
	return nil, nil
}

func (f *fakeCatalog) GetByID(_ context.Context, exerciseID string) (port.Exercise, error) {
	if f.err != nil {
		return port.Exercise{}, f.err
	}
	f.calls[exerciseID]++
	ex, ok := f.byID[exerciseID]
	if !ok {
		return port.Exercise{}, fmt.Errorf("%w: %s", port.ErrExerciseNotFound, exerciseID)
	}
	return ex, nil
}

var _ port.ExerciseCatalogReader = (*fakeCatalog)(nil)

// ex builds a PrescribedExercise for the ID with numbers inside every limit
// (RPE 6.5 sits in the ACCUMULATION window planOf uses).
func ex(id string) PrescribedExercise {
	return PrescribedExercise{ExerciseID: id, TargetSets: 3, TargetReps: 8, TargetRPE: 6.5}
}

// session builds a one-week-friendly SessionPlan whose main exercises are the
// given IDs.
func sessionOf(date string, mainIDs ...string) SessionPlan {
	mains := make([]PrescribedExercise, 0, len(mainIDs))
	for _, id := range mainIDs {
		mains = append(mains, ex(id))
	}
	return SessionPlan{
		ScheduledDate:            date,
		SlotTime:                 "06:00-07:30",
		SessionKind:              kindTraining, // what the backend pins for an unscheduled slot
		EstimatedDurationMinutes: 60,
		TargetMuscleGroups:       []string{"chest"},
		Prescription:             WorkoutPrescription{MainExercises: mains},
	}
}

func planOf(sessions ...SessionPlan) *GeneratedPlan {
	return &GeneratedPlan{
		Weeks: []WeekPlan{{WeekNumber: 1, Phase: "ACCUMULATION", Sessions: sessions}},
	}
}

func TestPlanValidator_AllValid(t *testing.T) {
	cat := newFakeCatalog("bench-press", "squat")
	v := newPlanValidator(cat)

	got, err := v.validate(context.Background(), planOf(sessionOf("2026-08-03", "bench-press", "squat")), false)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}

	if len(got.Issues) != 0 {
		t.Errorf("got %d issues, want 0: %v", len(got.Issues), got.Issues)
	}
	if got.Degraded {
		t.Error("got Degraded = true, want false")
	}
	if want := "Name of bench-press"; got.Names["bench-press"] != want {
		t.Errorf("got Names[bench-press] = %q, want %q", got.Names["bench-press"], want)
	}
}

func TestPlanValidator_StrictRejectsUnknownID(t *testing.T) {
	cat := newFakeCatalog("bench-press")
	v := newPlanValidator(cat)
	plan := planOf(sessionOf("2026-08-03", "bench-press", "barbell-bench-press"))

	got, err := v.validate(context.Background(), plan, false)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}

	if len(got.Issues) != 1 {
		t.Fatalf("got %d issues, want 1: %v", len(got.Issues), got.Issues)
	}
	if want := "barbell-bench-press"; got.Issues[0].ExerciseID != want {
		t.Errorf("got issue for %q, want %q", got.Issues[0].ExerciseID, want)
	}
	// Strict mode must not modify the plan.
	if n := planSessionCount(got.Plan); n != 1 {
		t.Errorf("got %d sessions in returned plan, want 1 (unmodified)", n)
	}
	if n := len(got.Plan.Weeks[0].Sessions[0].Prescription.MainExercises); n != 2 {
		t.Errorf("got %d main exercises, want 2 (unmodified)", n)
	}
}

func TestPlanValidator_DropModeRemovesInvalidExercise(t *testing.T) {
	cat := newFakeCatalog("bench-press", "squat")
	v := newPlanValidator(cat)
	plan := planOf(sessionOf("2026-08-03", "bench-press", "made-up", "squat"))

	got, err := v.validate(context.Background(), plan, true)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}

	if !got.Degraded {
		t.Error("got Degraded = false, want true")
	}
	mains := got.Plan.Weeks[0].Sessions[0].Prescription.MainExercises
	if len(mains) != 2 {
		t.Fatalf("got %d surviving main exercises, want 2", len(mains))
	}
	for _, m := range mains {
		if m.ExerciseID == "made-up" {
			t.Error("invalid exercise made-up survived salvage")
		}
	}
}

func TestPlanValidator_DropModeRemovesSessionWithNoValidMains(t *testing.T) {
	cat := newFakeCatalog("bench-press")
	v := newPlanValidator(cat)
	plan := planOf(
		sessionOf("2026-08-03", "all-fake-1", "all-fake-2"),
		sessionOf("2026-08-05", "bench-press"),
	)

	got, err := v.validate(context.Background(), plan, true)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}

	if n := planSessionCount(got.Plan); n != 1 {
		t.Fatalf("got %d surviving sessions, want 1", n)
	}
	if d := got.Plan.Weeks[0].Sessions[0].ScheduledDate; d != "2026-08-05" {
		t.Errorf("got surviving session on %s, want the sibling on 2026-08-05", d)
	}
	if !got.Degraded {
		t.Error("got Degraded = false, want true")
	}
}

// A week emptied by salvage is kept (empty) so the roadmap keeps its four weeks.
func TestPlanValidator_DropModeKeepsEmptiedWeek(t *testing.T) {
	cat := newFakeCatalog("bench-press")
	v := newPlanValidator(cat)
	plan := &GeneratedPlan{Weeks: []WeekPlan{
		{WeekNumber: 1, Sessions: []SessionPlan{sessionOf("2026-08-03", "nope")}},
		{WeekNumber: 2, Sessions: []SessionPlan{sessionOf("2026-08-10", "bench-press")}},
	}}

	got, err := v.validate(context.Background(), plan, true)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}

	if len(got.Plan.Weeks) != 2 {
		t.Fatalf("got %d weeks, want both kept", len(got.Plan.Weeks))
	}
	if n := len(got.Plan.Weeks[0].Sessions); n != 0 {
		t.Errorf("week 1 keeps %d sessions, want 0 (its only session was invalid)", n)
	}
	if !got.Degraded {
		t.Error("a dropped session must mark the plan degraded")
	}
}

func TestPlanValidator_DropModeAllSessionsDropped(t *testing.T) {
	cat := newFakeCatalog("bench-press")
	v := newPlanValidator(cat)

	got, err := v.validate(context.Background(), planOf(sessionOf("2026-08-03", "nope")), true)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}

	// The validator reports an empty plan; turning that into a hard failure is
	// the caller's job.
	if n := planSessionCount(got.Plan); n != 0 {
		t.Errorf("got %d surviving sessions, want 0", n)
	}
}

// TestPlanValidator_TransportErrorIsNotAValidationFailure guards the most
// important production behaviour here: a catalog outage must abort, not
// silently degrade every user's plan into an empty one.
func TestPlanValidator_TransportErrorIsNotAValidationFailure(t *testing.T) {
	outage := errors.New("connection refused")
	cat := newFakeCatalog("bench-press")
	cat.err = outage
	v := newPlanValidator(cat)

	for _, dropInvalid := range []bool{false, true} {
		got, err := v.validate(context.Background(), planOf(sessionOf("2026-08-03", "bench-press")), dropInvalid)
		if err == nil {
			t.Fatalf("dropInvalid=%v: got nil error, want the transport failure", dropInvalid)
		}
		if !errors.Is(err, outage) {
			t.Errorf("dropInvalid=%v: got error %v, want it to wrap %v", dropInvalid, err, outage)
		}
		if got.Degraded {
			t.Errorf("dropInvalid=%v: outage must not be reported as mere degradation", dropInvalid)
		}
	}
}

func TestPlanValidator_MemoizesLookups(t *testing.T) {
	cat := newFakeCatalog("bench-press")
	v := newPlanValidator(cat)
	plan := planOf(
		sessionOf("2026-08-03", "bench-press"),
		sessionOf("2026-08-04", "bench-press"),
		sessionOf("2026-08-05", "bench-press"),
	)

	if _, err := v.validate(context.Background(), plan, false); err != nil {
		t.Fatalf("validate returned error: %v", err)
	}

	if got := cat.calls["bench-press"]; got != 1 {
		t.Errorf("got %d catalog lookups for bench-press, want 1 (memoized)", got)
	}
}

func TestPlanValidator_WarmupsAndCooldownsValidatedButDoNotKillSession(t *testing.T) {
	cat := newFakeCatalog("bench-press")
	v := newPlanValidator(cat)

	sp := sessionOf("2026-08-03", "bench-press")
	sp.Prescription.WarmUps = []PrescribedExercise{ex("fake-warmup")}
	sp.Prescription.CoolDowns = []PrescribedExercise{ex("fake-cooldown")}

	got, err := v.validate(context.Background(), planOf(sp), true)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}

	if len(got.Issues) != 2 {
		t.Errorf("got %d issues, want 2 (one warm-up, one cool-down): %v", len(got.Issues), got.Issues)
	}
	if n := planSessionCount(got.Plan); n != 1 {
		t.Fatalf("got %d surviving sessions, want 1: a bad warm-up must not kill the session", n)
	}
	kept := got.Plan.Weeks[0].Sessions[0]
	if len(kept.Prescription.WarmUps) != 0 {
		t.Errorf("got %d warm-ups, want 0 (the invalid one dropped)", len(kept.Prescription.WarmUps))
	}
	if len(kept.Prescription.MainExercises) != 1 {
		t.Errorf("got %d main exercises, want 1 (kept)", len(kept.Prescription.MainExercises))
	}
}

func TestPlanValidator_RejectsMalformedDate(t *testing.T) {
	cat := newFakeCatalog("bench-press")
	v := newPlanValidator(cat)

	got, err := v.validate(context.Background(), planOf(sessionOf("2026-13-40", "bench-press")), true)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}

	if len(got.Issues) == 0 {
		t.Fatal("got no issues, want one naming the malformed date")
	}
	if !strings.Contains(got.Issues[0].Reason, "scheduled_date") {
		t.Errorf("got issue %q, want it to name scheduled_date", got.Issues[0].Reason)
	}
	if n := planSessionCount(got.Plan); n != 0 {
		t.Errorf("got %d surviving sessions, want 0: an unparseable date is unsalvageable", n)
	}
}

func TestPlanValidator_ReportsWeeklySessionCap(t *testing.T) {
	cat := newFakeCatalog("bench-press")
	v := newPlanValidator(cat)

	sessions := make([]SessionPlan, 0, maxSessionsPerWeek+1)
	for i := range maxSessionsPerWeek + 1 {
		sessions = append(sessions, sessionOf(fmt.Sprintf("2026-08-%02d", i+3), "bench-press"))
	}

	got, err := v.validate(context.Background(), planOf(sessions...), false)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}

	var found bool
	for _, iss := range got.Issues {
		if strings.Contains(iss.Reason, "exceeding the limit") {
			found = true
		}
	}
	if !found {
		t.Errorf("got issues %v, want one reporting the weekly session cap", got.Issues)
	}
}

func TestPlanValidator_ReportsEmptyMainExercises(t *testing.T) {
	cat := newFakeCatalog("bench-press")
	v := newPlanValidator(cat)

	sp := SessionPlan{
		ScheduledDate:            "2026-08-03",
		SlotTime:                 "06:00-07:30",
		EstimatedDurationMinutes: 45,
		Prescription:             WorkoutPrescription{},
	}

	got, err := v.validate(context.Background(), planOf(sp), false)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}

	if len(got.Issues) != 1 {
		t.Fatalf("got %d issues, want 1: %v", len(got.Issues), got.Issues)
	}
	if !strings.Contains(got.Issues[0].Reason, "at least one main exercise") {
		t.Errorf("got issue %q, want it to name the empty main_exercises slot", got.Issues[0].Reason)
	}
}

func TestPlanValidator_NilPlan(t *testing.T) {
	v := newPlanValidator(newFakeCatalog())

	got, err := v.validate(context.Background(), nil, false)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if len(got.Issues) != 1 {
		t.Errorf("got %d issues, want 1 for a nil plan", len(got.Issues))
	}
}

// injuryCatalog knows a squat (QUADRICEPS primary, GLUTES secondary) and a bench press.
func injuryCatalog() *fakeCatalog {
	cat := newFakeCatalog("bench-press")
	cat.byID["squat"] = port.Exercise{
		ExerciseID:       "squat",
		Name:             "Barbell Squat",
		PrimaryMuscle:    taxonomy.MuscleGroupQuadriceps,
		SecondaryMuscles: []taxonomy.MuscleGroup{taxonomy.MuscleGroupGlutes},
		Equipment:        taxonomy.EquipmentBarbell,
		Category:         taxonomy.CategoryStrength,
	}
	return cat
}

func TestPlanValidator_InjuryExclusionIsRetryableAndSalvageDrops(t *testing.T) {
	v := newPlanValidator(injuryCatalog()).withAthlete(&service.Athlete{
		Equipment: service.DefaultGymEquipment(),
		Injuries:  []service.Injury{{Region: taxonomy.JointRegionKnee, Severity: taxonomy.InjurySeverityModerate}},
	})
	sp := sessionOf("2026-08-03", "bench-press", "squat")

	got, err := v.validate(context.Background(), planOf(sp), false)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if len(got.Issues) != 1 || got.Issues[0].ExerciseID != "squat" ||
		!strings.Contains(got.Issues[0].Reason, "active injury") {
		t.Fatalf("got issues %v, want one injury issue for squat", got.Issues)
	}

	salvaged, err := v.validate(context.Background(), planOf(sp), true)
	if err != nil {
		t.Fatalf("validate (salvage) returned error: %v", err)
	}
	mains := salvaged.Plan.Weeks[0].Sessions[0].Prescription.MainExercises
	if len(mains) != 1 || mains[0].ExerciseID != "bench-press" || !salvaged.Degraded {
		t.Errorf("salvage kept %v (degraded=%v), want only bench-press", mains, salvaged.Degraded)
	}
}

func TestPlanValidator_InjuryRPECap(t *testing.T) {
	v := newPlanValidator(injuryCatalog()).withAthlete(&service.Athlete{
		Equipment: service.DefaultGymEquipment(),
		Injuries:  []service.Injury{{Region: taxonomy.JointRegionLowerBack, Severity: taxonomy.InjurySeverityModerate}},
	})
	sp := sessionOf("2026-08-03", "squat")
	sp.Prescription.MainExercises[0].TargetRPE = 7.5

	got, err := v.validate(context.Background(), planOf(sp), false)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if len(got.Issues) != 1 || !strings.Contains(got.Issues[0].Reason, "injury cap") {
		t.Fatalf("got issues %v, want one RPE cap issue", got.Issues)
	}
	if !strings.Contains(got.Cautions["squat"], "max RPE 6") {
		t.Errorf("got caution %q, want the RPE 6 limit", got.Cautions["squat"])
	}

	salvaged, err := v.validate(context.Background(), planOf(sp), true)
	if err != nil {
		t.Fatalf("validate (salvage) returned error: %v", err)
	}
	if rpe := salvaged.Plan.Weeks[0].Sessions[0].Prescription.MainExercises[0].TargetRPE; rpe != 6 {
		t.Errorf("salvage RPE = %v, want clamped to 6", rpe)
	}
	if sp.Prescription.MainExercises[0].TargetRPE != 7.5 {
		t.Error("salvage must not mutate the input plan")
	}
}

func TestAnnotateInjuryCautions(t *testing.T) {
	sp := sessionOf("2026-08-03", "squat", "bench-press")
	sp.Reasoning = "Leg day."
	plan := planOf(sp)

	annotateInjuryCautions(plan, map[string]string{"squat": "max RPE 7 (KNEE (MILD): caution)"})

	got := plan.Weeks[0].Sessions[0].Reasoning
	if !strings.HasPrefix(got, "Leg day.") || !strings.Contains(got, "Injury caution: squat: max RPE 7") {
		t.Errorf("got reasoning %q", got)
	}
}

func TestPlanValidator_EquipmentNotOwnedIsRetryable(t *testing.T) {
	v := newPlanValidator(injuryCatalog()).withAthlete(&service.Athlete{
		Equipment: []taxonomy.Equipment{taxonomy.EquipmentDumbbell},
	})
	sp := sessionOf("2026-08-03", "bench-press")

	got, err := v.validate(context.Background(), planOf(sp), false)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if len(got.Issues) != 1 || !strings.Contains(got.Issues[0].Reason, "BARBELL") {
		t.Errorf("got issues %v, want one missing-equipment issue", got.Issues)
	}
}
