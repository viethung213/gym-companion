package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
)

type FollowRepository interface {
	Follow(ctx context.Context, follow *aggregate.Follow) error
	Unfollow(ctx context.Context, followerID, followingID string) error
	IsFollowing(ctx context.Context, followerID, followingID string) (bool, error)
	GetFollowers(ctx context.Context, userID string, limit int, cursor string) ([]*aggregate.Follow, string, int32, error)
	GetFollowing(ctx context.Context, userID string, limit int, cursor string) ([]*aggregate.Follow, string, int32, error)
	GetFollowingIDs(ctx context.Context, followerID string) ([]string, error)
	CountFollowers(ctx context.Context, userID string) (int32, error)
	CountFollowing(ctx context.Context, userID string) (int32, error)
	FilterFollowingIDs(ctx context.Context, followerID string, targetUserIDs []string) (map[string]bool, error)
	CountFollowersByIDs(ctx context.Context, userIDs []string) (map[string]int32, error)
}
