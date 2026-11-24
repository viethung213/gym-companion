package persistence

import (
	"reflect"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/exercise/domain"
)

func TestMappingExerciseRecord_RoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		give domain.Info
	}{
		{
			name: "all fields",
			give: domain.Info{
				ID:                 "ex-123",
				Slug:               "Plank",
				Name:               "Plank",
				PrimaryMuscle:      domain.MuscleGroupAbdominals,
				SecondaryMuscles:   []domain.MuscleGroup{domain.MuscleGroupGlutes, domain.MuscleGroupLowerBack},
				Equipment:          domain.EquipmentBodyOnly,
				Category:           domain.CategoryStrength,
				Level:              domain.LevelBeginner,
				Force:              domain.ForceStatic,
				Mechanic:           domain.MechanicIsolation,
				Instructions:       []string{"Hold position"},
				Images:             []string{"Plank/0.jpg", "Plank/1.jpg"},
				VideoURL:           "https://example.com/video.mp4",
				MeasurementType:    domain.MeasurementTypeDuration,
				DefaultRestSeconds: 45,
				MotionSlug:         "plank",
				HasAISupported:     true,
				Source:             domain.SourceFreeExerciseDB,
				Status:             domain.StatusActive,
				CreatedAt:          now,
				UpdatedAt:          now,
			},
		},
		{
			name: "nullable fields empty",
			give: domain.Info{
				ID:              "ex-124",
				Slug:            "Arm_Circles",
				Name:            "Arm Circles",
				PrimaryMuscle:   domain.MuscleGroupShoulders,
				Equipment:       domain.EquipmentBodyOnly,
				Category:        domain.CategoryStretching,
				Level:           domain.LevelBeginner,
				MeasurementType: domain.MeasurementTypeDuration,
				Source:          domain.SourceCustom,
				Status:          domain.StatusDraft,
				CreatedAt:       now,
				UpdatedAt:       now,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			record := newExerciseRecord(&tt.give)

			if record.SecondaryMuscles == nil || record.Instructions == nil || record.Images == nil {
				t.Error("TEXT[] columns are NOT NULL; record arrays must not be nil")
			}
			if tt.give.Force == "" && record.Force != nil {
				t.Errorf("empty force must map to NULL, got %q", *record.Force)
			}
			if tt.give.MotionSlug == "" && record.MotionSlug != nil {
				t.Errorf("empty motion slug must map to NULL, got %q", *record.MotionSlug)
			}

			if got := record.toDomainInfo(); !reflect.DeepEqual(got, tt.give) {
				t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, tt.give)
			}
		})
	}
}
