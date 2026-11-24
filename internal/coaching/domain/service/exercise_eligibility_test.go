package service

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
)

// squat loads QUADRICEPS (primary) and GLUTES, HAMSTRINGS (secondary).
func squat() ExerciseTraits {
	return ExerciseTraits{
		PrimaryMuscle:    taxonomy.MuscleGroupQuadriceps,
		SecondaryMuscles: []taxonomy.MuscleGroup{taxonomy.MuscleGroupGlutes, taxonomy.MuscleGroupHamstrings},
		Equipment:        taxonomy.EquipmentBarbell,
		Category:         taxonomy.CategoryStrength,
		Level:            taxonomy.LevelBeginner,
		Force:            taxonomy.ForcePush,
		Mechanic:         taxonomy.MechanicCompound,
	}
}

func withCategory(t ExerciseTraits, c taxonomy.Category) ExerciseTraits {
	t.Category = c
	return t
}

// Injuries chosen against squat(): KNEE covers its primary QUADRICEPS,
// LOWER_BACK covers only its secondary GLUTES, ANKLE_FOOT covers neither.
const (
	kneePrimary     = taxonomy.JointRegionKnee
	backSecondary   = taxonomy.JointRegionLowerBack
	ankleUnaffected = taxonomy.JointRegionAnkleFoot
)

// TestEvaluateInjuries_Table covers every row of docs/05 §8.2.
func TestEvaluateInjuries_Table(t *testing.T) {
	strength := squat()
	plyo := withCategory(squat(), taxonomy.CategoryPlyometrics)
	olympic := withCategory(squat(), taxonomy.CategoryOlympicWeightlifting)
	strongman := withCategory(squat(), taxonomy.CategoryStrongman)

	tests := []struct {
		name         string
		give         ExerciseTraits
		region       taxonomy.JointRegion
		severity     taxonomy.InjurySeverity
		wantExcluded bool
		wantMaxRPE   float64
		wantPrefer   bool
	}{
		// SEVERE: exclude primary or secondary load.
		{"severe primary", strength, kneePrimary, taxonomy.InjurySeveritySevere, true, 0, false},
		{"severe secondary", strength, backSecondary, taxonomy.InjurySeveritySevere, true, 0, false},
		{"severe unaffected", strength, ankleUnaffected, taxonomy.InjurySeveritySevere, false, 0, false},

		// MODERATE: exclude primary; exclude secondary + high impact; keep other secondary at RPE <= 6.
		{"moderate primary", strength, kneePrimary, taxonomy.InjurySeverityModerate, true, 0, false},
		{"moderate primary high impact", plyo, kneePrimary, taxonomy.InjurySeverityModerate, true, 0, false},
		{"moderate secondary plyometrics", plyo, backSecondary, taxonomy.InjurySeverityModerate, true, 0, false},
		{"moderate secondary olympic", olympic, backSecondary, taxonomy.InjurySeverityModerate, true, 0, false},
		{"moderate secondary strongman", strongman, backSecondary, taxonomy.InjurySeverityModerate, true, 0, false},
		{"moderate secondary kept", strength, backSecondary, taxonomy.InjurySeverityModerate, false, 6, true},
		{"moderate unaffected", strength, ankleUnaffected, taxonomy.InjurySeverityModerate, false, 0, false},

		// MILD: exclude primary or secondary + high impact; keep other load at RPE <= 7.
		{"mild primary high impact", plyo, kneePrimary, taxonomy.InjurySeverityMild, true, 0, false},
		{"mild secondary high impact", strongman, backSecondary, taxonomy.InjurySeverityMild, true, 0, false},
		{"mild primary kept", strength, kneePrimary, taxonomy.InjurySeverityMild, false, 7, false},
		{"mild secondary kept", strength, backSecondary, taxonomy.InjurySeverityMild, false, 7, false},
		{"mild unaffected high impact", plyo, ankleUnaffected, taxonomy.InjurySeverityMild, false, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateInjuries(&tt.give, []Injury{{Region: tt.region, Severity: tt.severity}})

			if got.Excluded != tt.wantExcluded || got.MaxRPE != tt.wantMaxRPE ||
				got.PreferMachineOrCable != tt.wantPrefer {
				t.Errorf("got excluded=%v maxRPE=%v prefer=%v, want %v %v %v (%s)",
					got.Excluded, got.MaxRPE, got.PreferMachineOrCable,
					tt.wantExcluded, tt.wantMaxRPE, tt.wantPrefer, got.Reason())
			}
			if affected := tt.wantExcluded || tt.wantMaxRPE > 0; affected == (got.Reason() == "") {
				t.Errorf("got reason %q, want a reason only when the exercise is affected", got.Reason())
			}
		})
	}
}

