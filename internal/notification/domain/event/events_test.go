package event_test

import (
	"testing"

	"github.com/viethung213/gym-companion/internal/notification/domain/event"
)

func TestDomainEvents(t *testing.T) {
	t.Parallel()

	t.Run("NewDeviceRegisteredEvent", func(t *testing.T) {
		t.Parallel()

		ev := event.NewDeviceRegisteredEvent("evt-123", "user-1", "token-abc", "ANDROID")
		if got, want := ev.ID, "evt-123"; got != want {
			t.Errorf("got ID %s, want %s", got, want)
		}
		if got, want := ev.UserID, "user-1"; got != want {
			t.Errorf("got UserID %s, want %s", got, want)
		}
		if got, want := ev.DeviceToken, "token-abc"; got != want {
			t.Errorf("got DeviceToken %s, want %s", got, want)
		}
		if got, want := ev.DeviceType, "ANDROID"; got != want {
			t.Errorf("got DeviceType %s, want %s", got, want)
		}
	})

	t.Run("NewNotificationSentEvent", func(t *testing.T) {
		t.Parallel()

		data := map[string]string{"foo": "bar"}
		ev := event.NewNotificationSentEvent("notif-1", "user-1", "PUSH", "Title", "Body", data)
		if got, want := ev.NotificationID, "notif-1"; got != want {
			t.Errorf("got NotificationID %s, want %s", got, want)
		}
		if got, want := ev.UserID, "user-1"; got != want {
			t.Errorf("got UserID %s, want %s", got, want)
		}
		if got, want := ev.Channel, "PUSH"; got != want {
			t.Errorf("got Channel %s, want %s", got, want)
		}
		if got, want := ev.Title, "Title"; got != want {
			t.Errorf("got Title %s, want %s", got, want)
		}
		if got, want := ev.Body, "Body"; got != want {
			t.Errorf("got Body %s, want %s", got, want)
		}
		if got, want := ev.Data["foo"], "bar"; got != want {
			t.Errorf("got Data[foo] %s, want %s", got, want)
		}
	})
}
