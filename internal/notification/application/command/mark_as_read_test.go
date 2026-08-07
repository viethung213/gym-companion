package command_test

import (
	"context"
	"testing"

	"github.com/viethung213/gym-companion/internal/notification/application/command"
)

func TestMarkNotificationAsReadHandler(t *testing.T) {
	t.Parallel()

	notifRepo := &mockNotificationRepo{}
	handler := command.NewMarkNotificationAsReadHandler(notifRepo)

	t.Run("mark single notification as read", func(t *testing.T) {
		t.Parallel()

		cmd := command.MarkNotificationAsReadCommand{
			UserID:         "usr-1",
			NotificationID: "notif-123",
		}

		err := handler.Handle(context.Background(), cmd)
		if err != nil {
			t.Fatalf("got unexpected error: %v", err)
		}
	})

	t.Run("mark all notifications as read", func(t *testing.T) {
		t.Parallel()

		cmd := command.MarkNotificationAsReadCommand{
			UserID:         "usr-1",
			NotificationID: "all",
		}

		err := handler.Handle(context.Background(), cmd)
		if err != nil {
			t.Fatalf("got unexpected error: %v", err)
		}
	})
}
