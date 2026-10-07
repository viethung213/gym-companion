package query

import (
	"context"
	"strings"

	"github.com/viethung213/gym-companion/internal/social/domain/repository"
)

type SearchUsersQuery struct {
	CurrentUserID string
	Query         string
	Role          string
	PageSize      int
	Cursor        string
}

type DiscoverUserDTO struct {
	UserID        string
	FullName      string
	AvatarURL     string
	Role          string
	IsFollowing   bool
	FollowerCount int32
}

type SearchUsersResult struct {
	Users      []*DiscoverUserDTO
	NextCursor string
	TotalCount int32
}

type SearchUsersHandler struct {
	userSnapshotRepo repository.UserSnapshotRepository
	followRepo       repository.FollowRepository
}

func NewSearchUsersHandler(
	userSnapshotRepo repository.UserSnapshotRepository,
	followRepo repository.FollowRepository,
) *SearchUsersHandler {
	return &SearchUsersHandler{
		userSnapshotRepo: userSnapshotRepo,
		followRepo:       followRepo,
	}
}

func (h *SearchUsersHandler) Handle(ctx context.Context, q SearchUsersQuery) (*SearchUsersResult, error) {
	// Không hỗ trợ tìm kiếm role "admin"
	role := strings.ToLower(strings.TrimSpace(q.Role))
	if role == "admin" {
		return &SearchUsersResult{
			Users:      []*DiscoverUserDTO{},
			NextCursor: "",
			TotalCount: 0,
		}, nil
	}

	limit := q.PageSize
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	users, nextCursor, total, err := h.userSnapshotRepo.SearchUsers(ctx, q.Query, role, limit, q.Cursor)
	if err != nil {
		return nil, err
	}

	followingSet := make(map[string]bool)
	if q.CurrentUserID != "" {
		followingIDs, err := h.followRepo.GetFollowingIDs(ctx, q.CurrentUserID)
		if err == nil {
			for _, id := range followingIDs {
				followingSet[id] = true
			}
		}
	}

	dtos := make([]*DiscoverUserDTO, len(users))
	for i, u := range users {
		followerCount, _ := h.followRepo.CountFollowers(ctx, u.ID())
		isFollowing := false
		if q.CurrentUserID != "" && q.CurrentUserID != u.ID() {
			isFollowing = followingSet[u.ID()]
		}

		dtos[i] = &DiscoverUserDTO{
			UserID:        u.ID(),
			FullName:      u.FullName(),
			AvatarURL:     u.AvatarURL(),
			Role:          u.Role(),
			IsFollowing:   isFollowing,
			FollowerCount: followerCount,
		}
	}

	return &SearchUsersResult{
		Users:      dtos,
		NextCursor: nextCursor,
		TotalCount: total,
	}, nil
}
