package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/social/domain/repository"
)

type GetFollowersQuery struct {
	UserID        string
	CurrentUserID string
	PageSize      int
	Cursor        string
}

type GetFollowersResult struct {
	Followers  []*UserSocialSummaryDTO
	NextCursor string
	TotalCount int32
}

type GetFollowersHandler struct {
	followRepo       repository.FollowRepository
	userSnapshotRepo repository.UserSnapshotRepository
}

func NewGetFollowersHandler(
	followRepo repository.FollowRepository,
	userSnapshotRepo repository.UserSnapshotRepository,
) *GetFollowersHandler {
	return &GetFollowersHandler{
		followRepo:       followRepo,
		userSnapshotRepo: userSnapshotRepo,
	}
}

func (h *GetFollowersHandler) Handle(ctx context.Context, q GetFollowersQuery) (*GetFollowersResult, error) {
	if q.PageSize <= 0 {
		q.PageSize = 20
	}

	follows, nextCursor, totalCount, err := h.followRepo.GetFollowers(ctx, q.UserID, q.PageSize, q.Cursor)
	if err != nil {
		return nil, fmt.Errorf("fetch followers: %w", err)
	}

	userIDs := make([]string, 0, len(follows))
	for _, f := range follows {
		userIDs = append(userIDs, f.FollowerID())
	}

	userInfoMap, _ := h.userSnapshotRepo.GetByIDs(ctx, userIDs)

	result := make([]*UserSocialSummaryDTO, 0, len(follows))
	for _, f := range follows {
		fullName := ""
		avatarURL := ""
		if u, ok := userInfoMap[f.FollowerID()]; ok && u != nil {
			fullName = u.FullName()
			avatarURL = u.AvatarURL()
		}

		isFollowing := false
		if q.CurrentUserID != "" && q.CurrentUserID != f.FollowerID() {
			isFollowing, _ = h.followRepo.IsFollowing(ctx, q.CurrentUserID, f.FollowerID())
		}

		result = append(result, &UserSocialSummaryDTO{
			UserID:      f.FollowerID(),
			FullName:    fullName,
			AvatarURL:   avatarURL,
			IsFollowing: isFollowing,
		})
	}

	return &GetFollowersResult{
		Followers:  result,
		NextCursor: nextCursor,
		TotalCount: totalCount,
	}, nil
}
