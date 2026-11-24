// Package protomap maps exercise taxonomy value objects to and from the
// contract enums. A proto value name is the enum prefix plus the domain code.
package protomap

import (
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/exercise/domain"
	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
)

const (
	prefixMuscleGroup = "MUSCLE_GROUP_"
	prefixEquipment   = "EQUIPMENT_"
	prefixCategory    = "EXERCISE_CATEGORY_"
	prefixLevel       = "EXERCISE_LEVEL_"
	prefixForce       = "EXERCISE_FORCE_"
	prefixMechanic    = "EXERCISE_MECHANIC_"
)

type protoEnum interface {
	~int32
	String() string
}

func MuscleGroupToProto(v domain.MuscleGroup) exercisemsg.MuscleGroup {
	return exercisemsg.MuscleGroup(exercisemsg.MuscleGroup_value[prefixMuscleGroup+string(v)])
}

// MuscleGroupFromProto rejects UNSPECIFIED and unknown values.
func MuscleGroupFromProto(e exercisemsg.MuscleGroup) (domain.MuscleGroup, error) {
	return fromProto(e, prefixMuscleGroup, domain.ParseMuscleGroup)
}

func MuscleGroupsToProto(vs []domain.MuscleGroup) []exercisemsg.MuscleGroup {
	if len(vs) == 0 {
		return nil
	}
	out := make([]exercisemsg.MuscleGroup, len(vs))
	for i, v := range vs {
		out[i] = MuscleGroupToProto(v)
	}
	return out
}

// MuscleGroupsFromProto rejects the whole list if any value is invalid.
func MuscleGroupsFromProto(es []exercisemsg.MuscleGroup) ([]domain.MuscleGroup, error) {
	return listFromProto(es, MuscleGroupFromProto)
}

func EquipmentToProto(v domain.Equipment) exercisemsg.Equipment {
	return exercisemsg.Equipment(exercisemsg.Equipment_value[prefixEquipment+string(v)])
}

func EquipmentFromProto(e exercisemsg.Equipment) (domain.Equipment, error) {
	return fromProto(e, prefixEquipment, domain.ParseEquipment)
}

func EquipmentListFromProto(es []exercisemsg.Equipment) ([]domain.Equipment, error) {
	return listFromProto(es, EquipmentFromProto)
}

func CategoryToProto(v domain.Category) exercisemsg.ExerciseCategory {
	return exercisemsg.ExerciseCategory(exercisemsg.ExerciseCategory_value[prefixCategory+string(v)])
}

func CategoryFromProto(e exercisemsg.ExerciseCategory) (domain.Category, error) {
	return fromProto(e, prefixCategory, domain.ParseCategory)
}

func CategoriesFromProto(es []exercisemsg.ExerciseCategory) ([]domain.Category, error) {
	return listFromProto(es, CategoryFromProto)
}

func LevelToProto(v domain.Level) exercisemsg.ExerciseLevel {
	return exercisemsg.ExerciseLevel(exercisemsg.ExerciseLevel_value[prefixLevel+string(v)])
}

func LevelFromProto(e exercisemsg.ExerciseLevel) (domain.Level, error) {
	return fromProto(e, prefixLevel, domain.ParseLevel)
}

func LevelsFromProto(es []exercisemsg.ExerciseLevel) ([]domain.Level, error) {
	return listFromProto(es, LevelFromProto)
}

// ForceToProto maps the empty Force to UNSPECIFIED.
func ForceToProto(v domain.Force) exercisemsg.ExerciseForce {
	return exercisemsg.ExerciseForce(exercisemsg.ExerciseForce_value[prefixForce+string(v)])
}

// ForceFromProto rejects UNSPECIFIED; use OptionalForceFromProto for nullable fields.
func ForceFromProto(e exercisemsg.ExerciseForce) (domain.Force, error) {
	return fromProto(e, prefixForce, domain.ParseForce)
}

// OptionalForceFromProto maps UNSPECIFIED to the empty Force.
func OptionalForceFromProto(e exercisemsg.ExerciseForce) (domain.Force, error) {
	if e == exercisemsg.ExerciseForce_EXERCISE_FORCE_UNSPECIFIED {
		return "", nil
	}
	return ForceFromProto(e)
}

// MechanicToProto maps the empty Mechanic to UNSPECIFIED.
func MechanicToProto(v domain.Mechanic) exercisemsg.ExerciseMechanic {
	return exercisemsg.ExerciseMechanic(exercisemsg.ExerciseMechanic_value[prefixMechanic+string(v)])
}

// MechanicFromProto rejects UNSPECIFIED; use OptionalMechanicFromProto for nullable fields.
func MechanicFromProto(e exercisemsg.ExerciseMechanic) (domain.Mechanic, error) {
	return fromProto(e, prefixMechanic, domain.ParseMechanic)
}

// OptionalMechanicFromProto maps UNSPECIFIED to the empty Mechanic.
func OptionalMechanicFromProto(e exercisemsg.ExerciseMechanic) (domain.Mechanic, error) {
	if e == exercisemsg.ExerciseMechanic_EXERCISE_MECHANIC_UNSPECIFIED {
		return "", nil
	}
	return MechanicFromProto(e)
}

func fromProto[E protoEnum, T ~string](e E, prefix string, parse func(string) (T, error)) (T, error) {
	if e == 0 {
		var zero T
		return zero, fmt.Errorf("%w: %sUNSPECIFIED is not allowed", domain.ErrInvalidExercise, prefix)
	}
	// An unknown number stringifies to its digits and fails parse.
	return parse(strings.TrimPrefix(e.String(), prefix))
}

func listFromProto[E protoEnum, T ~string](es []E, convert func(E) (T, error)) ([]T, error) {
	if len(es) == 0 {
		return nil, nil
	}
	out := make([]T, len(es))
	for i, e := range es {
		v, err := convert(e)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}
