package aggregate_test

import (
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
)

func TestInAppNotificationAggregate(t *testing.T) {
	t.Parallel()

	t.Run("NewInAppNotification validation errors", func(t *testing.T) {
		t.Parallel()

		_, err := aggregate.NewInAppNotification("n-1", "", "Title", "Body", nil)
		if !errors.Is(err, derror.ErrEmptyUserID) {
			t.Errorf("got error %v, want %v", err, derror.ErrEmptyUserID)
		}

		_, err = aggregate.NewInAppNotification("n-1", "user-1", "", "Body", nil)
		if !errors.Is(err, derror.ErrEmptyTitle) {
			t.Errorf("got error %v, want %v", err, derror.ErrEmptyTitle)
		}
	})

	t.Run("NewInAppNotification success and MarkAsRead", func(t *testing.T) {
		t.Parallel()

		data := map[string]string{"foo": "bar"}
		notif, err := aggregate.NewInAppNotification("notif-123", "user-1", "Test Title", "Test Body", data)
		if err != nil {
			t.Fatalf("got unexpected error: %v", err)
		}

		if got, want := notif.ID(), "notif-123"; got != want {
			t.Errorf("got ID %s, want %s", got, want)
		}
		if got, want := notif.UserID(), "user-1"; got != want {
			t.Errorf("got UserID %s, want %s", got, want)
		}
		if got, want := notif.Title(), "Test Title"; got != want {
			t.Errorf("got Title %s, want %s", got, want)
		}
		if got, want := notif.Body(), "Test Body"; got != want {
			t.Errorf("got Body %s, want %s", got, want)
		}
		if got, want := notif.IsRead(), false; got != want {
			t.Errorf("got IsRead %v, want %v", got, want)
		}
		if got, want := notif.Data()["foo"], "bar"; got != want {
			t.Errorf("got Data[foo] %s, want %s", got, want)
		}

		notif.MarkAsRead()
		if got, want := notif.IsRead(), true; got != want {
			t.Errorf("after MarkAsRead(), got IsRead %v, want %v", got, want)
		}
	})

	t.Run("ReconstituteInAppNotification", func(t *testing.T) {
		t.Parallel()

		now := time.Now().UTC()
		notif := aggregate.ReconstituteInAppNotification(
			"notif-999",
			"user-888",
			"Title 999",
			"Body 999",
			map[string]string{"a": "b"},
			true,
			now,
		)

		if got, want := notif.ID(), "notif-999"; got != want {
			t.Errorf("got ID %s, want %s", got, want)
		}
		if got, want := notif.IsRead(), true; got != want {
			t.Errorf("got IsRead %v, want %v", got, want)
		}
	})
}
