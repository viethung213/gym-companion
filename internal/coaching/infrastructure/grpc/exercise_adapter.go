package grpc

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	exercisesvc "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/service"
	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
)

// ExerciseAdapter implements port.ExerciseQueryService using gRPC to call ExerciseService.
type ExerciseAdapter struct {
	client exercisesvc.ExerciseServiceClient
}

var _ port.ExerciseQueryService = (*ExerciseAdapter)(nil)

// NewExerciseAdapter creates a new ExerciseAdapter with the provided gRPC client.
func NewExerciseAdapter(client exercisesvc.ExerciseServiceClient) *ExerciseAdapter {
	return &ExerciseAdapter{client: client}
}

// SearchExercises calls ExerciseService.SearchExercises over gRPC with injury filters.
func (a *ExerciseAdapter) SearchExercises(ctx context.Context, filters port.ExerciseSearchFilters) ([]port.ExerciseInfo, error) {
	req := &exercisemsg.SearchExercisesRequest{
		AvoidInjuryAreas: filters.AvoidInjuryAreas,
		Limit:            50, // ponytail: default limit per muscle group query
	}

	if len(filters.TargetMuscleGroups) > 0 {
		req.Keyword = filters.TargetMuscleGroups[0]
	}

	resp, err := a.client.SearchExercises(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("grpc search exercises: %w", err)
	}

	if resp == nil || len(resp.Exercises) == 0 {
		return nil, nil
	}

	results := make([]port.ExerciseInfo, len(resp.Exercises))
	for i, ex := range resp.Exercises {
		results[i] = port.ExerciseInfo{
			ID:              ex.Id,
			Name:            ex.Name,
			Category:        "Compound", // ponytail: fallback category if metadata not set
			PrimaryMuscle:   ex.TargetMuscleId,
			EquipmentNeeded: ex.EquipmentId,
		}
	}

	return results, nil
}
