package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
)

// benchOnly is a catalog where every id resolves to a barbell bench press.
func benchOnly() *exerciseCatalogMock {
	return &exerciseCatalogMock{fn: func(_ context.Context, id string) (port.Exercise, error) {
		return port.Exercise{
			ExerciseID: id, Name: "Bench Press",
			PrimaryMuscle:    taxonomy.MuscleGroupChest,
			SecondaryMuscles: []taxonomy.MuscleGroup{taxonomy.MuscleGroupTriceps, taxonomy.MuscleGroupShoulders},
			Equipment:        taxonomy.EquipmentBarbell, Level: taxonomy.LevelIntermediate,
		}, nil
	}}
}

// injuredProfiles returns a profile in loc carrying the given injuries.
type injuredProfiles struct {
	loc      *time.Location
	injuries []port.InjuryStatus
}

func (p injuredProfiles) GetProfile(_ context.Context, userID string) (port.Profile, error) {
	return port.Profile{UserID: userID, Location: p.loc, ActiveInjuries: p.injuries}, nil
}

// oneWeekRoadmap is an empty roadmap whose single week starts on start.
func oneWeekRoadmap(t *testing.T, start time.Time) *roadmap.Roadmap {
	t.Helper()
	wp, err := roadmap.NewWeekPlan(&roadmap.WeekPlanInfo{
		WeekPlanID: "w-1", RoadmapID: "rm-1", UserID: "user-1", WeekNumber: 1,
		Phase: roadmap.PhaseAccumulation, StartDate: start, EndDate: start.AddDate(0, 0, 7),
	})
	if err != nil {
		t.Fatal(err)
	}
	rm, err := roadmap.NewRoadmap(&roadmap.Info{
		RoadmapID: "rm-1", UserID: "user-1", Status: roadmap.StatusActive,
		StartDate: start, EndDate: start.AddDate(0, 0, 28),
	}, []*roadmap.WeekPlan{wp}, start)
	if err != nil {
		t.Fatal(err)
	}
	return rm
}

func adHocHandler(rm *roadmap.Roadmap, profiles port.UserProfileReader, now time.Time) *CreateAdhocSessionHandler {
	n := 0
	return NewCreateAdhocSessionHandler(
		&transactionManagerMock{fn: func(ctx context.Context, f func(context.Context) error) error { return f(ctx) }},
		&roadmapRepositoryMock{
			findActiveByUserFn: func(context.Context, string) (*roadmap.Roadmap, error) { return rm, nil },
			saveFn:             func(context.Context, *roadmap.Roadmap) error { return nil },
		},
		benchOnly(), profiles,
		&idGeneratorMock{fn: func() string { n++; return "id-" + string(rune('a'+n)) }},
		&clockMock{now: now},
	)
}

// At 23:30 UTC it is already the next morning in Ho Chi Minh City: the session
// belongs to the athlete's day, on a DayPlan created for it. The day is read in
// the roadmap's zone, the athlete's zone its sessions are dated in.
func TestCreateAdhoc_UsesAthletesLocalDate(t *testing.T) {
	hcm, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Skip("tzdata unavailable")
	}
	start := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)
	rm := oneWeekRoadmap(t, start)
	rm.SetTimezone(hcm.String())
	now := time.Date(2026, 3, 10, 23, 30, 0, 0, time.UTC) // Wed 06:30 in HCM

	res, err := adHocHandler(rm, injuredProfiles{loc: hcm}, now).
		Handle(context.Background(), CreateAdhocSessionCommand{UserID: "user-1", ExerciseIDs: []string{"ex-1"}})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	want := time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC)
	if got := res.SessionPlan.ScheduledDate(); !got.Equal(want) {
		t.Errorf("scheduled %s, want the local date %s", got.Format(time.DateOnly), want.Format(time.DateOnly))
	}
	day, ok := findDay(rm, res.SessionPlan.Info().DayPlanID)
	if !ok || !day.ScheduledDate().Equal(want) {
		t.Errorf("session placed on day %v, want a DayPlan dated %s", day, want.Format(time.DateOnly))
	}
}

