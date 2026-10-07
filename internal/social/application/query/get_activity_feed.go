package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/social/domain/repository"
)

type GetActivityFeedQuery struct {
	CurrentUserID string
	PageSize      int
	Cursor        string
}

type GetActivityFeedResult struct {
	Items      []*FeedItemDTO
	NextCursor string
}

type GetActivityFeedHandler struct {
	followRepo       repository.FollowRepository
	feedItemRepo     repository.FeedItemRepository
	interactionRepo  repository.InteractionRepository
	userSnapshotRepo repository.UserSnapshotRepository
}

func NewGetActivityFeedHandler(
	followRepo repository.FollowRepository,
	feedItemRepo repository.FeedItemRepository,
	interactionRepo repository.InteractionRepository,
	userSnapshotRepo repository.UserSnapshotRepository,
) *GetActivityFeedHandler {
	return &GetActivityFeedHandler{
		followRepo:       followRepo,
		feedItemRepo:     feedItemRepo,
		interactionRepo:  interactionRepo,
		userSnapshotRepo: userSnapshotRepo,
	}
}

func (h *GetActivityFeedHandler) Handle(ctx context.Context, q GetActivityFeedQuery) (*GetActivityFeedResult, error) {
	if q.PageSize <= 0 {
		q.PageSize = 20
	}

	followingIDs, err := h.followRepo.GetFollowingIDs(ctx, q.CurrentUserID)
	if err != nil {
		return nil, fmt.Errorf("get following ids: %w", err)
	}

	// Tác giả bao gồm chính user + những người user follow
	authorIDs := append([]string{q.CurrentUserID}, followingIDs...)

	feedItems, nextCursor, fetchErr := h.feedItemRepo.GetByAuthorIDs(ctx, authorIDs, q.PageSize, q.Cursor)
	if fetchErr != nil {
		return nil, fmt.Errorf("fetch feed items: %w", fetchErr)
	}

	// Lấy thông tin user (Tên, Avatar) theo batch
	var distinctUserIDs []string
	seen := make(map[string]bool)
	for _, item := range feedItems {
		if !seen[item.UserID()] {
			seen[item.UserID()] = true
			distinctUserIDs = append(distinctUserIDs, item.UserID())
		}
	}

	userInfoMap, _ := h.userSnapshotRepo.GetByIDs(ctx, distinctUserIDs)

	result := make([]*FeedItemDTO, 0, len(feedItems))
	for _, item := range feedItems {
		authorName := ""
		avatarURL := ""
		if u, ok := userInfoMap[item.UserID()]; ok && u != nil {
			authorName = u.FullName()
			avatarURL = u.AvatarURL()
		}

		userReaction := ""
		if q.CurrentUserID != "" {
			if r, err := h.interactionRepo.GetReaction(ctx, q.CurrentUserID, item.ID()); err == nil && r != nil {
				userReaction = r.ReactionType().String()
			}
		}

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
			UserReaction:    userReaction,
			CreatedAt:       item.CreatedAt(),
			UpdatedAt:       item.UpdatedAt(),
			WorkoutData:     item.WorkoutData(),
		})
	}

	return &GetActivityFeedResult{
		Items:      result,
		NextCursor: nextCursor,
	}, nil
}
