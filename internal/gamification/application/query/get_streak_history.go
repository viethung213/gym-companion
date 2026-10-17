package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
)

// GetStreakHistoryQuery defines parameters for retrieving streak active days (heatmap).
type GetStreakHistoryQuery struct {
	UserID string
	Limit  int
	Offset int
}

// GetStreakHistoryResult contains the recorded active days.
type GetStreakHistoryResult struct {
	Entries []*entity.StreakHistoryEntry
	Total   int64
}

// GetStreakHistoryHandler handles GetStreakHistoryQuery.
type GetStreakHistoryHandler struct {
	userStreakRepo repository.UserStreakRepository
}

// NewGetStreakHistoryHandler constructs GetStreakHistoryHandler.
func NewGetStreakHistoryHandler(userStreakRepo repository.UserStreakRepository) *GetStreakHistoryHandler {
	return &GetStreakHistoryHandler{userStreakRepo: userStreakRepo}
}

// Handle fetches paginated streak activity entries.
func (h *GetStreakHistoryHandler) Handle(ctx context.Context, q GetStreakHistoryQuery) (*GetStreakHistoryResult, error) {
	entries, total, err := h.userStreakRepo.GetHistory(ctx, q.UserID, q.Limit, q.Offset)
	if err != nil {
		return nil, fmt.Errorf("get streak history: %w", err)
	}

	return &GetStreakHistoryResult{
		Entries: entries,
		Total:   total,
	}, nil
}