// It used to land on the roadmap's last day; now it is refused.
func TestCreateAdhoc_OutsideRoadmapIsRefused(t *testing.T) {
	rm := oneWeekRoadmap(t, time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC))
	_, err := adHocHandler(rm, injuredProfiles{}, time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)).
		Handle(context.Background(), CreateAdhocSessionCommand{UserID: "user-1", ExerciseIDs: []string{"ex-1"}})
	if !errors.Is(err, roadmap.ErrDateOutsideRoadmap) {
		t.Fatalf("err = %v, want ErrDateOutsideRoadmap", err)
	}
}

// A severe chest injury excludes the bench press; a mild one caps its RPE.
func TestCreateAdhoc_RespectsInjuries(t *testing.T) {
	start := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)
	now := start.Add(10 * time.Hour)
	cmd := CreateAdhocSessionCommand{UserID: "user-1", ExerciseIDs: []string{"ex-1"}}

	severe := injuredProfiles{injuries: []port.InjuryStatus{{Region: taxonomy.JointRegionChest, Severity: taxonomy.InjurySeveritySevere}}}
	if _, err := adHocHandler(oneWeekRoadmap(t, start), severe, now).Handle(context.Background(), cmd); !errors.Is(err, ErrExerciseExcludedByInjury) {
		t.Fatalf("severe chest injury: err = %v, want ErrExerciseExcludedByInjury", err)
	}

	mild := injuredProfiles{injuries: []port.InjuryStatus{{Region: taxonomy.JointRegionChest, Severity: taxonomy.InjurySeverityMild}}}
	res, err := adHocHandler(oneWeekRoadmap(t, start), mild, now).Handle(context.Background(), cmd)
	if err != nil {
		t.Fatalf("mild chest injury: %v", err)
	}
	line := res.SessionPlan.Info().Prescription.MainExercises[0]
	if line.TargetRPE > adHocTargetRPE || line.Notes == "" {
		t.Errorf("mild injury line = RPE %v notes %q, want RPE within the cap and a caution", line.TargetRPE, line.Notes)
	}
}

func findDay(rm *roadmap.Roadmap, id string) (*roadmap.DayPlan, bool) {
	for _, w := range rm.Weeks() {
		for _, d := range w.Days() {
			if d.ID() == id {
				return d, true
			}
		}
	}
	return nil, false
}

// A session built from a suggestion keeps its warm-up, its numbers and its
// cool-down, instead of becoming default 3 x 8 main lines.
func TestCreateAdhoc_KeepsTheBuiltPrescription(t *testing.T) {
	start := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)
	built := &roadmap.WorkoutPrescription{
		WarmUps:       []roadmap.PrescribedExercise{{ExerciseID: "warm", TargetSets: 1, DurationSeconds: 60}},
		MainExercises: []roadmap.PrescribedExercise{{ExerciseID: "bench", TargetSets: 4, TargetReps: 10, TargetWeight: 12.5, TargetRPE: 6.5, RestSetSec: 90}},
		CoolDowns:     []roadmap.PrescribedExercise{{ExerciseID: "cool", DurationSeconds: 60}},
	}
	res, err := adHocHandler(oneWeekRoadmap(t, start), injuredProfiles{}, start.Add(10*time.Hour)).
		Handle(context.Background(), CreateAdhocSessionCommand{UserID: "user-1", Prescription: built})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	p := res.SessionPlan.Info().Prescription
	if len(p.WarmUps) != 1 || len(p.MainExercises) != 1 || len(p.CoolDowns) != 1 {
		t.Fatalf("prescription %+v, want the three parts kept", p)
	}
	if m := p.MainExercises[0]; m.TargetSets != 4 || m.TargetReps != 10 || m.TargetWeight != 12.5 || m.TargetRPE != 6.5 {
		t.Errorf("main line %+v, want 4 x 10 at 12.5 kg RPE 6.5", m)
	}
	if c := p.CoolDowns[0]; c.TargetSets != 1 || c.DurationSeconds != 60 || c.TargetReps != 0 {
		t.Errorf("cool-down %+v, want one 60 s hold", c)
	}
	if p.MainExercises[0].ExerciseName == "" {
		t.Error("exercise name not filled from the catalog")
	}
}

