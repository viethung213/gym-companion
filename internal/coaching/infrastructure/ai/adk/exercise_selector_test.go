package adk

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
)

func exerciseOf(id string, primary taxonomy.MuscleGroup, eq taxonomy.Equipment, cat taxonomy.Category) port.Exercise {
	return port.Exercise{
		ExerciseID:      id,
		Name:            "Name of " + id,
		PrimaryMuscle:   primary,
		Equipment:       eq,
		Category:        cat,
		Level:           taxonomy.LevelBeginner,
		Mechanic:        taxonomy.MechanicCompound,
		MeasurementType: measurementReps,
	}
}

func ids(tuples [][]string) []string {
	out := make([]string, 0, len(tuples))
	for _, t := range tuples {
		out = append(out, t[0])
	}
	return out
}

func TestFilterEligibleExercises(t *testing.T) {
	expert := exerciseOf("expert", taxonomy.MuscleGroupChest, taxonomy.EquipmentBarbell, taxonomy.CategoryStrength)
	expert.Level = taxonomy.LevelExpert
	elbowLoaded := exerciseOf("barbell-curl", taxonomy.MuscleGroupChest, taxonomy.EquipmentBarbell, taxonomy.CategoryStrength)
	elbowLoaded.SecondaryMuscles = []taxonomy.MuscleGroup{taxonomy.MuscleGroupTriceps}
	cableLoaded := exerciseOf("cable-press", taxonomy.MuscleGroupChest, taxonomy.EquipmentCable, taxonomy.CategoryStrength)
	cableLoaded.SecondaryMuscles = []taxonomy.MuscleGroup{taxonomy.MuscleGroupTriceps}

	all := []port.Exercise{
		elbowLoaded,
		exerciseOf("kettlebell", taxonomy.MuscleGroupChest, taxonomy.EquipmentKettlebells, taxonomy.CategoryStrength),
		exerciseOf("other", taxonomy.MuscleGroupChest, taxonomy.EquipmentOther, taxonomy.CategoryStrength),
		exerciseOf("pushup", taxonomy.MuscleGroupChest, taxonomy.EquipmentBodyOnly, taxonomy.CategoryStrength),
		exerciseOf("biceps-curl", taxonomy.MuscleGroupBiceps, taxonomy.EquipmentBarbell, taxonomy.CategoryStrength),
		expert,
		cableLoaded,
	}
	athlete := &service.Athlete{
		Equipment: service.DefaultGymEquipment(),
		Level:     taxonomy.LevelIntermediate,
		Injuries:  []service.Injury{{Region: taxonomy.JointRegionElbow, Severity: taxonomy.InjurySeverityModerate}},
	}

	got := filterEligibleExercises(all, athlete)

	gotIDs := make([]string, 0, len(got))
	for i := range got {
		gotIDs = append(gotIDs, got[i].exercise.ExerciseID)
	}
	// kettlebell/other: not owned; biceps-curl: primary in ELBOW; expert: above level.
	// barbell-curl is kept (secondary load, RPE <= 6) but moves behind guided exercises.
	want := []string{"pushup", "cable-press", "barbell-curl"}
	if !reflect.DeepEqual(gotIDs, want) {
		t.Errorf("got %v, want %v", gotIDs, want)
	}
	if got[2].verdict.MaxRPE != 6 {
		t.Errorf("barbell-curl max RPE = %v, want 6", got[2].verdict.MaxRPE)
	}
}

