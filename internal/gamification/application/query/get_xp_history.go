package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
)

// GetXpHistoryQuery defines parameters for retrieving paginated XP transactions.
type GetXpHistoryQuery struct {
	UserID string
	Limit  int
	Offset int
}

// GetXpHistoryResult holds the paginated list of XP transactions.
type GetXpHistoryResult struct {
	Entries []*entity.XpHistoryEntry
	Total   int64
}

// GetXpHistoryHandler handles GetXpHistoryQuery.
type GetXpHistoryHandler struct {
	userXpRepo repository.UserXpRepository
}

// NewGetXpHistoryHandler constructs GetXpHistoryHandler.
func NewGetXpHistoryHandler(userXpRepo repository.UserXpRepository) *GetXpHistoryHandler {
	return &GetXpHistoryHandler{userXpRepo: userXpRepo}
}

// Handle fetches the paginated XP history.
func (h *GetXpHistoryHandler) Handle(ctx context.Context, q GetXpHistoryQuery) (*GetXpHistoryResult, error) {
	entries, total, err := h.userXpRepo.GetHistory(ctx, q.UserID, q.Limit, q.Offset)
	if err != nil {
		return nil, fmt.Errorf("get xp history: %w", err)
	}

	return &GetXpHistoryResult{
		Entries: entries,
		Total:   total,
	}, nil
}