// TestEvaluateInjuries_StrictestWins checks that several injuries combine to
// the strictest outcome (§8.2 "Nhiều chấn thương").
func TestEvaluateInjuries_StrictestWins(t *testing.T) {
	ex := squat()
	tests := []struct {
		name         string
		give         []Injury
		wantExcluded bool
		wantMaxRPE   float64
		wantPrefer   bool
	}{
		{
			name: "mild primary and moderate secondary keep the lower cap",
			give: []Injury{
				{Region: kneePrimary, Severity: taxonomy.InjurySeverityMild},
				{Region: backSecondary, Severity: taxonomy.InjurySeverityModerate},
			},
			wantMaxRPE: 6, wantPrefer: true,
		},
		{
			name: "order does not matter",
			give: []Injury{
				{Region: backSecondary, Severity: taxonomy.InjurySeverityModerate},
				{Region: kneePrimary, Severity: taxonomy.InjurySeverityMild},
			},
			wantMaxRPE: 6, wantPrefer: true,
		},
		{
			name: "any exclusion excludes",
			give: []Injury{
				{Region: backSecondary, Severity: taxonomy.InjurySeverityMild},
				{Region: ankleUnaffected, Severity: taxonomy.InjurySeveritySevere},
				{Region: kneePrimary, Severity: taxonomy.InjurySeverityModerate},
			},
			wantExcluded: true, wantMaxRPE: 7,
		},
		{
			name:         "same region at two severities takes the stricter",
			give:         []Injury{{Region: backSecondary, Severity: taxonomy.InjurySeverityMild}, {Region: backSecondary, Severity: taxonomy.InjurySeveritySevere}},
			wantExcluded: true, wantMaxRPE: 7,
		},
		{name: "no injuries", give: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateInjuries(&ex, tt.give)
			if got.Excluded != tt.wantExcluded || got.MaxRPE != tt.wantMaxRPE || got.PreferMachineOrCable != tt.wantPrefer {
				t.Errorf("got excluded=%v maxRPE=%v prefer=%v, want %v %v %v",
					got.Excluded, got.MaxRPE, got.PreferMachineOrCable, tt.wantExcluded, tt.wantMaxRPE, tt.wantPrefer)
			}
		})
	}
}

func TestInjury_MusclesIncludeReportedMuscle(t *testing.T) {
	ex := squat()
	tests := []struct {
		name string
		give Injury
		want bool
	}{
		{"region without the muscle", Injury{Region: taxonomy.JointRegionElbow, Severity: taxonomy.InjurySeveritySevere}, false},
		{"reported muscle adds to the region", Injury{
			Region: taxonomy.JointRegionElbow, Muscle: taxonomy.MuscleGroupHamstrings, Severity: taxonomy.InjurySeveritySevere,
		}, true},
		{"muscle-only avoid", Injury{Muscle: taxonomy.MuscleGroupQuadriceps, Severity: taxonomy.InjurySeverityModerate}, true},
		{"unknown severity is strictest", Injury{Region: backSecondary, Severity: "WHATEVER"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EvaluateInjuries(&ex, []Injury{tt.give}).Excluded; got != tt.want {
				t.Errorf("got excluded %v, want %v", got, tt.want)
			}
		})
	}
}

