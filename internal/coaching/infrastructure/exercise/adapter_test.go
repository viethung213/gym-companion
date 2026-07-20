package exercise

import (
	"context"
	"slices"
	"testing"
	"time"

	coachingport "github.com/viethung213/gym-companion/internal/coaching/application/port"
	exerciseport "github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/application/query"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
)

type catalogRepository struct {
	exercises []*domain.Exercise
	filters   *exerciseport.SearchFilters
}

func (*catalogRepository) Save(
	context.Context,
	*domain.Exercise,
	*domain.Event,
) error {
	return nil
}

func (*catalogRepository) FindByID(
	context.Context,
	string,
) (*domain.Exercise, error) {
	return nil, domain.ErrExerciseNotFound
}

func (r *catalogRepository) SearchActive(
	_ context.Context,
	filters *exerciseport.SearchFilters,
) ([]*domain.Exercise, error) {
	r.filters = filters
	return r.exercises, nil
}

func (*catalogRepository) GetMetadata(context.Context) (exerciseport.Metadata, error) {
	return exerciseport.Metadata{}, nil
}

func TestAdapterSearchExercises(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
	exercise, err := domain.RehydrateExercise(domain.Info{
		ID:             "exercise-1",
		Name:           "Squat",
		BodyPartID:     "legs",
		EquipmentID:    "barbell",
		TargetMuscleID: "quads",
		Status:         domain.StatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if err != nil {
		t.Fatalf("rehydrate exercise: %v", err)
	}

	repository := &catalogRepository{
		exercises: []*domain.Exercise{exercise},
	}
	catalog := query.NewCatalog(repository)
	adapter, err := NewAdapter(catalog)
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}

	got, err := adapter.SearchExercises(
		context.Background(),
		&coachingport.ExerciseSearchFilters{
			TargetMuscleGroups: []string{"quads", "hamstrings"},
			AvoidInjuryAreas:   []string{"lower-back"},
		},
	)
	if err != nil {
		t.Fatalf("search exercises: %v", err)
	}
	if len(got) != 1 || got[0].ID != "exercise-1" {
		t.Errorf("exercises got = %#v", got)
	}
	if !slices.Equal(
		repository.filters.TargetMuscleIDs,
		[]string{"quads", "hamstrings"},
	) {
		t.Errorf("target muscle IDs got = %v", repository.filters.TargetMuscleIDs)
	}
	if !slices.Equal(repository.filters.AvoidMuscleIDs, []string{"lower-back"}) {
		t.Errorf("avoided muscle IDs got = %v", repository.filters.AvoidMuscleIDs)
	}
}

func TestNewAdapterRejectsNilCatalog(t *testing.T) {
	t.Parallel()

	if _, err := NewAdapter(nil); err == nil {
		t.Fatal("nil catalog unexpectedly accepted")
	}
}
