package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/social/domain/repository"
)

type ListCommentsQuery struct {
	FeedItemID string
	PageSize   int
	Cursor     string
}

type ListCommentsResult struct {
	Comments   []*CommentItemDTO
	NextCursor string
	TotalCount int32
}

type ListCommentsHandler struct {
	interactionRepo  repository.InteractionRepository
	userSnapshotRepo repository.UserSnapshotRepository
}

func NewListCommentsHandler(
	interactionRepo repository.InteractionRepository,
	userSnapshotRepo repository.UserSnapshotRepository,
) *ListCommentsHandler {
	return &ListCommentsHandler{
		interactionRepo:  interactionRepo,
		userSnapshotRepo: userSnapshotRepo,
	}
}

func (h *ListCommentsHandler) Handle(ctx context.Context, q ListCommentsQuery) (*ListCommentsResult, error) {
	if q.PageSize <= 0 {
		q.PageSize = 20
	}

	comments, nextCursor, totalCount, err := h.interactionRepo.ListComments(ctx, q.FeedItemID, q.PageSize, q.Cursor)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}

	userIDs := make([]string, 0, len(comments))
	for _, c := range comments {
		userIDs = append(userIDs, c.UserID())
	}

	userInfoMap, _ := h.userSnapshotRepo.GetByIDs(ctx, userIDs)

	result := make([]*CommentItemDTO, 0, len(comments))
	for _, c := range comments {
		authorName := ""
		avatarURL := ""
		if u, ok := userInfoMap[c.UserID()]; ok && u != nil {
			authorName = u.FullName()
			avatarURL = u.AvatarURL()
		}

		result = append(result, &CommentItemDTO{
			ID:              c.ID(),
			UserID:          c.UserID(),
			AuthorName:      authorName,
			AuthorAvatarURL: avatarURL,
			FeedItemID:      c.FeedItemID(),
			ParentID:        c.ParentID(),
			Content:         c.Content(),
			CreatedAt:       c.CreatedAt(),
		})
	}

	return &ListCommentsResult{
		Comments:   result,
		NextCursor: nextCursor,
		TotalCount: totalCount,
	}, nil
}
