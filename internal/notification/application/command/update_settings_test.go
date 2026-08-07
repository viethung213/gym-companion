package command_test

import (
	"context"
	"testing"

	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
)

type mockSettingRepo struct {
	settings map[string]*aggregate.Setting
}

func newMockSettingRepo() *mockSettingRepo {
	return &mockSettingRepo{settings: make(map[string]*aggregate.Setting)}
}

func (m *mockSettingRepo) GetByUserID(ctx context.Context, userID string) (*aggregate.Setting, error) {
	s, exists := m.settings[userID]
	if !exists {
		return nil, derror.ErrSettingNotFound
	}
	return s, nil
}

func (m *mockSettingRepo) Save(ctx context.Context, setting *aggregate.Setting) error {
	m.settings[setting.UserID()] = setting
	return nil
}

func TestUpdateNotificationSettingsHandler(t *testing.T) {
	t.Parallel()

	settingRepo := newMockSettingRepo()
	handler := command.NewUpdateNotificationSettingsHandler(settingRepo)

	t.Run("update non-existing setting creates default and updates", func(t *testing.T) {
		cmd := command.UpdateNotificationSettingsCommand{
			UserID:          "usr-fresh",
			EnablePush:      false,
			EnableEmail:     true,
			EnableSMS:       false,
			QuietHoursStart: "22:00",
			QuietHoursEnd:   "07:00",
		}

		err := handler.Handle(context.Background(), cmd)
		if err != nil {
			t.Fatalf("got unexpected error: %v", err)
		}

		saved, err := settingRepo.GetByUserID(context.Background(), "usr-fresh")
		if err != nil {
			t.Fatalf("get saved setting error: %v", err)
		}

		if got, want := saved.EnablePush(), false; got != want {
			t.Errorf("got EnablePush %v, want %v", got, want)
		}
		if got, want := saved.QuietHoursStart(), "22:00"; got != want {
			t.Errorf("got QuietHoursStart %s, want %s", got, want)
		}
	})
}
