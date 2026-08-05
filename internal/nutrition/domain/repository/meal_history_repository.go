package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
)

type MealHistoryRepository interface {
	FindByUserID(ctx context.Context, userID string) (*aggregate.MealHistory, error)
	Save(ctx context.Context, history *aggregate.MealHistory) error
}
