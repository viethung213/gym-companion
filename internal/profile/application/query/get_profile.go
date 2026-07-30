package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/profile/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/profile/domain/repository"
)

type GetProfileQuery struct {
	UserID string
}

type GetProfileHandler struct {
	repo repository.UserProfileRepository
}

func NewGetProfileHandler(repo repository.UserProfileRepository) *GetProfileHandler {
	return &GetProfileHandler{repo: repo}
}

func (h *GetProfileHandler) Handle(ctx context.Context, q GetProfileQuery) (*aggregate.UserProfile, error) {
	profile, err := h.repo.FindByUserID(ctx, q.UserID)
	if err != nil {
		return nil, fmt.Errorf("get profile by user_id: %w", err)
	}
	return profile, nil
}
