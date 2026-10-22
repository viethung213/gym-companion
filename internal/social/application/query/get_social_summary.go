package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
)

type GetSocialSummaryQuery struct {
	TargetUserID  string
	CurrentUserID string
}

type GetSocialSummaryHandler struct {
	followRepo       repository.FollowRepository
	feedItemRepo     repository.FeedItemRepository
	userSnapshotRepo repository.UserSnapshotRepository
}

func NewGetSocialSummaryHandler(
	followRepo repository.FollowRepository,
	feedItemRepo repository.FeedItemRepository,
	userSnapshotRepo repository.UserSnapshotRepository,
) *GetSocialSummaryHandler {
	return &GetSocialSummaryHandler{
		followRepo:       followRepo,
		feedItemRepo:     feedItemRepo,
		userSnapshotRepo: userSnapshotRepo,
	}
}

func (h *GetSocialSummaryHandler) Handle(ctx context.Context, q GetSocialSummaryQuery) (*SocialSummaryDTO, error) {
	userInfo, err := h.userSnapshotRepo.GetByID(ctx, q.TargetUserID)
	if err != nil {
		return nil, fmt.Errorf("get user snapshot: %w", err)
	}

	fullName := "User"
	avatarURL := entity.DefaultAvatarURL
	role := "user"
	var followerCount int32
	var followingCount int32

	if userInfo != nil {
		fullName = userInfo.FullName()
		avatarURL = userInfo.AvatarURL()
		role = userInfo.Role()
		followerCount = userInfo.FollowerCount()
		followingCount = userInfo.FollowingCount()
	}

	feedCount, err := h.feedItemRepo.CountByUserID(ctx, q.TargetUserID)
	if err != nil {
		return nil, fmt.Errorf("count feed items: %w", err)
	}

	isFollowing := false
	if q.CurrentUserID != "" && q.CurrentUserID != q.TargetUserID {
		isFollowing, _ = h.followRepo.IsFollowing(ctx, q.CurrentUserID, q.TargetUserID)
	}

	return &SocialSummaryDTO{
		UserID:         q.TargetUserID,
		FullName:       fullName,
		AvatarURL:      avatarURL,
		Role:           role,
		FollowerCount:  followerCount,
		FollowingCount: followingCount,
		PostCount:      feedCount,
		IsFollowing:    isFollowing,
	}, nil
}
