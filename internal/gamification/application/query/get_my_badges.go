package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
)

// GetMyBadgesQuery defines parameters to retrieve unlocked achievements.
type GetMyBadgesQuery struct {
	UserID string
}

// GetMyBadgesResult contains the list of unlocked badges.
type GetMyBadgesResult struct {
	Badges []*aggregate.UserBadge
}

// GetMyBadgesHandler handles GetMyBadgesQuery.
type GetMyBadgesHandler struct {
	badgeRepo repository.BadgeRepository
}

// NewGetMyBadgesHandler constructs GetMyBadgesHandler.
func NewGetMyBadgesHandler(badgeRepo repository.BadgeRepository) *GetMyBadgesHandler {
	return &GetMyBadgesHandler{badgeRepo: badgeRepo}
}

// Handle fetches badges unlocked by the user.
func (h *GetMyBadgesHandler) Handle(ctx context.Context, q GetMyBadgesQuery) (*GetMyBadgesResult, error) {
	badges, err := h.badgeRepo.FindByUserID(ctx, q.UserID)
	if err != nil {
		return nil, fmt.Errorf("find user badges: %w", err)
	}

	return &GetMyBadgesResult{
		Badges: badges,
	}, nil
}
