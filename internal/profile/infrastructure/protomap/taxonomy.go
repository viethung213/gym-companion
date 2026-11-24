// Package protomap maps profile taxonomy value objects to and from the
// contract enums. A proto value name is the enum prefix plus the domain code.
package protomap

import (
	"fmt"
	"strings"

	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

const (
	prefixJointRegion    = "JOINT_REGION_"
	prefixMuscleGroup    = "MUSCLE_GROUP_"
	prefixEquipment      = "EQUIPMENT_"
	prefixInjurySeverity = "INJURY_SEVERITY_"
	prefixBodySide       = "BODY_SIDE_"
)

type protoEnum interface {
	~int32
	String() string
}

func JointRegionToProto(v vo.JointRegion) exercisemsg.JointRegion {
	return exercisemsg.JointRegion(exercisemsg.JointRegion_value[prefixJointRegion+string(v)])
}

// JointRegionFromProto rejects UNSPECIFIED and unknown values.
func JointRegionFromProto(e exercisemsg.JointRegion) (vo.JointRegion, error) {
	return fromProto(e, prefixJointRegion, vo.ParseJointRegion)
}

// MuscleGroupToProto maps the empty MuscleGroup to UNSPECIFIED.
func MuscleGroupToProto(v vo.MuscleGroup) exercisemsg.MuscleGroup {
	return exercisemsg.MuscleGroup(exercisemsg.MuscleGroup_value[prefixMuscleGroup+string(v)])
}

func MuscleGroupFromProto(e exercisemsg.MuscleGroup) (vo.MuscleGroup, error) {
	return fromProto(e, prefixMuscleGroup, vo.ParseMuscleGroup)
}

// OptionalMuscleGroupFromProto maps UNSPECIFIED to the empty MuscleGroup.
func OptionalMuscleGroupFromProto(e exercisemsg.MuscleGroup) (vo.MuscleGroup, error) {
	if e == exercisemsg.MuscleGroup_MUSCLE_GROUP_UNSPECIFIED {
		return "", nil
	}
	return MuscleGroupFromProto(e)
}

func MuscleGroupsToProto(vs []vo.MuscleGroup) []exercisemsg.MuscleGroup {
	return listToProto(vs, MuscleGroupToProto)
}

func MuscleGroupsFromProto(es []exercisemsg.MuscleGroup) ([]vo.MuscleGroup, error) {
	return listFromProto(es, MuscleGroupFromProto)
}

func EquipmentToProto(v vo.Equipment) exercisemsg.Equipment {
	return exercisemsg.Equipment(exercisemsg.Equipment_value[prefixEquipment+string(v)])
}

func EquipmentFromProto(e exercisemsg.Equipment) (vo.Equipment, error) {
	return fromProto(e, prefixEquipment, vo.ParseEquipment)
}

func EquipmentListToProto(vs []vo.Equipment) []exercisemsg.Equipment {
	return listToProto(vs, EquipmentToProto)
}

func EquipmentListFromProto(es []exercisemsg.Equipment) ([]vo.Equipment, error) {
	return listFromProto(es, EquipmentFromProto)
}

func InjurySeverityToProto(v vo.InjurySeverity) exercisemsg.InjurySeverity {
	return exercisemsg.InjurySeverity(exercisemsg.InjurySeverity_value[prefixInjurySeverity+string(v)])
}

func InjurySeverityFromProto(e exercisemsg.InjurySeverity) (vo.InjurySeverity, error) {
	return fromProto(e, prefixInjurySeverity, vo.ParseInjurySeverity)
}

func BodySideToProto(v vo.BodySide) exercisemsg.BodySide {
	return exercisemsg.BodySide(exercisemsg.BodySide_value[prefixBodySide+string(v)])
}

// BodySideFromProto maps UNSPECIFIED to BOTH, the default side.
func BodySideFromProto(e exercisemsg.BodySide) (vo.BodySide, error) {
	if e == exercisemsg.BodySide_BODY_SIDE_UNSPECIFIED {
		return vo.BodySideBoth, nil
	}
	return fromProto(e, prefixBodySide, vo.ParseBodySide)
}

func fromProto[E protoEnum, T ~string](e E, prefix string, parse func(string) (T, error)) (T, error) {
	if e == 0 {
		var zero T
		return zero, fmt.Errorf("%w: %sUNSPECIFIED is not allowed", derror.ErrInvalidTaxonomy, prefix)
	}
	// An unknown number stringifies to its digits and fails parse.
	return parse(strings.TrimPrefix(e.String(), prefix))
}

func listToProto[T ~string, E protoEnum](vs []T, convert func(T) E) []E {
	if len(vs) == 0 {
		return nil
	}
	out := make([]E, len(vs))
	for i, v := range vs {
		out[i] = convert(v)
	}
	return out
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