// The built session is still held to the athlete: an injury caps a line's
// RPE, and a line past a hard limit is refused.
func TestCreateAdhoc_BuiltPrescriptionIsChecked(t *testing.T) {
	start := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)
	now := start.Add(10 * time.Hour)
	mild := injuredProfiles{injuries: []port.InjuryStatus{{Region: taxonomy.JointRegionChest, Severity: taxonomy.InjurySeverityMild}}}
	cmd := func(sets int32, rpe float32) CreateAdhocSessionCommand {
		return CreateAdhocSessionCommand{UserID: "user-1", Prescription: &roadmap.WorkoutPrescription{
			MainExercises: []roadmap.PrescribedExercise{{ExerciseID: "bench", TargetSets: sets, TargetReps: 8, TargetRPE: rpe}},
		}}
	}

	res, err := adHocHandler(oneWeekRoadmap(t, start), mild, now).Handle(context.Background(), cmd(3, 9))
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if line := res.SessionPlan.Info().Prescription.MainExercises[0]; line.TargetRPE > 7 || line.Notes == "" {
		t.Errorf("line RPE %v notes %q, want the mild-injury cap 7 and a caution", line.TargetRPE, line.Notes)
	}

	if _, err := adHocHandler(oneWeekRoadmap(t, start), injuredProfiles{}, now).Handle(context.Background(), cmd(30, 7)); !errors.Is(err, ErrInvalidAdhocPrescription) {
		t.Errorf("30 sets: err = %v, want ErrInvalidAdhocPrescription", err)
	}
}

// A timed exercise left without numbers gets a hold, not 8 reps: 30 s as a
// warm-up or cool-down, 45 s as main work. A counted one gets 8 reps.
func TestCreateAdhoc_BuiltLineDefaultsFollowTheMeasure(t *testing.T) {
	start := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)
	catalog := &exerciseCatalogMock{fn: func(_ context.Context, id string) (port.Exercise, error) {
		ex := port.Exercise{ExerciseID: id, Name: id, PrimaryMuscle: taxonomy.MuscleGroupAbdominals,
			Equipment: taxonomy.EquipmentBodyOnly, Level: taxonomy.LevelBeginner, MeasurementType: "REPS"}
		if id == "plank" {
			ex.MeasurementType = "DURATION"
		}
		return ex, nil
	}}
	n := 0
	h := NewCreateAdhocSessionHandler(
		&transactionManagerMock{fn: func(ctx context.Context, f func(context.Context) error) error { return f(ctx) }},
		&roadmapRepositoryMock{
			findActiveByUserFn: func(context.Context, string) (*roadmap.Roadmap, error) { return oneWeekRoadmap(t, start), nil },
			saveFn:             func(context.Context, *roadmap.Roadmap) error { return nil },
		},
		catalog, injuredProfiles{},
		&idGeneratorMock{fn: func() string { n++; return "id-" + string(rune('a'+n)) }},
		&clockMock{now: start.Add(10 * time.Hour)},
	)

	res, err := h.Handle(context.Background(), CreateAdhocSessionCommand{UserID: "user-1", Prescription: &roadmap.WorkoutPrescription{
		MainExercises: []roadmap.PrescribedExercise{{ExerciseID: "plank"}, {ExerciseID: "crunch"}},
		CoolDowns:     []roadmap.PrescribedExercise{{ExerciseID: "plank"}},
	}})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	p := res.SessionPlan.Info().Prescription
	if m := p.MainExercises[0]; m.DurationSeconds != 45 || m.TargetReps != 0 {
		t.Errorf("timed main line %+v, want a 45 s hold", m)
	}
	if m := p.MainExercises[1]; m.TargetReps != 8 || m.DurationSeconds != 0 {
		t.Errorf("counted main line %+v, want 8 reps", m)
	}
	if c := p.CoolDowns[0]; c.DurationSeconds != 30 || c.TargetReps != 0 || c.TargetSets != 1 {
		t.Errorf("timed cool-down %+v, want one 30 s hold", c)
	}
}

