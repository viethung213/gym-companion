package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/profile/domain/repository"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

type GetBodyMetricsHistoryQuery struct {
	UserID string
}

type GetBodyMetricsHistoryHandler struct {
	repo repository.UserProfileRepository
}

func NewGetBodyMetricsHistoryHandler(repo repository.UserProfileRepository) *GetBodyMetricsHistoryHandler {
	return &GetBodyMetricsHistoryHandler{repo: repo}
}

func (h *GetBodyMetricsHistoryHandler) Handle(ctx context.Context, q GetBodyMetricsHistoryQuery) ([]vo.PeriodicMetric, error) {
	metrics, err := h.repo.FindBodyMetricsHistory(ctx, q.UserID)
	if err != nil {
		return nil, fmt.Errorf("get body metrics history for user %s: %w", q.UserID, err)
	}
	return metrics, nil
}
