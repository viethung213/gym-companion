package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/social/domain/entity"
)

type InteractionRepository interface {
	// Reactions
	SaveReaction(ctx context.Context, reaction *entity.Reaction) error
	DeleteReaction(ctx context.Context, userID, feedItemID string) error
	GetReaction(ctx context.Context, userID, feedItemID string) (*entity.Reaction, error)
	GetReactionsByFeedItem(ctx context.Context, feedItemID string) ([]*entity.Reaction, error)

	// Comments
	AddComment(ctx context.Context, comment *entity.Comment) error
	GetCommentByID(ctx context.Context, id string) (*entity.Comment, error)
	DeleteComment(ctx context.Context, id, userID string) error
	ListComments(ctx context.Context, feedItemID string, limit int, cursor string) ([]*entity.Comment, string, int32, error)
}
