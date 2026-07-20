package exercise

import (
	"context"
	"errors"
	"fmt"
	"strings"

	coachingport "github.com/viethung213/gym-companion/internal/coaching/application/port"
	exerciseport "github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/application/query"
)

type Adapter struct {
	catalog *query.Catalog
}

var _ coachingport.ExerciseQueryService = (*Adapter)(nil)

func NewAdapter(catalog *query.Catalog) (*Adapter, error) {
	if catalog == nil {
		return nil, errors.New("exercise catalog is required")
	}

	return &Adapter{catalog: catalog}, nil
}

func (a *Adapter) SearchExercises(
	ctx context.Context,
	filters *coachingport.ExerciseSearchFilters,
) ([]coachingport.ExerciseInfo, error) {
	searchFilters := &exerciseport.SearchFilters{Limit: 50}
	for _, muscleGroup := range filters.TargetMuscleGroups {
		if !strings.EqualFold(strings.TrimSpace(muscleGroup), "FullBody") {
			searchFilters.TargetMuscleIDs = append(
				searchFilters.TargetMuscleIDs,
				muscleGroup,
			)
		}
	}
	searchFilters.AvoidMuscleIDs = append(
		[]string(nil),
		filters.AvoidInjuryAreas...,
	)

	exercises, err := a.catalog.Search(ctx, searchFilters)
	if err != nil {
		return nil, fmt.Errorf("query exercise catalog: %w", err)
	}

	results := make([]coachingport.ExerciseInfo, len(exercises))
	for i, current := range exercises {
		info := current.Info()
		results[i] = coachingport.ExerciseInfo{
			ID:              info.ID,
			Name:            info.Name,
			Category:        "Compound",
			PrimaryMuscle:   info.TargetMuscleID,
			EquipmentNeeded: info.EquipmentID,
		}
	}

	return results, nil
}
