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
}
