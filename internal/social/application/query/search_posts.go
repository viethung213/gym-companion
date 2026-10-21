package query

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

type SearchPostsQuery struct {
	ViewerID string
	AuthorID string
	Query    string
	ItemType string
	PageSize int
	Cursor   string
}

type SearchPostsResult struct {
	Items      []*FeedItemDTO
	NextCursor string
	TotalCount int32
}

type SearchPostsHandler struct {
	feedItemRepo     repository.FeedItemRepository
	interactionRepo  repository.InteractionRepository
	userSnapshotRepo repository.UserSnapshotRepository
}

func NewSearchPostsHandler(
	feedItemRepo repository.FeedItemRepository,
	interactionRepo repository.InteractionRepository,
	userSnapshotRepo repository.UserSnapshotRepository,
) *SearchPostsHandler {
	return &SearchPostsHandler{
		feedItemRepo:     feedItemRepo,
		interactionRepo:  interactionRepo,
		userSnapshotRepo: userSnapshotRepo,
	}
}

//nolint:gocritic // q parameter struct passed by value per CQRS query handler design
func (h *SearchPostsHandler) Handle(ctx context.Context, q SearchPostsQuery) (*SearchPostsResult, error) {
	queryText := strings.TrimSpace(q.Query)
	authorID := strings.TrimSpace(q.AuthorID)
	if queryText == "" && authorID == "" {
		return nil, derror.ErrEmptySearchQuery
	}

	if authorID != "" {
		if _, err := uuid.Parse(authorID); err != nil {
			return nil, derror.ErrInvalidUserID
		}
	}

	if authorID == "" && utf8.RuneCountInString(queryText) < 2 {
		return nil, derror.ErrSearchQueryTooShort
	}

	limit := q.PageSize
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	normalizedItemType := ""
	if q.ItemType == string(vo.ItemTypePost) || q.ItemType == string(vo.ItemTypeWorkoutActivity) {
		normalizedItemType = q.ItemType
	}

	filter := repository.SearchPostsFilter{
		ViewerID: strings.TrimSpace(q.ViewerID),
		AuthorID: authorID,
		Query:    queryText,
		ItemType: normalizedItemType,
		Limit:    limit,
		Cursor:   strings.TrimSpace(q.Cursor),
	}

	feedItems, nextCursor, totalCount, err := h.feedItemRepo.SearchPosts(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("search posts: %w", err)
	}

	var distinctUserIDs []string
	seen := make(map[string]bool)
	for _, item := range feedItems {
		if !seen[item.UserID()] {
			seen[item.UserID()] = true
			distinctUserIDs = append(distinctUserIDs, item.UserID())
		}
	}

	var userInfoMap map[string]*entity.UserSnapshot
	if len(distinctUserIDs) > 0 {
		var err error
		userInfoMap, err = h.userSnapshotRepo.GetByIDs(ctx, distinctUserIDs)
		if err != nil {
			return nil, fmt.Errorf("get user snapshots: %w", err)
		}
	}

	var userReactions map[string]*entity.Reaction
	if filter.ViewerID != "" && len(feedItems) > 0 {
		itemIDs := make([]string, len(feedItems))
		for i, item := range feedItems {
			itemIDs[i] = item.ID()
		}
		var err error
		userReactions, err = h.interactionRepo.GetUserReactions(ctx, filter.ViewerID, itemIDs)
		if err != nil {
			return nil, fmt.Errorf("get user reactions: %w", err)
		}
	}

	result := make([]*FeedItemDTO, 0, len(feedItems))
	for _, item := range feedItems {
		authorName := ""
		avatarURL := entity.DefaultAvatarURL
		if u, ok := userInfoMap[item.UserID()]; ok && u != nil {
			authorName = u.FullName()
			if u.AvatarURL() != "" {
				avatarURL = u.AvatarURL()
			}
		}

		userReaction := ""
		if userReactions != nil {
			if r, ok := userReactions[item.ID()]; ok && r != nil {
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

	return &SearchPostsResult{
		Items:      result,
		NextCursor: nextCursor,
		TotalCount: totalCount,
	}, nil
}