func TestBuildCandidateLists(t *testing.T) {
	stretch := func(id string, m taxonomy.MuscleGroup) port.Exercise {
		e := exerciseOf(id, m, taxonomy.EquipmentBodyOnly, taxonomy.CategoryStretching)
		e.MeasurementType = measurementDuration
		e.Mechanic = ""
		return e
	}
	isolation := exerciseOf("fly", taxonomy.MuscleGroupChest, taxonomy.EquipmentDumbbell, taxonomy.CategoryStrength)
	isolation.Mechanic = taxonomy.MechanicIsolation
	capped := exerciseOf("dips", taxonomy.MuscleGroupChest, taxonomy.EquipmentBodyOnly, taxonomy.CategoryStrength)
	capped.SecondaryMuscles = []taxonomy.MuscleGroup{taxonomy.MuscleGroupTriceps}

	exercises := []port.Exercise{
		exerciseOf("bench", taxonomy.MuscleGroupChest, taxonomy.EquipmentBarbell, taxonomy.CategoryStrength),
		isolation,
		capped,
		exerciseOf("row-1", taxonomy.MuscleGroupLats, taxonomy.EquipmentCable, taxonomy.CategoryStrength),
		exerciseOf("row-2", taxonomy.MuscleGroupLats, taxonomy.EquipmentCable, taxonomy.CategoryStrength),
		exerciseOf("row-3", taxonomy.MuscleGroupLats, taxonomy.EquipmentCable, taxonomy.CategoryStrength),
		exerciseOf("row-4", taxonomy.MuscleGroupLats, taxonomy.EquipmentCable, taxonomy.CategoryStrength),
		exerciseOf("run", taxonomy.MuscleGroupQuadriceps, taxonomy.EquipmentBodyOnly, taxonomy.CategoryCardio),
		stretch("chest-stretch", taxonomy.MuscleGroupChest),
		stretch("calf-stretch", taxonomy.MuscleGroupCalves),
	}
	athlete := &service.Athlete{
		Equipment: service.DefaultGymEquipment(),
		Level:     taxonomy.LevelBeginner,
		Injuries:  []service.Injury{{Region: taxonomy.JointRegionElbow, Severity: taxonomy.InjurySeverityMild}},
	}

	lists := buildCandidateLists(exercises, &candidatePrefs{
		PreferredMuscles: []taxonomy.MuscleGroup{taxonomy.MuscleGroupChest},
		PrimaryGoal:      "HYPERTROPHY",
	}, athlete)

	if got := ids(lists.main["CHEST"]); !reflect.DeepEqual(got, []string{"bench", "fly", "dips"}) {
		t.Errorf("CHEST main = %v (preferred quota 8, stretching excluded)", got)
	}
	if got := len(lists.main["LATS"]); got != defaultMuscleQuota {
		t.Errorf("LATS main has %d, want default quota %d", got, defaultMuscleQuota)
	}
	if _, ok := lists.main["QUADRICEPS"]; ok {
		t.Error("cardio must be skipped when neither preferred nor wanted by the goal")
	}
	if got := lists.main["CHEST"][1]; got[4] != movementTypeAccessory || got[2] != "DUMBBELL" || got[3] != "BEGINNER" {
		t.Errorf("fly tuple = %v, want DUMBBELL / BEGINNER / Accessory", got)
	}
	if got := ids(lists.recovery["CHEST"]); !reflect.DeepEqual(got, []string{"chest-stretch"}) {
		t.Errorf("CHEST recovery = %v, want chest-stretch", got)
	}
	if got := ids(lists.recovery[generalBucket]); !reflect.DeepEqual(got, []string{"calf-stretch"}) {
		t.Errorf("General recovery = %v, want calf-stretch", got)
	}
	if !strings.Contains(lists.limits["dips"], "max RPE 7") || len(lists.limits) != 1 {
		t.Errorf("limits = %v, want only dips at RPE 7", lists.limits)
	}

	fatLoss := buildCandidateLists(exercises, &candidatePrefs{PrimaryGoal: "FAT_LOSS"}, athlete)
	if got := fatLoss.main["QUADRICEPS"]; len(got) != 1 || got[0][4] != movementTypeCardio {
		t.Errorf("fat-loss QUADRICEPS = %v, want the cardio run", got)
	}
}