// TestRegionMuscles pins the region to muscle table of docs/05 §4.
func TestRegionMuscles(t *testing.T) {
	m := func(ms ...taxonomy.MuscleGroup) []taxonomy.MuscleGroup { return ms }
	want := map[taxonomy.JointRegion][]taxonomy.MuscleGroup{
		taxonomy.JointRegionNeck: m(taxonomy.MuscleGroupNeck, taxonomy.MuscleGroupTraps),
		taxonomy.JointRegionShoulder: m(taxonomy.MuscleGroupShoulders, taxonomy.MuscleGroupChest, taxonomy.MuscleGroupLats,
			taxonomy.MuscleGroupTraps, taxonomy.MuscleGroupBiceps, taxonomy.MuscleGroupTriceps),
		taxonomy.JointRegionElbow:     m(taxonomy.MuscleGroupBiceps, taxonomy.MuscleGroupTriceps, taxonomy.MuscleGroupForearms),
		taxonomy.JointRegionWristHand: m(taxonomy.MuscleGroupForearms),
		taxonomy.JointRegionUpperBack: m(taxonomy.MuscleGroupMiddleBack, taxonomy.MuscleGroupLats, taxonomy.MuscleGroupTraps),
		taxonomy.JointRegionLowerBack: m(taxonomy.MuscleGroupLowerBack, taxonomy.MuscleGroupGlutes, taxonomy.MuscleGroupAbdominals),
		taxonomy.JointRegionChest:     m(taxonomy.MuscleGroupChest),
		taxonomy.JointRegionHip: m(taxonomy.MuscleGroupGlutes, taxonomy.MuscleGroupQuadriceps, taxonomy.MuscleGroupHamstrings,
			taxonomy.MuscleGroupAdductors, taxonomy.MuscleGroupAbductors),
		taxonomy.JointRegionKnee: m(taxonomy.MuscleGroupQuadriceps, taxonomy.MuscleGroupHamstrings, taxonomy.MuscleGroupCalves,
			taxonomy.MuscleGroupAdductors, taxonomy.MuscleGroupAbductors),
		taxonomy.JointRegionAnkleFoot: m(taxonomy.MuscleGroupCalves),
	}

	if len(want) != len(taxonomy.AllJointRegions()) {
		t.Fatalf("table covers %d regions, want all %d", len(want), len(taxonomy.AllJointRegions()))
	}
	for _, region := range taxonomy.AllJointRegions() {
		if got := RegionMuscles(region); !reflect.DeepEqual(got, want[region]) {
			t.Errorf("RegionMuscles(%s) = %v, want %v", region, got, want[region])
		}
	}
}

func TestEquipmentAvailable(t *testing.T) {
	home := []taxonomy.Equipment{taxonomy.EquipmentDumbbell}
	tests := []struct {
		give      taxonomy.Equipment
		available []taxonomy.Equipment
		want      bool
	}{
		{taxonomy.EquipmentBodyOnly, nil, true},
		{taxonomy.EquipmentDumbbell, home, true},
		{taxonomy.EquipmentBarbell, home, false},
		{taxonomy.EquipmentOther, DefaultGymEquipment(), false},
		{taxonomy.EquipmentOther, []taxonomy.Equipment{taxonomy.EquipmentOther}, true},
		{taxonomy.EquipmentMachine, DefaultGymEquipment(), true},
	}

	for _, tt := range tests {
		if got := EquipmentAvailable(tt.give, tt.available); got != tt.want {
			t.Errorf("EquipmentAvailable(%s, %v) = %v, want %v", tt.give, tt.available, got, tt.want)
		}
	}
}

func TestLevelAllowed(t *testing.T) {
	tests := []struct {
		exercise   taxonomy.Level
		experience string
		want       bool
	}{
		{taxonomy.LevelBeginner, "BEGINNER", true},
		{taxonomy.LevelIntermediate, "BEGINNER", false},
		{taxonomy.LevelIntermediate, "INTERMEDIATE", true},
		{taxonomy.LevelExpert, "INTERMEDIATE", false},
		{taxonomy.LevelExpert, "ADVANCED", true},
		{taxonomy.LevelExpert, "Advanced", true},
		{taxonomy.LevelExpert, "", true},
		{"", "BEGINNER", true},
	}

	for _, tt := range tests {
		if got := LevelAllowed(tt.exercise, UserLevel(tt.experience)); got != tt.want {
			t.Errorf("LevelAllowed(%s, %q) = %v, want %v", tt.exercise, tt.experience, got, tt.want)
		}
	}
}

