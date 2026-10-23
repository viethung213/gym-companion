package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
)

// GetGlobalLeaderboardQuery defines parameters for retrieving the top gymer leaderboard and personal standing.
type GetGlobalLeaderboardQuery struct {
	UserID string
	Limit  int
	Offset int
}

// GetGlobalLeaderboardResult holds the Top users and the caller's rank.
type GetGlobalLeaderboardResult struct {
	TopUsers []*entity.LeaderboardUser
	Total    int64
	MyRank   int64
	MyEntry  *entity.LeaderboardUser
}

// GetGlobalLeaderboardHandler handles GetGlobalLeaderboardQuery.
type GetGlobalLeaderboardHandler struct {
	leaderboardRepo repository.LeaderboardRepository
}

// NewGetGlobalLeaderboardHandler constructs GetGlobalLeaderboardHandler.
func NewGetGlobalLeaderboardHandler(leaderboardRepo repository.LeaderboardRepository) *GetGlobalLeaderboardHandler {
	return &GetGlobalLeaderboardHandler{leaderboardRepo: leaderboardRepo}
}

// Handle fetches the top gymer ranking and caller's standing in < 2ms.
func (h *GetGlobalLeaderboardHandler) Handle(
	ctx context.Context,
	q GetGlobalLeaderboardQuery,
) (*GetGlobalLeaderboardResult, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}

	topUsers, total, err := h.leaderboardRepo.GetGlobalTop(ctx, limit, q.Offset)
	if err != nil {
		return nil, fmt.Errorf("get global top leaderboard: %w", err)
	}

	var myRank int64
	var myEntry *entity.LeaderboardUser

	if q.UserID != "" {
		rank, entry, err := h.leaderboardRepo.GetUserRank(ctx, q.UserID)
		if err == nil {
			myRank = rank
			myEntry = entry
		}
	}

	return &GetGlobalLeaderboardResult{
		TopUsers: topUsers,
		Total:    total,
		MyRank:   myRank,
		MyEntry:  myEntry,
	}, nil
}
