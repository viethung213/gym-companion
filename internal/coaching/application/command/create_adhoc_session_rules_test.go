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
