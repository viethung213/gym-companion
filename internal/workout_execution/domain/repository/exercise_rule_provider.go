package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

// ExerciseRuleProvider defines the port for fetching exercise motion rules.
type ExerciseRuleProvider interface {
	GetRule(ctx context.Context, exerciseID string) (*vo.ExerciseMotionRule, error)
}