// Reproduces the review finding: an ad-hoc session requested by exercise ids
// (the solo-match path) still prescribed a timed exercise as 3 x 8 reps.
func TestCreateAdhoc_ByIDsTimedExerciseGetsAHold(t *testing.T) {
	start := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)
	catalog := &exerciseCatalogMock{fn: func(_ context.Context, id string) (port.Exercise, error) {
		return port.Exercise{ExerciseID: id, Name: id, PrimaryMuscle: taxonomy.MuscleGroupAbdominals,
			Equipment: taxonomy.EquipmentBodyOnly, Level: taxonomy.LevelBeginner, MeasurementType: "DURATION"}, nil
	}}
	n := 0
	h := NewCreateAdhocSessionHandler(
		&transactionManagerMock{fn: func(ctx context.Context, f func(context.Context) error) error { return f(ctx) }},
		&roadmapRepositoryMock{
			findActiveByUserFn: func(context.Context, string) (*roadmap.Roadmap, error) { return oneWeekRoadmap(t, start), nil },
			saveFn:             func(context.Context, *roadmap.Roadmap) error { return nil },
		},
		catalog, injuredProfiles{},
		&idGeneratorMock{fn: func() string { n++; return "id-" + string(rune('a'+n)) }},
		&clockMock{now: start.Add(10 * time.Hour)},
	)
	res, err := h.Handle(context.Background(), CreateAdhocSessionCommand{UserID: "user-1", ExerciseIDs: []string{"plank"}})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if m := res.SessionPlan.Info().Prescription.MainExercises[0]; m.DurationSeconds != 45 || m.TargetReps != 0 {
		t.Errorf("timed line %+v, want a 45 s hold", m)
	}
}

// Reproduces the review finding: a built session had no bound on its lines,
// so 1,000 lines of 10 sets were stored (showing 180 min, the length cap) after
// 1,000 catalog calls. Too many lines, or a session longer than
// MaxSessionMinutes, is refused before any catalog call.
func TestCreateAdhoc_BuiltPrescriptionIsBounded(t *testing.T) {
	start := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)
	calls := 0
	catalog := &exerciseCatalogMock{fn: func(_ context.Context, id string) (port.Exercise, error) {
		calls++
		return port.Exercise{ExerciseID: id, Name: id, PrimaryMuscle: taxonomy.MuscleGroupChest,
			Equipment: taxonomy.EquipmentBarbell, Level: taxonomy.LevelIntermediate, MeasurementType: "REPS"}, nil
	}}
	handler := func() *CreateAdhocSessionHandler {
		n := 0
		return NewCreateAdhocSessionHandler(
			&transactionManagerMock{fn: func(ctx context.Context, f func(context.Context) error) error { return f(ctx) }},
			&roadmapRepositoryMock{
				findActiveByUserFn: func(context.Context, string) (*roadmap.Roadmap, error) { return oneWeekRoadmap(t, start), nil },
				saveFn:             func(context.Context, *roadmap.Roadmap) error { return nil },
			},
			catalog, injuredProfiles{},
			&idGeneratorMock{fn: func() string { n++; return "id-" + string(rune('a'+n)) }},
			&clockMock{now: start.Add(10 * time.Hour)},
		)
	}
	lines := func(n int, sets int32) []roadmap.PrescribedExercise {
		out := make([]roadmap.PrescribedExercise, n)
		for i := range out {
			out[i] = roadmap.PrescribedExercise{ExerciseID: "bench", TargetSets: sets, TargetReps: 10, RestSetSec: 90}
		}
		return out
	}

	_, err := handler().Handle(context.Background(), CreateAdhocSessionCommand{UserID: "user-1",
		Prescription: &roadmap.WorkoutPrescription{MainExercises: lines(1000, 10)}})
	if !errors.Is(err, ErrInvalidAdhocPrescription) || calls != 0 {
		t.Errorf("1000 lines: err %v after %d catalog calls, want ErrInvalidAdhocPrescription before any", err, calls)
	}

	_, err = handler().Handle(context.Background(), CreateAdhocSessionCommand{UserID: "user-1",
		Prescription: &roadmap.WorkoutPrescription{MainExercises: lines(12, 10)}}) // ~4 h of work
	if !errors.Is(err, ErrInvalidAdhocPrescription) {
		t.Errorf("a 4-hour session: err %v, want ErrInvalidAdhocPrescription", err)
	}

	calls = 0
	if _, err = handler().Handle(context.Background(), CreateAdhocSessionCommand{UserID: "user-1",
		Prescription: &roadmap.WorkoutPrescription{MainExercises: lines(5, 3)}}); err != nil {
		t.Fatalf("a normal session: %v", err)
	}
	if calls != 1 {
		t.Errorf("catalog calls = %d for one exercise on 5 lines, want 1", calls)
	}
}
