package protomap

import (
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
)

// TestTaxonomy_NoDrift enforces docs/05 §7 for coaching: domain VO set ==
// proto enum set (prefix and UNSPECIFIED stripped). Coaching stores codes only
// in JSONB, so there is no CHECK to compare.
func TestTaxonomy_NoDrift(t *testing.T) {
	tests := []struct {
		name   string
		domain []string
		proto  map[string]int32
		prefix string
	}{
		{"muscle group", codes(taxonomy.AllMuscleGroups()), exercisemsg.MuscleGroup_value, prefixMuscleGroup},
		{"equipment", codes(taxonomy.AllEquipment()), exercisemsg.Equipment_value, prefixEquipment},
		{"category", codes(taxonomy.AllCategories()), exercisemsg.ExerciseCategory_value, prefixCategory},
		{"level", codes(taxonomy.AllLevels()), exercisemsg.ExerciseLevel_value, prefixLevel},
		{"force", codes(taxonomy.AllForces()), exercisemsg.ExerciseForce_value, prefixForce},
		{"mechanic", codes(taxonomy.AllMechanics()), exercisemsg.ExerciseMechanic_value, prefixMechanic},
		{"joint region", codes(taxonomy.AllJointRegions()), exercisemsg.JointRegion_value, prefixJointRegion},
		{"injury severity", codes(taxonomy.AllInjurySeverities()), exercisemsg.InjurySeverity_value, prefixInjurySeverity},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for name, number := range tt.proto {
				if number != 0 {
					got = append(got, strings.TrimPrefix(name, tt.prefix))
				}
			}
			sort.Strings(got)
			want := append([]string(nil), tt.domain...)
			sort.Strings(want)
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("proto codes %v, want domain codes %v", got, want)
			}
		})
	}
}

func TestFromProto(t *testing.T) {
	if got, err := JointRegion(exercisemsg.JointRegion_JOINT_REGION_KNEE); err != nil || got != taxonomy.JointRegionKnee {
		t.Errorf("JointRegion(KNEE) = %q, %v", got, err)
	}
	if got, err := OptionalForce(exercisemsg.ExerciseForce_EXERCISE_FORCE_UNSPECIFIED); err != nil || got != "" {
		t.Errorf("OptionalForce(UNSPECIFIED) = %q, %v", got, err)
	}
	if got := MuscleGroupToProto(taxonomy.MuscleGroupLowerBack); got != exercisemsg.MuscleGroup_MUSCLE_GROUP_LOWER_BACK {
		t.Errorf("MuscleGroupToProto(LOWER_BACK) = %v", got)
	}

	invalid := []func() error{
		func() error { _, err := MuscleGroup(exercisemsg.MuscleGroup_MUSCLE_GROUP_UNSPECIFIED); return err },
		func() error { _, err := Equipment(exercisemsg.Equipment(55)); return err },
		func() error {
			_, err := InjurySeverity(exercisemsg.InjurySeverity_INJURY_SEVERITY_UNSPECIFIED)
			return err
		},
		func() error { _, err := MuscleGroups([]exercisemsg.MuscleGroup{1, 0}); return err },
	}
	for i, call := range invalid {
		if err := call(); !errors.Is(err, taxonomy.ErrUnknownValue) {
			t.Errorf("case %d: got %v, want ErrUnknownValue", i, err)
		}
	}
}

func codes[T ~string](vs []T) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = string(v)
	}
	return out
}
