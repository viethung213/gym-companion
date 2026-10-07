package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/social/domain/repository"
)

type ListReactionsQuery struct {
	FeedItemID string
}

type ListReactionsResult struct {
	Reactions  []*ReactionItemDTO
	TotalCount int32
}

type ListReactionsHandler struct {
	interactionRepo  repository.InteractionRepository
	userSnapshotRepo repository.UserSnapshotRepository
}

func NewListReactionsHandler(
	interactionRepo repository.InteractionRepository,
	userSnapshotRepo repository.UserSnapshotRepository,
) *ListReactionsHandler {
	return &ListReactionsHandler{
		interactionRepo:  interactionRepo,
		userSnapshotRepo: userSnapshotRepo,
	}
}

func (h *ListReactionsHandler) Handle(ctx context.Context, q ListReactionsQuery) (*ListReactionsResult, error) {
	reactions, err := h.interactionRepo.GetReactionsByFeedItem(ctx, q.FeedItemID)
	if err != nil {
		return nil, fmt.Errorf("list reactions: %w", err)
	}

	userIDs := make([]string, 0, len(reactions))
	for _, r := range reactions {
		userIDs = append(userIDs, r.UserID())
	}

	userInfoMap, _ := h.userSnapshotRepo.GetByIDs(ctx, userIDs)

	result := make([]*ReactionItemDTO, 0, len(reactions))
	for _, r := range reactions {
		authorName := ""
		avatarURL := ""
		if u, ok := userInfoMap[r.UserID()]; ok && u != nil {
			authorName = u.FullName()
			avatarURL = u.AvatarURL()
		}

		result = append(result, &ReactionItemDTO{
			UserID:          r.UserID(),
			AuthorName:      authorName,
			AuthorAvatarURL: avatarURL,
			ReactionType:    r.ReactionType().String(),
			CreatedAt:       r.CreatedAt(),
		})
	}

	return &ListReactionsResult{
		Reactions:  result,
		TotalCount: int32(len(result)),
	}, nil
}
