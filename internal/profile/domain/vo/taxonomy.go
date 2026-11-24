package vo

import (
	"fmt"

	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
)

// Taxonomy value objects hold the short codes of the shared vocabulary
// (docs/05 §4). Parse functions reject unknown codes with ErrInvalidTaxonomy.

type JointRegion string

const (
	JointRegionNeck      JointRegion = "NECK"
	JointRegionShoulder  JointRegion = "SHOULDER"
	JointRegionElbow     JointRegion = "ELBOW"
	JointRegionWristHand JointRegion = "WRIST_HAND"
	JointRegionUpperBack JointRegion = "UPPER_BACK"
	JointRegionLowerBack JointRegion = "LOWER_BACK"
	JointRegionChest     JointRegion = "CHEST"
	JointRegionHip       JointRegion = "HIP"
	JointRegionKnee      JointRegion = "KNEE"
	JointRegionAnkleFoot JointRegion = "ANKLE_FOOT"
)

// AllJointRegions returns every JointRegion.
func AllJointRegions() []JointRegion {
	return []JointRegion{
		JointRegionNeck, JointRegionShoulder, JointRegionElbow, JointRegionWristHand,
		JointRegionUpperBack, JointRegionLowerBack, JointRegionChest, JointRegionHip,
		JointRegionKnee, JointRegionAnkleFoot,
	}
}

// ParseJointRegion validates a JointRegion code.
func ParseJointRegion(code string) (JointRegion, error) {
	return parseCode(code, "joint region", AllJointRegions())
}

type MuscleGroup string

const (
	MuscleGroupAbdominals MuscleGroup = "ABDOMINALS"
	MuscleGroupAbductors  MuscleGroup = "ABDUCTORS"
	MuscleGroupAdductors  MuscleGroup = "ADDUCTORS"
	MuscleGroupBiceps     MuscleGroup = "BICEPS"
	MuscleGroupCalves     MuscleGroup = "CALVES"
	MuscleGroupChest      MuscleGroup = "CHEST"
	MuscleGroupForearms   MuscleGroup = "FOREARMS"
	MuscleGroupGlutes     MuscleGroup = "GLUTES"
	MuscleGroupHamstrings MuscleGroup = "HAMSTRINGS"
	MuscleGroupLats       MuscleGroup = "LATS"
	MuscleGroupLowerBack  MuscleGroup = "LOWER_BACK"
	MuscleGroupMiddleBack MuscleGroup = "MIDDLE_BACK"
	MuscleGroupNeck       MuscleGroup = "NECK"
	MuscleGroupQuadriceps MuscleGroup = "QUADRICEPS"
	MuscleGroupShoulders  MuscleGroup = "SHOULDERS"
	MuscleGroupTraps      MuscleGroup = "TRAPS"
	MuscleGroupTriceps    MuscleGroup = "TRICEPS"
)

// AllMuscleGroups returns every MuscleGroup.
func AllMuscleGroups() []MuscleGroup {
	return []MuscleGroup{
		MuscleGroupAbdominals, MuscleGroupAbductors, MuscleGroupAdductors, MuscleGroupBiceps,
		MuscleGroupCalves, MuscleGroupChest, MuscleGroupForearms, MuscleGroupGlutes,
		MuscleGroupHamstrings, MuscleGroupLats, MuscleGroupLowerBack, MuscleGroupMiddleBack,
		MuscleGroupNeck, MuscleGroupQuadriceps, MuscleGroupShoulders, MuscleGroupTraps,
		MuscleGroupTriceps,
	}
}

// ParseMuscleGroup validates a MuscleGroup code.
func ParseMuscleGroup(code string) (MuscleGroup, error) {
	return parseCode(code, "muscle group", AllMuscleGroups())
}

type Equipment string

