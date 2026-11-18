package repository

import (
	"context"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/entity"
)

type WeeklyInsightRepository interface {
	Save(ctx context.Context, insight *entity.WeeklyInsight) error
	GetLatestByUserID(ctx context.Context, userID string) (*entity.WeeklyInsight, error)
	GetByUserIDAndStartDate(ctx context.Context, userID string, startDate time.Time) (*entity.WeeklyInsight, error)
}
