package aggregate_test

import (
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
	"github.com/viethung213/gym-companion/internal/notification/domain/vo"
)

func TestSettingAggregate(t *testing.T) {
	t.Parallel()

	t.Run("NewDefaultSetting with empty UserID", func(t *testing.T) {
		t.Parallel()

		_, err := aggregate.NewDefaultSetting("")
		if !errors.Is(err, derror.ErrEmptyUserID) {
			t.Fatalf("got error %v, want %v", err, derror.ErrEmptyUserID)
		}
	})

	t.Run("NewDefaultSetting success and update", func(t *testing.T) {
		t.Parallel()

		setting, err := aggregate.NewDefaultSetting("user-123")
		if err != nil {
			t.Fatalf("got unexpected error: %v", err)
		}

		if got, want := setting.UserID(), "user-123"; got != want {
			t.Errorf("got UserID %s, want %s", got, want)
		}
		if got, want := setting.EnablePush(), true; got != want {
			t.Errorf("got EnablePush %v, want %v", got, want)
		}
		if got, want := setting.EnableEmail(), true; got != want {
			t.Errorf("got EnableEmail %v, want %v", got, want)
		}
		if got, want := setting.EnableSMS(), false; got != want {
			t.Errorf("got EnableSMS %v, want %v", got, want)
		}

		if err := setting.Update(false, true, true, "22:00", "07:00"); err != nil {
			t.Fatalf("Update error: %v", err)
		}

		if got, want := setting.EnablePush(), false; got != want {
			t.Errorf("after update, got EnablePush %v, want %v", got, want)
		}
		if got, want := setting.QuietHoursStart(), "22:00"; got != want {
			t.Errorf("after update, got QuietHoursStart %s, want %s", got, want)
		}
		if got, want := setting.QuietHoursEnd(), "07:00"; got != want {
			t.Errorf("after update, got QuietHoursEnd %s, want %s", got, want)
		}
	})

	t.Run("Update with invalid quiet hours format returns error", func(t *testing.T) {
		t.Parallel()

		setting, err := aggregate.NewDefaultSetting("user-invalid-time")
		if err != nil {
			t.Fatalf("got unexpected error: %v", err)
		}

		err = setting.Update(true, true, false, "25:99", "07:00")
		if err == nil {
			t.Fatal("got nil error for invalid quiet start, want error")
		}
		if !errors.Is(err, vo.ErrInvalidTimeFormat) {
			t.Errorf("got error %v, want %v", err, vo.ErrInvalidTimeFormat)
		}
	})

	t.Run("ReconstituteSetting", func(t *testing.T) {
		t.Parallel()

		now := time.Now().UTC()
		setting := aggregate.ReconstituteSetting(
			"user-456",
			false,
			true,
			false,
			"23:00",
			"06:00",
			now,
			now,
		)

		if got, want := setting.UserID(), "user-456"; got != want {
			t.Errorf("got UserID %s, want %s", got, want)
		}
		if got, want := setting.CreatedAt(), now; got != want {
			t.Errorf("got CreatedAt %v, want %v", got, want)
		}
	})
}