const (
	EquipmentBodyOnly     Equipment = "BODY_ONLY"
	EquipmentBarbell      Equipment = "BARBELL"
	EquipmentDumbbell     Equipment = "DUMBBELL"
	EquipmentKettlebells  Equipment = "KETTLEBELLS"
	EquipmentCable        Equipment = "CABLE"
	EquipmentMachine      Equipment = "MACHINE"
	EquipmentBands        Equipment = "BANDS"
	EquipmentEZCurlBar    Equipment = "EZ_CURL_BAR"
	EquipmentMedicineBall Equipment = "MEDICINE_BALL"
	EquipmentExerciseBall Equipment = "EXERCISE_BALL"
	EquipmentFoamRoll     Equipment = "FOAM_ROLL"
	EquipmentOther        Equipment = "OTHER"
)

// AllEquipment returns every Equipment.
func AllEquipment() []Equipment {
	return []Equipment{
		EquipmentBodyOnly, EquipmentBarbell, EquipmentDumbbell, EquipmentKettlebells,
		EquipmentCable, EquipmentMachine, EquipmentBands, EquipmentEZCurlBar,
		EquipmentMedicineBall, EquipmentExerciseBall, EquipmentFoamRoll, EquipmentOther,
	}
}

// ParseEquipment validates an Equipment code.
func ParseEquipment(code string) (Equipment, error) {
	return parseCode(code, "equipment", AllEquipment())
}

type InjurySeverity string

const (
	InjurySeverityMild     InjurySeverity = "MILD"
	InjurySeverityModerate InjurySeverity = "MODERATE"
	InjurySeveritySevere   InjurySeverity = "SEVERE"
)

// AllInjurySeverities returns every InjurySeverity.
func AllInjurySeverities() []InjurySeverity {
	return []InjurySeverity{InjurySeverityMild, InjurySeverityModerate, InjurySeveritySevere}
}

// ParseInjurySeverity validates an InjurySeverity code.
func ParseInjurySeverity(code string) (InjurySeverity, error) {
	return parseCode(code, "injury severity", AllInjurySeverities())
}

type BodySide string

const (
	BodySideLeft  BodySide = "LEFT"
	BodySideRight BodySide = "RIGHT"
	BodySideBoth  BodySide = "BOTH"
)

// AllBodySides returns every BodySide.
func AllBodySides() []BodySide {
	return []BodySide{BodySideLeft, BodySideRight, BodySideBoth}
}

// ParseBodySide validates a BodySide code.
func ParseBodySide(code string) (BodySide, error) {
	return parseCode(code, "body side", AllBodySides())
}

// ParseEquipmentList validates every code, keeping order and dropping duplicates.
func ParseEquipmentList(codes []string) ([]Equipment, error) {
	return parseList(codes, ParseEquipment)
}

// ParseMuscleGroupList validates every code, keeping order and dropping duplicates.
func ParseMuscleGroupList(codes []string) ([]MuscleGroup, error) {
	return parseList(codes, ParseMuscleGroup)
}

func parseCode[T ~string](code, field string, all []T) (T, error) {
	for _, v := range all {
		if string(v) == code {
			return v, nil
		}
	}

	var zero T
	return zero, fmt.Errorf("%w: unknown %s %q", derror.ErrInvalidTaxonomy, field, code)
}

func parseList[T ~string](codes []string, parse func(string) (T, error)) ([]T, error) {
	var out []T
	seen := make(map[T]bool, len(codes))
	for _, code := range codes {
		v, err := parse(code)
		if err != nil {
			return nil, err
		}
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}

	return out, nil
}

// Valid reports whether r is a known JointRegion.
func (r JointRegion) Valid() bool { return isKnown(r, AllJointRegions()) }

// Valid reports whether m is a known MuscleGroup.
func (m MuscleGroup) Valid() bool { return isKnown(m, AllMuscleGroups()) }

// Valid reports whether e is a known Equipment.
func (e Equipment) Valid() bool { return isKnown(e, AllEquipment()) }

// Valid reports whether s is a known InjurySeverity.
func (s InjurySeverity) Valid() bool { return isKnown(s, AllInjurySeverities()) }

// Valid reports whether s is a known BodySide.
func (s BodySide) Valid() bool { return isKnown(s, AllBodySides()) }

func isKnown[T ~string](v T, all []T) bool {
	for _, known := range all {
		if v == known {
			return true
		}
	}
	return false
}
