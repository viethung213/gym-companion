package query

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
)

// GetMyXpQuery defines query parameters for fetching personal XP and level progression.
type GetMyXpQuery struct {
	UserID string
}

// GetMyXpResult contains the user's XP, level, and next-level progress.
type GetMyXpResult struct {
	Xp                 int64
	Level              int32
	XpToNextLevel      int64
	ProgressPercentage float64
	UpdatedAt          time.Time
}

// GetMyXpHandler handles GetMyXpQuery.
type GetMyXpHandler struct {
	userXpRepo repository.UserXpRepository
}

// NewGetMyXpHandler constructs GetMyXpHandler.
func NewGetMyXpHandler(userXpRepo repository.UserXpRepository) *GetMyXpHandler {
	return &GetMyXpHandler{userXpRepo: userXpRepo}
}

// Handle fetches the user XP state or returns default level 1 values if not found.
func (h *GetMyXpHandler) Handle(ctx context.Context, q GetMyXpQuery) (*GetMyXpResult, error) {
	userXp, err := h.userXpRepo.FindByID(ctx, q.UserID)
	if err != nil {
		return nil, fmt.Errorf("find user xp: %w", err)
	}

	if userXp == nil {
		return &GetMyXpResult{
			Xp:                 0,
			Level:              1,
			XpToNextLevel:      vo.XpToNextLevel(0),
			ProgressPercentage: vo.ProgressPercentage(0),
			UpdatedAt:          time.Now().UTC(),
		}, nil
	}

	xp := userXp.Xp()
	level := userXp.Level()

	return &GetMyXpResult{
		Xp:                 xp,
		Level:              level,
		XpToNextLevel:      vo.XpToNextLevel(xp),
		ProgressPercentage: vo.ProgressPercentage(xp),
		UpdatedAt:          userXp.UpdatedAt(),
	}, nil
}
