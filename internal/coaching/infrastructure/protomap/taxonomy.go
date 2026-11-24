// Package protomap maps contract enums to the coaching taxonomy. A proto value
// name is the enum prefix plus the domain code.
package protomap

import (
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
)

const (
	prefixMuscleGroup    = "MUSCLE_GROUP_"
	prefixEquipment      = "EQUIPMENT_"
	prefixCategory       = "EXERCISE_CATEGORY_"
	prefixLevel          = "EXERCISE_LEVEL_"
	prefixForce          = "EXERCISE_FORCE_"
	prefixMechanic       = "EXERCISE_MECHANIC_"
	prefixJointRegion    = "JOINT_REGION_"
	prefixInjurySeverity = "INJURY_SEVERITY_"
)

type protoEnum interface {
	~int32
	String() string
}

func MuscleGroup(e exercisemsg.MuscleGroup) (taxonomy.MuscleGroup, error) {
	return fromProto(e, prefixMuscleGroup, taxonomy.ParseMuscleGroup)
}

// OptionalMuscleGroup maps UNSPECIFIED to the empty MuscleGroup.
func OptionalMuscleGroup(e exercisemsg.MuscleGroup) (taxonomy.MuscleGroup, error) {
	if e == exercisemsg.MuscleGroup_MUSCLE_GROUP_UNSPECIFIED {
		return "", nil
	}
	return MuscleGroup(e)
}

func MuscleGroups(es []exercisemsg.MuscleGroup) ([]taxonomy.MuscleGroup, error) {
	return listFromProto(es, MuscleGroup)
}

func Equipment(e exercisemsg.Equipment) (taxonomy.Equipment, error) {
	return fromProto(e, prefixEquipment, taxonomy.ParseEquipment)
}

func EquipmentList(es []exercisemsg.Equipment) ([]taxonomy.Equipment, error) {
	return listFromProto(es, Equipment)
}

func Category(e exercisemsg.ExerciseCategory) (taxonomy.Category, error) {
	return fromProto(e, prefixCategory, taxonomy.ParseCategory)
}

func Level(e exercisemsg.ExerciseLevel) (taxonomy.Level, error) {
	return fromProto(e, prefixLevel, taxonomy.ParseLevel)
}

// OptionalForce maps UNSPECIFIED (no value in the source) to the empty Force.
func OptionalForce(e exercisemsg.ExerciseForce) (taxonomy.Force, error) {
	if e == exercisemsg.ExerciseForce_EXERCISE_FORCE_UNSPECIFIED {
		return "", nil
	}
	return fromProto(e, prefixForce, taxonomy.ParseForce)
}

// OptionalMechanic maps UNSPECIFIED (no value in the source) to the empty Mechanic.
func OptionalMechanic(e exercisemsg.ExerciseMechanic) (taxonomy.Mechanic, error) {
	if e == exercisemsg.ExerciseMechanic_EXERCISE_MECHANIC_UNSPECIFIED {
		return "", nil
	}
	return fromProto(e, prefixMechanic, taxonomy.ParseMechanic)
}

func JointRegion(e exercisemsg.JointRegion) (taxonomy.JointRegion, error) {
	return fromProto(e, prefixJointRegion, taxonomy.ParseJointRegion)
}

func InjurySeverity(e exercisemsg.InjurySeverity) (taxonomy.InjurySeverity, error) {
	return fromProto(e, prefixInjurySeverity, taxonomy.ParseInjurySeverity)
}

// MuscleGroupToProto maps a coaching code back to the contract enum.
func MuscleGroupToProto(v taxonomy.MuscleGroup) exercisemsg.MuscleGroup {
	return exercisemsg.MuscleGroup(exercisemsg.MuscleGroup_value[prefixMuscleGroup+string(v)])
}

// EquipmentToProto maps a coaching code back to the contract enum.
func EquipmentToProto(v taxonomy.Equipment) exercisemsg.Equipment {
	return exercisemsg.Equipment(exercisemsg.Equipment_value[prefixEquipment+string(v)])
}

// CategoryToProto maps a coaching code back to the contract enum.
func CategoryToProto(v taxonomy.Category) exercisemsg.ExerciseCategory {
	return exercisemsg.ExerciseCategory(exercisemsg.ExerciseCategory_value[prefixCategory+string(v)])
}

// LevelToProto maps a coaching code back to the contract enum.
func LevelToProto(v taxonomy.Level) exercisemsg.ExerciseLevel {
	return exercisemsg.ExerciseLevel(exercisemsg.ExerciseLevel_value[prefixLevel+string(v)])
}

func fromProto[E protoEnum, T ~string](e E, prefix string, parse func(string) (T, error)) (T, error) {
	if e == 0 {
		var zero T
		return zero, fmt.Errorf("%w: %sUNSPECIFIED", taxonomy.ErrUnknownValue, prefix)
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
