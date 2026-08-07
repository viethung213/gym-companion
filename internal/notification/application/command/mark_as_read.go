package command

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
)

type MarkNotificationAsReadCommand struct {
	UserID         string
	NotificationID string // If empty or "all", mark all as read
}

type MarkNotificationAsReadHandler struct {
	notificationRepo repository.NotificationRepository
}

func NewMarkNotificationAsReadHandler(notificationRepo repository.NotificationRepository) *MarkNotificationAsReadHandler {
	return &MarkNotificationAsReadHandler{notificationRepo: notificationRepo}
}

func (h *MarkNotificationAsReadHandler) Handle(ctx context.Context, cmd MarkNotificationAsReadCommand) error {
	if cmd.NotificationID == "" || cmd.NotificationID == "all" {
		if err := h.notificationRepo.MarkAllAsRead(ctx, cmd.UserID); err != nil {
			return fmt.Errorf("mark all notifications as read: %w", err)
		}
		return nil
	}

	if err := h.notificationRepo.MarkAsRead(ctx, cmd.UserID, cmd.NotificationID); err != nil {
		return fmt.Errorf("mark notification as read: %w", err)
	}

	return nil
}
