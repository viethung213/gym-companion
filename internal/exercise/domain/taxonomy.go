package domain

import "fmt"

// Taxonomy value objects hold the short codes of free-exercise-db
// (docs/05 §4). Parse functions reject unknown codes with ErrInvalidExercise.

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

type Category string

const (
	CategoryStrength             Category = "STRENGTH"
	CategoryStretching           Category = "STRETCHING"
	CategoryPlyometrics          Category = "PLYOMETRICS"
	CategoryPowerlifting         Category = "POWERLIFTING"
	CategoryOlympicWeightlifting Category = "OLYMPIC_WEIGHTLIFTING"
	CategoryStrongman            Category = "STRONGMAN"
	CategoryCardio               Category = "CARDIO"
)

// AllCategories returns every Category.
func AllCategories() []Category {
	return []Category{
		CategoryStrength, CategoryStretching, CategoryPlyometrics, CategoryPowerlifting,
		CategoryOlympicWeightlifting, CategoryStrongman, CategoryCardio,
	}
}

// ParseCategory validates a Category code.
func ParseCategory(code string) (Category, error) {
	return parseCode(code, "category", AllCategories())
}

type Level string

const (
	LevelBeginner     Level = "BEGINNER"
	LevelIntermediate Level = "INTERMEDIATE"
	LevelExpert       Level = "EXPERT"
)

// AllLevels returns every Level.
func AllLevels() []Level {
	return []Level{LevelBeginner, LevelIntermediate, LevelExpert}
}

// ParseLevel validates a Level code.
func ParseLevel(code string) (Level, error) {
	return parseCode(code, "level", AllLevels())
}

// Force is optional on an exercise; the empty value means "none".
type Force string

const (
	ForcePush   Force = "PUSH"
	ForcePull   Force = "PULL"
	ForceStatic Force = "STATIC"
)

// AllForces returns every Force.
func AllForces() []Force {
	return []Force{ForcePush, ForcePull, ForceStatic}
}

// ParseForce validates a Force code.
func ParseForce(code string) (Force, error) {
	return parseCode(code, "force", AllForces())
}

// Mechanic is optional on an exercise; the empty value means "none".
type Mechanic string

const (
	MechanicCompound  Mechanic = "COMPOUND"
	MechanicIsolation Mechanic = "ISOLATION"
)

// AllMechanics returns every Mechanic.
func AllMechanics() []Mechanic {
	return []Mechanic{MechanicCompound, MechanicIsolation}
}

// ParseMechanic validates a Mechanic code.
func ParseMechanic(code string) (Mechanic, error) {
	return parseCode(code, "mechanic", AllMechanics())
}

func parseCode[T ~string](code, field string, all []T) (T, error) {
	for _, v := range all {
		if string(v) == code {
			return v, nil
		}
	}

	var zero T
	return zero, fmt.Errorf("%w: unknown %s %q", ErrInvalidExercise, field, code)
}

func isKnown[T ~string](v T, all []T) bool {
	for _, known := range all {
		if v == known {
			return true
		}
	}
	return false
}