func TestExerciseTraits_Predicates(t *testing.T) {
	ex := squat()
	if ex.IsBodyweight() || ex.IsMachineOrCable() || ex.IsHighImpact() || ex.IsWarmUpCoolDown() {
		t.Errorf("barbell strength squat misclassified")
	}
	ex.Equipment = taxonomy.EquipmentCable
	ex.Category = taxonomy.CategoryStretching
	if !ex.IsMachineOrCable() || !ex.IsWarmUpCoolDown() {
		t.Errorf("cable stretch misclassified")
	}
	ex.Equipment = taxonomy.EquipmentBodyOnly
	if !ex.IsBodyweight() {
		t.Errorf("body-only exercise must be bodyweight")
	}
}

// TestRankReplacements covers each ranking tier and tie-breaker of docs/05 §8.6.
func TestRankReplacements(t *testing.T) {
	original := squat() // QUADRICEPS, PUSH, COMPOUND, barbell
	cand := func(id string, mutate func(*ExerciseTraits)) ReplacementCandidate {
		tr := squat()
		mutate(&tr)
		return ReplacementCandidate{ID: id, Traits: tr}
	}
	athlete := &Athlete{
		Equipment: DefaultGymEquipment(),
		Level:     taxonomy.LevelIntermediate,
		Injuries:  []Injury{{Region: taxonomy.JointRegionAnkleFoot, Severity: taxonomy.InjurySeverityMild}},
	}
	targets := []taxonomy.MuscleGroup{taxonomy.MuscleGroupGlutes}

	tests := []struct {
		name    string
		give    []ReplacementCandidate
		athlete *Athlete
		want    []string
	}{
		{
			name: "tiers: same primary+force, same primary, session target+force",
			give: []ReplacementCandidate{
				cand("t3-glute-push", func(t *ExerciseTraits) { t.PrimaryMuscle = taxonomy.MuscleGroupGlutes }),
				cand("t2-quad-pull", func(t *ExerciseTraits) { t.Force = taxonomy.ForcePull }),
				cand("t1-quad-push", func(*ExerciseTraits) {}),
			},
			want: []string{"t1-quad-push", "t2-quad-pull", "t3-glute-push"},
		},
		{
			name: "outside every tier is dropped",
			give: []ReplacementCandidate{
				cand("glute-pull", func(t *ExerciseTraits) { t.PrimaryMuscle = taxonomy.MuscleGroupGlutes; t.Force = taxonomy.ForcePull }),
				cand("chest-push", func(t *ExerciseTraits) { t.PrimaryMuscle = taxonomy.MuscleGroupChest }),
			},
			want: nil,
		},
		{
			name: "same mechanic first within a tier",
			give: []ReplacementCandidate{
				cand("isolation", func(t *ExerciseTraits) { t.Mechanic = taxonomy.MechanicIsolation }),
				cand("compound", func(*ExerciseTraits) {}),
			},
			want: []string{"compound", "isolation"},
		},
		{
			name: "mild injury keeps catalog order for equipment",
			give: []ReplacementCandidate{
				cand("barbell", func(*ExerciseTraits) {}),
				cand("machine", func(t *ExerciseTraits) { t.Equipment = taxonomy.EquipmentMachine }),
			},
			want: []string{"barbell", "machine"},
		},
		{
			name: "moderate injury prefers machine or cable after mechanic",
			give: []ReplacementCandidate{
				cand("barbell", func(*ExerciseTraits) {}),
				cand("cable-isolation", func(t *ExerciseTraits) {
					t.Equipment = taxonomy.EquipmentCable
					t.Mechanic = taxonomy.MechanicIsolation
				}),
				cand("machine", func(t *ExerciseTraits) { t.Equipment = taxonomy.EquipmentMachine }),
			},
			athlete: &Athlete{
				Equipment: DefaultGymEquipment(), Level: taxonomy.LevelIntermediate,
				Injuries: []Injury{{Region: taxonomy.JointRegionElbow, Severity: taxonomy.InjurySeverityModerate}},
			},
			want: []string{"machine", "barbell", "cable-isolation"},
		},
		{
			name: "excluded, stretching, too hard, missing equipment and original are not candidates",
			give: []ReplacementCandidate{
				cand("original", func(*ExerciseTraits) {}),
				cand("stretch", func(t *ExerciseTraits) { t.Category = taxonomy.CategoryStretching }),
				cand("expert", func(t *ExerciseTraits) { t.Level = taxonomy.LevelExpert }),
				cand("kettlebell", func(t *ExerciseTraits) { t.Equipment = taxonomy.EquipmentKettlebells }),
				cand("calf-load", func(t *ExerciseTraits) {
					t.SecondaryMuscles = []taxonomy.MuscleGroup{taxonomy.MuscleGroupCalves}
					t.Category = taxonomy.CategoryPlyometrics
				}),
				cand("ok", func(*ExerciseTraits) {}),
			},
			want: []string{"ok"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := tt.athlete
			if a == nil {
				a = athlete
			}
			var got []string
			for _, c := range RankReplacements("original", &original, tt.give, targets, a) {
				got = append(got, c.ID)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

type upstreamTraits struct {
	ID               string   `json:"id"`
	Equipment        *string  `json:"equipment"`
	Category         string   `json:"category"`
	PrimaryMuscles   []string `json:"primaryMuscles"`
	SecondaryMuscles []string `json:"secondaryMuscles"`
}

func sourceCode(v string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(v, "-", ""), " ", "_"))
}

// TestEligibilityCensus counts the catalog exercises left per region and
// severity on the vendored free-exercise-db data (docs/05 §12) and checks the
// counts are monotonic: a stricter severity never keeps more exercises.
func TestEligibilityCensus(t *testing.T) {
	raw, err := os.ReadFile("../../../shared/seeder/data/upstream/free-exercise-db/exercises.json")
	if err != nil {
		t.Fatalf("read upstream catalog: %v", err)
	}
	var records []upstreamTraits
	if err := json.Unmarshal(raw, &records); err != nil {
		t.Fatalf("unmarshal upstream catalog: %v", err)
	}

	catalog := make(map[string]ExerciseTraits, len(records))
	for _, r := range records {
		tr := ExerciseTraits{
			PrimaryMuscle: taxonomy.MuscleGroup(sourceCode(r.PrimaryMuscles[0])),
			Category:      taxonomy.Category(sourceCode(r.Category)),
		}
		for _, m := range append(r.PrimaryMuscles[1:], r.SecondaryMuscles...) {
			tr.SecondaryMuscles = append(tr.SecondaryMuscles, taxonomy.MuscleGroup(sourceCode(m)))
		}
		catalog[r.ID] = tr
	}

	severities := taxonomy.AllInjurySeverities()
	lines := []string{fmt.Sprintf("%-11s %6s %6s %6s   (of %d)", "region", "MILD", "MOD", "SEVERE", len(catalog))}
	for _, region := range taxonomy.AllJointRegions() {
		kept := make([]int, len(severities))
		for i, sev := range severities {
			for _, tr := range catalog {
				if !EvaluateInjuries(&tr, []Injury{{Region: region, Severity: sev}}).Excluded {
					kept[i]++
				}
			}
		}
		if !(kept[0] >= kept[1] && kept[1] >= kept[2]) {
			t.Errorf("%s: kept %v is not monotonic MILD >= MODERATE >= SEVERE", region, kept)
		}
		lines = append(lines, fmt.Sprintf("%-11s %6d %6d %6d", region, kept[0], kept[1], kept[2]))
	}
	sort.Strings(lines[1:])
	t.Logf("exercises kept per region x severity:\n%s", strings.Join(lines, "\n"))

	// §12 example: RDL is kept under MODERATE LOWER_BACK (hamstring primary), dropped under SEVERE.
	rdl := catalog["Romanian_Deadlift"]
	if v := EvaluateInjuries(&rdl, []Injury{{Region: taxonomy.JointRegionLowerBack, Severity: taxonomy.InjurySeverityModerate}}); v.Excluded || v.MaxRPE != 6 {
		t.Errorf("RDL under MODERATE LOWER_BACK: got excluded=%v maxRPE=%v, want kept at RPE 6", v.Excluded, v.MaxRPE)
	}
	if v := EvaluateInjuries(&rdl, []Injury{{Region: taxonomy.JointRegionLowerBack, Severity: taxonomy.InjurySeveritySevere}}); !v.Excluded {
		t.Error("RDL under SEVERE LOWER_BACK must be excluded")
	}
}
