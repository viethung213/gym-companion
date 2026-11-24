package adk

import (
	"testing"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
)

// TestCharacterization_F3_KneeInjuryExcludesSquat pins finding F3 of
// docs/05_exercise_catalog_refactor_spec.md §3: filterEligibleExercises used
// to compare the injury string with the muscle / body-part name via EqualFold,
// so a "Knee" injury matched neither "Quadriceps" nor "Upper Legs" and the
// squat stayed eligible. Per §4 KNEE covers QUADRICEPS, so the squat must be
// excluded and the unrelated exercise kept.
func TestCharacterization_F3_KneeInjuryExcludesSquat(t *testing.T) {
	all := []port.Exercise{
		{
			ExerciseID:       "squat",
			Name:             "Barbell Squat",
			PrimaryMuscle:    taxonomy.MuscleGroupQuadriceps,
			SecondaryMuscles: []taxonomy.MuscleGroup{taxonomy.MuscleGroupGlutes, taxonomy.MuscleGroupHamstrings},
			Equipment:        taxonomy.EquipmentBarbell,
			Category:         taxonomy.CategoryStrength,
			Level:            taxonomy.LevelIntermediate,
		},
		{
			ExerciseID:       "bench",
			Name:             "Barbell Bench Press - Medium Grip",
			PrimaryMuscle:    taxonomy.MuscleGroupChest,
			SecondaryMuscles: []taxonomy.MuscleGroup{taxonomy.MuscleGroupShoulders, taxonomy.MuscleGroupTriceps},
			Equipment:        taxonomy.EquipmentBarbell,
			Category:         taxonomy.CategoryStrength,
			Level:            taxonomy.LevelIntermediate,
		},
	}
	profile := port.Profile{
		ExperienceLevel:    "INTERMEDIATE",
		AvailableEquipment: []taxonomy.Equipment{taxonomy.EquipmentBarbell},
		ActiveInjuries: []port.InjuryStatus{
			{Region: taxonomy.JointRegionKnee, Severity: taxonomy.InjurySeverityModerate},
		},
	}

	athlete := profile.Athlete()
	got := filterEligibleExercises(all, &athlete)

	gotIDs := make([]string, 0, len(got))
	for i := range got {
		gotIDs = append(gotIDs, got[i].exercise.ExerciseID)
	}
	if len(gotIDs) != 1 || gotIDs[0] != "bench" {
		t.Errorf("got eligible %v, want [bench] (squat excluded by KNEE injury)", gotIDs)
	}
}
