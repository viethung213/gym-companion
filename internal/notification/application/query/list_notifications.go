package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
)

type ListNotificationsQuery struct {
	UserID string
	Limit  int32
	Offset int32
}

type ListNotificationsResult struct {
	Items      []*aggregate.InAppNotification
	TotalCount int32
}

type ListNotificationsHandler struct {
	notificationRepo repository.NotificationRepository
}

func NewListNotificationsHandler(notificationRepo repository.NotificationRepository) *ListNotificationsHandler {
	return &ListNotificationsHandler{notificationRepo: notificationRepo}
}

func (h *ListNotificationsHandler) Handle(ctx context.Context, q ListNotificationsQuery) (*ListNotificationsResult, error) {
	if q.Limit <= 0 {
		q.Limit = 20
	}
	if q.Offset < 0 {
		q.Offset = 0
	}

	items, total, err := h.notificationRepo.ListByUserID(ctx, q.UserID, q.Limit, q.Offset)
	if err != nil {
		return nil, fmt.Errorf("list in-app notifications: %w", err)
	}

	return &ListNotificationsResult{
		Items:      items,
		TotalCount: total,
	}, nil
}
