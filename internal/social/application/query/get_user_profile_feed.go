package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/social/domain/repository"
)

type GetUserProfileFeedQuery struct {
	TargetUserID  string
	CurrentUserID string
	PageSize      int
	Cursor        string
}

type GetUserProfileFeedResult struct {
	Items      []*FeedItemDTO
	NextCursor string
}

type GetUserProfileFeedHandler struct {
	feedItemRepo     repository.FeedItemRepository
	userSnapshotRepo repository.UserSnapshotRepository
}

func NewGetUserProfileFeedHandler(
	feedItemRepo repository.FeedItemRepository,
	userSnapshotRepo repository.UserSnapshotRepository,
) *GetUserProfileFeedHandler {
	return &GetUserProfileFeedHandler{
		feedItemRepo:     feedItemRepo,
		userSnapshotRepo: userSnapshotRepo,
	}
}

func (h *GetUserProfileFeedHandler) Handle(ctx context.Context, q GetUserProfileFeedQuery) (*GetUserProfileFeedResult, error) {
	if q.PageSize <= 0 {
		q.PageSize = 20
	}

	feedItems, nextCursor, err := h.feedItemRepo.GetByUserID(ctx, q.TargetUserID, q.PageSize, q.Cursor)
	if err != nil {
		return nil, fmt.Errorf("fetch user feed items: %w", err)
	}

	userInfo, _ := h.userSnapshotRepo.GetByID(ctx, q.TargetUserID)
	authorName := ""
	avatarURL := ""
	if userInfo != nil {
		authorName = userInfo.FullName()
		avatarURL = userInfo.AvatarURL()
	}

	result := make([]*FeedItemDTO, 0, len(feedItems))
	for _, item := range feedItems {
		result = append(result, &FeedItemDTO{
			ID:              item.ID(),
			UserID:          item.UserID(),
			AuthorName:      authorName,
			AuthorAvatarURL: avatarURL,
			ItemType:        item.ItemType().String(),
			Caption:         item.Caption(),
			MediaURLs:       item.MediaURLs(),
			Visibility:      item.Visibility().String(),
			ReactionCount:   item.ReactionCount(),
			CommentCount:    item.CommentCount(),
			CreatedAt:       item.CreatedAt(),
			UpdatedAt:       item.UpdatedAt(),
			WorkoutData:     item.WorkoutData(),
		})
	}

	return &GetUserProfileFeedResult{
		Items:      result,
		NextCursor: nextCursor,
	}, nil
}
