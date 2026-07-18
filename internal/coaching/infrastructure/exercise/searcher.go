package exercise

import (
	"context"
	"fmt"
	"strings"

	coachingport "github.com/viethung213/gym-companion/internal/coaching/application/port"
	exerciseport "github.com/viethung213/gym-companion/internal/exercise/application/port"
	exercisequery "github.com/viethung213/gym-companion/internal/exercise/application/query"
)

var _ coachingport.ExerciseSearcher = (*Searcher)(nil)

type Searcher struct {
	handler *exercisequery.SearchExercisesHandler
}

func NewSearcher(handler *exercisequery.SearchExercisesHandler) *Searcher {
	return &Searcher{handler: handler}
}

func (s *Searcher) Search(
	ctx context.Context,
	criteria coachingport.ExerciseSearchCriteria,
) ([]coachingport.ExerciseCandidate, error) {
	if len(criteria.MuscleGroupIDs) == 0 || criteria.Limit <= 0 {
		return nil, nil
	}

	equipmentIDs := criteria.EquipmentIDs
	if len(equipmentIDs) == 0 {
		equipmentIDs = []string{""}
	}
	quota := (criteria.Limit + len(criteria.MuscleGroupIDs) - 1) / len(criteria.MuscleGroupIDs)
	candidates := make([]coachingport.ExerciseCandidate, 0, criteria.Limit)
	seen := make(map[string]struct{}, criteria.Limit)
	for _, muscleGroupID := range criteria.MuscleGroupIDs {
		groupCandidates, err := s.searchGroup(
			ctx,
			muscleGroupID,
			equipmentIDs,
			difficultyForExercise(criteria.Difficulty),
			quota,
		)
		if err != nil {
			return nil, err
		}
		for _, candidate := range groupCandidates {
			if _, exists := seen[candidate.ID]; exists {
				continue
			}
			seen[candidate.ID] = struct{}{}
			candidates = append(candidates, candidate)
			if len(candidates) == criteria.Limit {
				return candidates, nil
			}
		}
	}
	return candidates, nil
}

func (s *Searcher) searchGroup(
	ctx context.Context,
	muscleGroupID string,
	equipmentIDs []string,
	difficulty string,
	limit int,
) ([]coachingport.ExerciseCandidate, error) {
	candidates := make([]coachingport.ExerciseCandidate, 0, limit)
	for _, equipmentID := range equipmentIDs {
		remaining := limit - len(candidates)
		if remaining == 0 {
			break
		}
		items, err := s.search(ctx, &exerciseport.SearchFilters{
			BodyPartID:  muscleGroupID,
			EquipmentID: equipmentID,
			Difficulty:  difficulty,
			Limit:       int32(remaining),
		})
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			items, err = s.search(ctx, &exerciseport.SearchFilters{
				TargetMuscleID: muscleGroupID,
				EquipmentID:    equipmentID,
				Difficulty:     difficulty,
				Limit:          int32(remaining),
			})
			if err != nil {
				return nil, err
			}
		}
		candidates = append(candidates, items...)
	}
	return candidates, nil
}

func (s *Searcher) search(
	ctx context.Context,
	filters *exerciseport.SearchFilters,
) ([]coachingport.ExerciseCandidate, error) {
	exercises, err := s.handler.Handle(ctx, exercisequery.SearchExercisesQuery{Filters: filters})
	if err != nil {
		return nil, fmt.Errorf("search exercises application query: %w", err)
	}
	candidates := make([]coachingport.ExerciseCandidate, 0, len(exercises))
	for _, item := range exercises {
		info := item.Info()
		candidates = append(candidates, coachingport.ExerciseCandidate{
			ID:                 info.ID,
			Name:               info.Name,
			TargetMuscleID:     info.TargetMuscleID,
			EquipmentID:        info.EquipmentID,
			Difficulty:         info.Difficulty,
			DefaultRestSeconds: int(info.DefaultRestSeconds),
		})
	}
	return candidates, nil
}

func difficultyForExercise(experienceLevel string) string {
	if experienceLevel == "" {
		return ""
	}
	return strings.ToUpper(experienceLevel[:1]) + strings.ToLower(experienceLevel[1:])
}
