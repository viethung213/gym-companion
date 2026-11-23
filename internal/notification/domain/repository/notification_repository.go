package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
)

type NotificationRepository interface {
	Save(ctx context.Context, item *aggregate.InAppNotification) error
	ListByUserID(ctx context.Context, userID string, limit, offset int32) ([]*aggregate.InAppNotification, int32, error)
	MarkAsRead(ctx context.Context, userID, notificationID string) error
	MarkAllAsRead(ctx context.Context, userID string) error
}

// ReactionRollupRepository optionally supports rolling up unread POST_REACTION notifications
// for the same post into a single aggregated notification row.
type ReactionRollupRepository interface {
	FindUnreadReactionByPost(ctx context.Context, userID, feedItemID string) (*aggregate.InAppNotification, error)
	UpdateNotificationContent(ctx context.Context, item *aggregate.InAppNotification) error
}
