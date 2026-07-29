package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
	"github.com/viethung213/gym-companion/internal/profile/domain/repository"
)

type GetInjuryHistoryQuery struct {
	UserID string
}

type GetInjuryHistoryHandler struct {
	repo repository.UserProfileRepository
}

func NewGetInjuryHistoryHandler(repo repository.UserProfileRepository) *GetInjuryHistoryHandler {
	return &GetInjuryHistoryHandler{repo: repo}
}

func (h *GetInjuryHistoryHandler) Handle(ctx context.Context, q GetInjuryHistoryQuery) ([]*entity.Injury, error) {
	injuries, err := h.repo.FindInjuryHistory(ctx, q.UserID)
	if err != nil {
		return nil, fmt.Errorf("get injury history for user %s: %w", q.UserID, err)
	}
	return injuries, nil
}
