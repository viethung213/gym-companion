package query_test

import (
	"context"
	"testing"

	"github.com/viethung213/gym-companion/internal/notification/application/query"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
)

type mockSettingRepo struct {
	settings map[string]*aggregate.Setting
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

func TestGetNotificationSettingsHandler(t *testing.T) {
	t.Parallel()

	repo := &mockSettingRepo{settings: make(map[string]*aggregate.Setting)}
	handler := query.NewGetNotificationSettingsHandler(repo)

	t.Run("get non-existing setting returns default setting", func(t *testing.T) {
		t.Parallel()

		res, err := handler.Handle(context.Background(), query.GetNotificationSettingsQuery{UserID: "usr-new"})
		if err != nil {
			t.Fatalf("got unexpected error: %v", err)
		}

		if got, want := res.UserID(), "usr-new"; got != want {
			t.Errorf("got UserID %s, want %s", got, want)
		}
		if got, want := res.EnablePush(), true; got != want {
			t.Errorf("got EnablePush %v, want %v", got, want)
		}
	})
}