func TestAdHocIntent_ParseDropsUnknownValues(t *testing.T) {
	raw := adHocIntentResult{
		MuscleGroups: []string{"CHEST", "lower back", "Upper Legs"},
		Equipment:    []string{"DUMBBELL", "bench"},
		AvoidMuscles: []string{"TRICEPS"},
		AvoidRegions: []string{"knee", "TOE"},
	}

	got := raw.parse(context.Background())

	if !reflect.DeepEqual(got.MuscleGroups, []taxonomy.MuscleGroup{taxonomy.MuscleGroupChest, taxonomy.MuscleGroupLowerBack}) ||
		!reflect.DeepEqual(got.Equipment, []taxonomy.Equipment{taxonomy.EquipmentDumbbell}) ||
		!reflect.DeepEqual(got.AvoidRegions, []taxonomy.JointRegion{taxonomy.JointRegionKnee}) {
		t.Errorf("parsed intent = %+v", got)
	}

	avoid := got.avoidInjuries()
	want := []service.Injury{
		{Region: taxonomy.JointRegionKnee, Severity: taxonomy.InjurySeverityModerate},
		{Muscle: taxonomy.MuscleGroupTriceps, Severity: taxonomy.InjurySeverityModerate},
	}
	if !reflect.DeepEqual(avoid, want) {
		t.Errorf("avoid injuries = %v, want %v (temporary MODERATE)", avoid, want)
	}
}

type listCatalog struct {
	fakeCatalog
	all []port.Exercise
}

func (c *listCatalog) SearchByFilter(context.Context, *port.ExerciseFilter) ([]port.Exercise, error) {
	return c.all, nil
}

func TestApplyAdHocIntent_AvoidsRegionsLikeInjuries(t *testing.T) {
	cat := &listCatalog{all: []port.Exercise{
		exerciseOf("squat", taxonomy.MuscleGroupQuadriceps, taxonomy.EquipmentDumbbell, taxonomy.CategoryStrength),
		exerciseOf("press", taxonomy.MuscleGroupChest, taxonomy.EquipmentDumbbell, taxonomy.CategoryStrength),
		exerciseOf("machine-press", taxonomy.MuscleGroupChest, taxonomy.EquipmentMachine, taxonomy.CategoryStrength),
	}}
	agent := &CoachingContextAgent{catalog: cat}
	in := CoachInput{Profile: UserProfile{
		ExperienceLevel:    "BEGINNER",
		AvailableEquipment: []string{"DUMBBELL", "MACHINE"},
	}}
	intent := &adHocIntent{
		MuscleGroups: []taxonomy.MuscleGroup{taxonomy.MuscleGroupChest, taxonomy.MuscleGroupQuadriceps},
		Equipment:    []taxonomy.Equipment{taxonomy.EquipmentDumbbell},
		AvoidRegions: []taxonomy.JointRegion{taxonomy.JointRegionKnee},
	}

	if err := agent.applyAdHocIntent(context.Background(), &in, intent); err != nil {
		t.Fatalf("applyAdHocIntent: %v", err)
	}

	if _, ok := in.AvailableExercises["QUADRICEPS"]; ok {
		t.Error("squat must be excluded: KNEE avoid is a temporary MODERATE injury")
	}
	if got := ids(in.AvailableExercises["CHEST"]); !reflect.DeepEqual(got, []string{"press"}) {
		t.Errorf("CHEST = %v, want only the dumbbell press (intent equipment)", got)
	}
	if !reflect.DeepEqual(in.Profile.AvailableEquipment, []string{"DUMBBELL"}) {
		t.Errorf("profile equipment = %v, want the intent equipment for tools and validator", in.Profile.AvailableEquipment)
	}
	if len(in.Profile.ActiveInjuries) != 1 || in.Profile.ActiveInjuries[0].Region != "KNEE" ||
		in.Profile.ActiveInjuries[0].Severity != "MODERATE" {
		t.Errorf("active injuries = %v, want the KNEE avoid", in.Profile.ActiveInjuries)
	}
}

