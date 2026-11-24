package protomap

import (
	"errors"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/viethung213/gym-companion/internal/exercise/domain"
	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
)

const exerciseMigrationPath = "../../../shared/database/migrations/core/02-exercise.sql"

// TestTaxonomy_NoDrift enforces docs/05 §7: domain VO set == proto enum set
// (prefix and UNSPECIFIED stripped) == migration CHECK set.
func TestTaxonomy_NoDrift(t *testing.T) {
	migration, err := os.ReadFile(exerciseMigrationPath)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}

	tests := []struct {
		name       string
		domain     []string
		proto      map[string]int32
		prefix     string
		constraint string
	}{
		{"muscle group", codes(domain.AllMuscleGroups()), exercisemsg.MuscleGroup_value,
			prefixMuscleGroup, "chk_exercises_primary_muscle"},
		{"secondary muscles", codes(domain.AllMuscleGroups()), exercisemsg.MuscleGroup_value,
			prefixMuscleGroup, "chk_exercises_secondary_muscles"},
		{"equipment", codes(domain.AllEquipment()), exercisemsg.Equipment_value,
			prefixEquipment, "chk_exercises_equipment"},
		{"category", codes(domain.AllCategories()), exercisemsg.ExerciseCategory_value,
			prefixCategory, "chk_exercises_category"},
		{"level", codes(domain.AllLevels()), exercisemsg.ExerciseLevel_value,
			prefixLevel, "chk_exercises_level"},
		{"force", codes(domain.AllForces()), exercisemsg.ExerciseForce_value,
			prefixForce, "chk_exercises_force"},
		{"mechanic", codes(domain.AllMechanics()), exercisemsg.ExerciseMechanic_value,
			prefixMechanic, "chk_exercises_mechanic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := sorted(tt.domain)
			if got := protoCodes(tt.proto, tt.prefix); !equal(got, want) {
				t.Errorf("proto codes %v, want domain codes %v", got, want)
			}
			if got := checkCodes(t, string(migration), tt.constraint); !equal(got, want) {
				t.Errorf("migration %s codes %v, want domain codes %v", tt.constraint, got, want)
			}
		})
	}
}

func TestMuscleGroup_RoundTrip(t *testing.T) {
	for _, v := range domain.AllMuscleGroups() {
		got, err := MuscleGroupFromProto(MuscleGroupToProto(v))
		if err != nil || got != v {
			t.Errorf("round trip %s: got %q, err %v", v, got, err)
		}
	}
}

func TestFromProto_RejectsUnspecifiedAndUnknown(t *testing.T) {
	tests := []struct {
		name string
		give func() error
	}{
		{"muscle unspecified", func() error {
			_, err := MuscleGroupFromProto(exercisemsg.MuscleGroup_MUSCLE_GROUP_UNSPECIFIED)
			return err
		}},
		{"muscle unknown number", func() error {
			_, err := MuscleGroupFromProto(exercisemsg.MuscleGroup(99))
			return err
		}},
		{"equipment unspecified", func() error {
			_, err := EquipmentFromProto(exercisemsg.Equipment_EQUIPMENT_UNSPECIFIED)
			return err
		}},
		{"category unknown number", func() error {
			_, err := CategoryFromProto(exercisemsg.ExerciseCategory(42))
			return err
		}},
		{"level unspecified", func() error {
			_, err := LevelFromProto(exercisemsg.ExerciseLevel_EXERCISE_LEVEL_UNSPECIFIED)
			return err
		}},
		{"optional force unknown number", func() error {
			_, err := OptionalForceFromProto(exercisemsg.ExerciseForce(7))
			return err
		}},
		{"muscle list with unspecified", func() error {
			_, err := MuscleGroupsFromProto([]exercisemsg.MuscleGroup{
				exercisemsg.MuscleGroup_MUSCLE_GROUP_CHEST,
				exercisemsg.MuscleGroup_MUSCLE_GROUP_UNSPECIFIED,
			})
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.give(); !errors.Is(err, domain.ErrInvalidExercise) {
				t.Errorf("got error %v, want ErrInvalidExercise", err)
			}
		})
	}
}

func TestOptionalFromProto_UnspecifiedIsEmpty(t *testing.T) {
	force, err := OptionalForceFromProto(exercisemsg.ExerciseForce_EXERCISE_FORCE_UNSPECIFIED)
	if err != nil || force != "" {
		t.Errorf("force: got %q, err %v; want empty", force, err)
	}
	mechanic, err := OptionalMechanicFromProto(exercisemsg.ExerciseMechanic_EXERCISE_MECHANIC_UNSPECIFIED)
	if err != nil || mechanic != "" {
		t.Errorf("mechanic: got %q, err %v; want empty", mechanic, err)
	}
	if got := ForceToProto(""); got != exercisemsg.ExerciseForce_EXERCISE_FORCE_UNSPECIFIED {
		t.Errorf("ForceToProto(empty) = %v, want UNSPECIFIED", got)
	}
}

func codes[T ~string](vs []T) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = string(v)
	}
	return out
}

func protoCodes(values map[string]int32, prefix string) []string {
	out := make([]string, 0, len(values))
	for name, number := range values {
		if number == 0 {
			continue
		}
		out = append(out, strings.TrimPrefix(name, prefix))
	}
	return sorted(out)
}

var quotedCode = regexp.MustCompile(`'([^']+)'`)

// checkCodes returns the quoted literals inside the named CHECK constraint.
func checkCodes(t *testing.T, sql, constraint string) []string {
	t.Helper()

	start := strings.Index(sql, "CONSTRAINT "+constraint+" CHECK (")
	if start < 0 {
		t.Fatalf("constraint %s not found", constraint)
	}
	body := sql[start+len("CONSTRAINT "+constraint+" CHECK "):]
	depth := 0
	end := -1
	for i, r := range body {
		if r == '(' {
			depth++
		}
		if r == ')' {
			depth--
			if depth == 0 {
				end = i
				break
			}
		}
	}
	if end < 0 {
		t.Fatalf("constraint %s is not closed", constraint)
	}

	matches := quotedCode.FindAllStringSubmatch(body[:end], -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return sorted(out)
}

func sorted(vs []string) []string {
	out := append([]string(nil), vs...)
	sort.Strings(out)
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
