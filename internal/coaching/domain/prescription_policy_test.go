package domain_test

import (
	"testing"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

func TestResolvePrescriptionSpec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		experienceLevel string
		category        string
		wantSets        int
		wantReps        int
	}{
		{
			name:            "beginner compound gets 3x10",
			experienceLevel: "beginner",
			category:        "Compound",
			wantSets:        3,
			wantReps:        10,
		},
		{
			name:            "beginner isolation gets 2x12",
			experienceLevel: "beginner",
			category:        "Isolation",
			wantSets:        2,
			wantReps:        12,
		},
		{
			name:            "intermediate compound gets 4x8",
			experienceLevel: "intermediate",
			category:        "Compound",
			wantSets:        4,
			wantReps:        8,
		},
		{
			name:            "advanced compound gets 5x6",
			experienceLevel: "advanced",
			category:        "Compound",
			wantSets:        5,
			wantReps:        6,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			spec := domain.ResolvePrescriptionSpec(tt.experienceLevel, tt.category)
			if got, want := spec.TargetSets, tt.wantSets; got != want {
				t.Errorf("TargetSets got = %d, want = %d", got, want)
			}
			if got, want := spec.TargetReps, tt.wantReps; got != want {
				t.Errorf("TargetReps got = %d, want = %d", got, want)
			}
		})
	}
}
