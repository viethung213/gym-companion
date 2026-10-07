package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/social/domain/repository"
)

type GetFollowingQuery struct {
	UserID        string
	CurrentUserID string
	PageSize      int
	Cursor        string
}

type GetFollowingResult struct {
	Following  []*UserSocialSummaryDTO
	NextCursor string
	TotalCount int32
}

type GetFollowingHandler struct {
	followRepo       repository.FollowRepository
	userSnapshotRepo repository.UserSnapshotRepository
}

func NewGetFollowingHandler(
	followRepo repository.FollowRepository,
	userSnapshotRepo repository.UserSnapshotRepository,
) *GetFollowingHandler {
	return &GetFollowingHandler{
		followRepo:       followRepo,
		userSnapshotRepo: userSnapshotRepo,
	}
}

func (h *GetFollowingHandler) Handle(ctx context.Context, q GetFollowingQuery) (*GetFollowingResult, error) {
	if q.PageSize <= 0 {
		q.PageSize = 20
	}

	follows, nextCursor, totalCount, err := h.followRepo.GetFollowing(ctx, q.UserID, q.PageSize, q.Cursor)
	if err != nil {
		return nil, fmt.Errorf("fetch following: %w", err)
	}

	userIDs := make([]string, 0, len(follows))
	for _, f := range follows {
		userIDs = append(userIDs, f.FollowingID())
	}

	userInfoMap, _ := h.userSnapshotRepo.GetByIDs(ctx, userIDs)

	result := make([]*UserSocialSummaryDTO, 0, len(follows))
	for _, f := range follows {
		fullName := ""
		avatarURL := ""
		if u, ok := userInfoMap[f.FollowingID()]; ok && u != nil {
			fullName = u.FullName()
			avatarURL = u.AvatarURL()
		}

		isFollowing := false
		if q.CurrentUserID != "" && q.CurrentUserID != f.FollowingID() {
			isFollowing, _ = h.followRepo.IsFollowing(ctx, q.CurrentUserID, f.FollowingID())
		}

		result = append(result, &UserSocialSummaryDTO{
			UserID:      f.FollowingID(),
			FullName:    fullName,
			AvatarURL:   avatarURL,
			IsFollowing: isFollowing,
		})
	}

	return &GetFollowingResult{
		Following:  result,
		NextCursor: nextCursor,
		TotalCount: totalCount,
	}, nil
}
