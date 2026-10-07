package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
)

type FeedItemRepository interface {
	Create(ctx context.Context, item *aggregate.FeedItem) error
	GetByID(ctx context.Context, id string) (*aggregate.FeedItem, error)
	Delete(ctx context.Context, id, userID string) error
	GetByUserID(ctx context.Context, userID string, limit int, cursor string) ([]*aggregate.FeedItem, string, error)
	GetByAuthorIDs(ctx context.Context, authorIDs []string, limit int, cursor string) ([]*aggregate.FeedItem, string, error)
	CountByUserID(ctx context.Context, userID string) (int32, error)
	UpdateReactionCount(ctx context.Context, id string, delta int) error
	UpdateCommentCount(ctx context.Context, id string, delta int) error
}