func TestReplaceInjuredExercise(t *testing.T) {
	squat := exerciseOf("squat", taxonomy.MuscleGroupQuadriceps, taxonomy.EquipmentBarbell, taxonomy.CategoryStrength)
	squat.Force = taxonomy.ForcePush
	legPress := exerciseOf("leg-press", taxonomy.MuscleGroupQuadriceps, taxonomy.EquipmentMachine, taxonomy.CategoryStrength)
	legPress.Force = taxonomy.ForcePush
	hipThrust := exerciseOf("hip-thrust", taxonomy.MuscleGroupGlutes, taxonomy.EquipmentBarbell, taxonomy.CategoryStrength)
	hipThrust.Force = taxonomy.ForcePush

	cat := &listCatalog{fakeCatalog: fakeCatalog{byID: map[string]port.Exercise{"squat": squat}, calls: map[string]int{}}}

	tests := []struct {
		name     string
		pool     []port.Exercise
		injuries []service.Injury
		want     string
	}{
		{
			name:     "same primary and force first",
			pool:     []port.Exercise{hipThrust, legPress, squat},
			injuries: []service.Injury{{Region: taxonomy.JointRegionAnkleFoot, Severity: taxonomy.InjurySeverityMild}},
			want:     "leg-press",
		},
		{
			name:     "no safe candidate drops the exercise",
			pool:     []port.Exercise{legPress, squat},
			injuries: []service.Injury{{Region: taxonomy.JointRegionKnee, Severity: taxonomy.InjurySeveritySevere}},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat.all = tt.pool
			athlete := &service.Athlete{Equipment: service.DefaultGymEquipment(), Injuries: tt.injuries}
			got, err := replaceInjuredExercise(context.Background(), cat, "squat",
				[]taxonomy.MuscleGroup{taxonomy.MuscleGroupGlutes}, athlete)
			if err != nil {
				t.Fatalf("replace: %v", err)
			}
			if got.SubstituteExerciseID != tt.want || got.SafetyReason == "" {
				t.Errorf("got %+v, want substitute %q with a safety reason", got, tt.want)
			}
		})
	}
}

func TestSearchExercises_HonoursEquipmentLevelAndInjuries(t *testing.T) {
	expert := exerciseOf("expert-press", taxonomy.MuscleGroupChest, taxonomy.EquipmentDumbbell, taxonomy.CategoryStrength)
	expert.Level = taxonomy.LevelExpert
	cat := &listCatalog{all: []port.Exercise{
		exerciseOf("db-press", taxonomy.MuscleGroupChest, taxonomy.EquipmentDumbbell, taxonomy.CategoryStrength),
		exerciseOf("pushup", taxonomy.MuscleGroupChest, taxonomy.EquipmentBodyOnly, taxonomy.CategoryStrength),
		exerciseOf("machine-press", taxonomy.MuscleGroupChest, taxonomy.EquipmentMachine, taxonomy.CategoryStrength),
		exerciseOf("cable-fly", taxonomy.MuscleGroupChest, taxonomy.EquipmentCable, taxonomy.CategoryStrength),
		exerciseOf("atlas-stone", taxonomy.MuscleGroupChest, taxonomy.EquipmentOther, taxonomy.CategoryStrength),
		exerciseOf("db-curl", taxonomy.MuscleGroupBiceps, taxonomy.EquipmentDumbbell, taxonomy.CategoryStrength),
		expert,
	}}
	athlete := &service.Athlete{
		Equipment: []taxonomy.Equipment{taxonomy.EquipmentBodyOnly, taxonomy.EquipmentDumbbell},
		Level:     taxonomy.LevelIntermediate,
		Injuries:  []service.Injury{{Region: taxonomy.JointRegionElbow, Severity: taxonomy.InjurySeveritySevere}},
	}

	got, err := searchExercises(context.Background(), cat, &SearchArgs{MuscleGroups: []string{"CHEST"}}, athlete)
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	gotIDs := make([]string, 0, len(got.Exercises))
	for _, e := range got.Exercises {
		gotIDs = append(gotIDs, e.ID)
	}
	if want := []string{"db-press", "pushup"}; !reflect.DeepEqual(gotIDs, want) {
		t.Errorf("got %v, want %v (no machine/cable/other, no expert, no elbow load)", gotIDs, want)
	}
}
