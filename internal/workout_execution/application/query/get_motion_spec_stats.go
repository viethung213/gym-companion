package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
)

type GetMotionSpecificationStatsQueryResult struct {
	TotalExercises   int
	ActivePoseRules  int
	ActiveVoiceFiles int
	ReadyAiSpecs     int
}

type GetMotionSpecificationStatsQueryHandler struct {
	motionRepo repository.MotionSpecificationRepository
}

func NewGetMotionSpecificationStatsQueryHandler(
	motionRepo repository.MotionSpecificationRepository,
) *GetMotionSpecificationStatsQueryHandler {
	return &GetMotionSpecificationStatsQueryHandler{
		motionRepo: motionRepo,
	}
}

func (h *GetMotionSpecificationStatsQueryHandler) Handle(
	ctx context.Context,
) (*GetMotionSpecificationStatsQueryResult, error) {
	total, rules, voice, ready, err := h.motionRepo.GetStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("get motion spec stats query: %w", err)
	}

	return &GetMotionSpecificationStatsQueryResult{
		TotalExercises:   total,
		ActivePoseRules:  rules,
		ActiveVoiceFiles: voice,
		ReadyAiSpecs:     ready,
	}, nil
}
