package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
)

type NotificationRepository interface {
	Save(ctx context.Context, item *aggregate.InAppNotification) error
	BatchSave(ctx context.Context, items []*aggregate.InAppNotification) error
	ListByUserID(ctx context.Context, userID string, limit, offset int32) ([]*aggregate.InAppNotification, int32, error)
	MarkAsRead(ctx context.Context, userID, notificationID string) error
	MarkAllAsRead(ctx context.Context, userID string) error
	FindByUserAndEventID(ctx context.Context, userID, eventID string) (*aggregate.InAppNotification, error)
	SaveSystemAnnouncement(ctx context.Context, announcement *aggregate.SystemAnnouncement) error
	MarkAnnouncementAsRead(ctx context.Context, announcementID, userID string) error
}

// ReactionRollupRepository optionally supports rolling up unread POST_REACTION notifications
// for the same post into a single aggregated notification row.
type ReactionRollupRepository interface {
	FindUnreadReactionByPost(ctx context.Context, userID, feedItemID string) (*aggregate.InAppNotification, error)
	FindUnreadReactionsByPostAndUsers(ctx context.Context, userIDs []string, feedItemID string) ([]*aggregate.InAppNotification, error)
	UpdateNotificationContent(ctx context.Context, item *aggregate.InAppNotification) error
	BatchUpdateNotificationContent(ctx context.Context, items []*aggregate.InAppNotification) error
}
