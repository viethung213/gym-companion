package seeder

import (
	"testing"

	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
)

func ptr(s string) *string { return &s }

func TestNewCreateExerciseRequest(t *testing.T) {
	base := func() upstreamExercise {
		return upstreamExercise{
			ID:               "Barbell_Squat",
			Name:             " Barbell Squat ",
			Force:            ptr("push"),
			Level:            "beginner",
			Mechanic:         ptr("compound"),
			Equipment:        ptr("barbell"),
			PrimaryMuscles:   []string{"quadriceps"},
			SecondaryMuscles: []string{"calves", "glutes"},
			Instructions:     []string{"Squat."},
			Category:         "strength",
			Images:           []string{"Barbell_Squat/0.jpg", "Barbell_Squat/1.jpg"},
		}
	}

	tests := []struct {
		name  string
		give  func() upstreamExercise
		check func(t *testing.T, got *exercisemsg.CreateExerciseRequest)
	}{
		{
			name: "values kept as in source",
			give: base,
			check: func(t *testing.T, got *exercisemsg.CreateExerciseRequest) {
				if got.GetSlug() != "Barbell_Squat" || got.GetName() != "Barbell Squat" {
					t.Errorf("slug/name = %q/%q", got.GetSlug(), got.GetName())
				}
				if got.GetPrimaryMuscle() != exercisemsg.MuscleGroup_MUSCLE_GROUP_QUADRICEPS ||
					got.GetEquipment() != exercisemsg.Equipment_EQUIPMENT_BARBELL ||
					got.GetCategory() != exercisemsg.ExerciseCategory_EXERCISE_CATEGORY_STRENGTH ||
					got.GetLevel() != exercisemsg.ExerciseLevel_EXERCISE_LEVEL_BEGINNER ||
					got.GetForce() != exercisemsg.ExerciseForce_EXERCISE_FORCE_PUSH ||
					got.GetMechanic() != exercisemsg.ExerciseMechanic_EXERCISE_MECHANIC_COMPOUND {
					t.Errorf("unexpected taxonomy: %v", got)
				}
				if len(got.GetSecondaryMuscles()) != 2 || len(got.GetImages()) != 2 {
					t.Errorf("secondary/images = %v/%v", got.GetSecondaryMuscles(), got.GetImages())
				}
				if got.GetMeasurementType() != exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_REPS ||
					got.GetDefaultRestSeconds() != compoundRestSeconds ||
					got.GetStatus() != exercisemsg.ExerciseStatus_EXERCISE_STATUS_ACTIVE {
					t.Errorf("measurement/rest/status = %v/%d/%v",
						got.GetMeasurementType(), got.GetDefaultRestSeconds(), got.GetStatus())
				}
			},
		},
		{
			name: "null equipment becomes BODY_ONLY",
			give: func() upstreamExercise {
				r := base()
				r.Equipment = nil
				return r
			},
			check: func(t *testing.T, got *exercisemsg.CreateExerciseRequest) {
				if got.GetEquipment() != exercisemsg.Equipment_EQUIPMENT_BODY_ONLY {
					t.Errorf("got %v, want BODY_ONLY", got.GetEquipment())
				}
			},
		},
		{
			name: "extra primary muscles move to secondary first",
			give: func() upstreamExercise {
				r := base()
				r.PrimaryMuscles = []string{"shoulders", "triceps"}
				r.SecondaryMuscles = []string{"lower back"}
				return r
			},
			check: func(t *testing.T, got *exercisemsg.CreateExerciseRequest) {
				want := []exercisemsg.MuscleGroup{
					exercisemsg.MuscleGroup_MUSCLE_GROUP_TRICEPS,
					exercisemsg.MuscleGroup_MUSCLE_GROUP_LOWER_BACK,
				}
				if got.GetPrimaryMuscle() != exercisemsg.MuscleGroup_MUSCLE_GROUP_SHOULDERS {
					t.Errorf("primary = %v, want SHOULDERS", got.GetPrimaryMuscle())
				}
				if len(got.GetSecondaryMuscles()) != 2 || got.GetSecondaryMuscles()[0] != want[0] ||
					got.GetSecondaryMuscles()[1] != want[1] {
					t.Errorf("secondary = %v, want %v", got.GetSecondaryMuscles(), want)
				}
			},
		},
		{
			name: "stretching is DURATION with 15s rest",
			give: func() upstreamExercise {
				r := base()
				r.Category = "stretching"
				r.Force = nil
				return r
			},
			check: func(t *testing.T, got *exercisemsg.CreateExerciseRequest) {
				if got.GetMeasurementType() != exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_DURATION ||
					got.GetDefaultRestSeconds() != stretchingRestSeconds {
					t.Errorf("measurement/rest = %v/%d", got.GetMeasurementType(), got.GetDefaultRestSeconds())
				}
				if got.GetForce() != exercisemsg.ExerciseForce_EXERCISE_FORCE_UNSPECIFIED {
					t.Errorf("null force = %v, want UNSPECIFIED", got.GetForce())
				}
			},
		},
		{
			name: "static force is DURATION",
			give: func() upstreamExercise {
				r := base()
				r.Force = ptr("static")
				r.Mechanic = ptr("isolation")
				return r
			},
			check: func(t *testing.T, got *exercisemsg.CreateExerciseRequest) {
				if got.GetMeasurementType() != exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_DURATION ||
					got.GetDefaultRestSeconds() != defaultRestSeconds {
					t.Errorf("measurement/rest = %v/%d", got.GetMeasurementType(), got.GetDefaultRestSeconds())
				}
			},
		},
		{
			name: "multi-word and hyphenated values",
			give: func() upstreamExercise {
				r := base()
				r.Equipment = ptr("e-z curl bar")
				r.Category = "olympic weightlifting"
				r.Mechanic = nil
				return r
			},
			check: func(t *testing.T, got *exercisemsg.CreateExerciseRequest) {
				if got.GetEquipment() != exercisemsg.Equipment_EQUIPMENT_EZ_CURL_BAR ||
					got.GetCategory() != exercisemsg.ExerciseCategory_EXERCISE_CATEGORY_OLYMPIC_WEIGHTLIFTING {
					t.Errorf("equipment/category = %v/%v", got.GetEquipment(), got.GetCategory())
				}
				if got.GetDefaultRestSeconds() != defaultRestSeconds {
					t.Errorf("rest = %d, want %d", got.GetDefaultRestSeconds(), defaultRestSeconds)
				}
			},
		},
		{
			name: "no images is DRAFT",
			give: func() upstreamExercise {
				r := base()
				r.Images = nil
				return r
			},
			check: func(t *testing.T, got *exercisemsg.CreateExerciseRequest) {
				if got.GetStatus() != exercisemsg.ExerciseStatus_EXERCISE_STATUS_DRAFT {
					t.Errorf("status = %v, want DRAFT", got.GetStatus())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := tt.give()
			got, err := newCreateExerciseRequest(&rec, "squat")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.GetMotionSlug() != "squat" {
				t.Errorf("motion slug = %q, want squat", got.GetMotionSlug())
			}
			tt.check(t, got)
		})
	}
}

func TestNewCreateExerciseRequest_RejectsUnknownValues(t *testing.T) {
	tests := []struct {
		name string
		give upstreamExercise
	}{
		{"no primary muscle", upstreamExercise{ID: "x", Level: "beginner", Category: "strength"}},
		{"unknown muscle", upstreamExercise{
			ID: "x", Level: "beginner", Category: "strength", PrimaryMuscles: []string{"wings"},
		}},
		{"unknown level", upstreamExercise{
			ID: "x", Level: "legend", Category: "strength", PrimaryMuscles: []string{"chest"},
		}},
		{"unknown equipment", upstreamExercise{
			ID: "x", Level: "beginner", Category: "strength", PrimaryMuscles: []string{"chest"},
			Equipment: ptr("rocket"),
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := newCreateExerciseRequest(&tt.give, ""); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
