package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
	"github.com/viethung213/gym-companion/internal/notification/domain/vo"
)

type mockDeviceRepo struct {
	devices           []*aggregate.Device
	deactivatedTokens []string
}

func (m *mockDeviceRepo) Save(ctx context.Context, device *aggregate.Device) error {
	m.devices = append(m.devices, device)
	return nil
}

func (m *mockDeviceRepo) GetActiveDevicesByUserID(ctx context.Context, userID string) ([]*aggregate.Device, error) {
	var result []*aggregate.Device
	for _, d := range m.devices {
		if d.UserID() == userID && d.IsActive() {
			result = append(result, d)
		}
	}
	return result, nil
}

func (m *mockDeviceRepo) DeactivateTokens(ctx context.Context, tokens []string) error {
	m.deactivatedTokens = append(m.deactivatedTokens, tokens...)
	return nil
}

type mockNotificationRepo struct {
	items []*aggregate.InAppNotification
}

func (m *mockNotificationRepo) Save(ctx context.Context, item *aggregate.InAppNotification) error {
	m.items = append(m.items, item)
	return nil
}

func (m *mockNotificationRepo) ListByUserID(ctx context.Context, userID string, limit, offset int32) ([]*aggregate.InAppNotification, int32, error) {
	return m.items, int32(len(m.items)), nil
}

func (m *mockNotificationRepo) MarkAsRead(ctx context.Context, userID, notificationID string) error {
	return nil
}

func (m *mockNotificationRepo) MarkAllAsRead(ctx context.Context, userID string) error {
	return nil
}

type mockSettingRepoWithMap struct {
	settings map[string]*aggregate.Setting
}

func (m *mockSettingRepoWithMap) GetByUserID(ctx context.Context, userID string) (*aggregate.Setting, error) {
	s, exists := m.settings[userID]
	if !exists {
		return nil, derror.ErrSettingNotFound
	}
	return s, nil
}

func (m *mockSettingRepoWithMap) Save(ctx context.Context, setting *aggregate.Setting) error {
	m.settings[setting.UserID()] = setting
	return nil
}

type mockPushProvider struct{}

func (m *mockPushProvider) SendPush(ctx context.Context, tokens []string, title, body string, data map[string]string) (*port.PushResponse, error) {
	return &port.PushResponse{
		SuccessCount: len(tokens),
		FailureCount: 0,
	}, nil
}

type mockTxManager struct{}

func (m *mockTxManager) ExecTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func TestSendPushNotificationHandler(t *testing.T) {
	t.Parallel()

	deviceRepo := &mockDeviceRepo{}
	dev, _ := aggregate.NewDevice("dev-1", "usr-1", "token-xyz", vo.DeviceTypeAndroid)
	_ = deviceRepo.Save(context.Background(), dev)

	notifRepo := &mockNotificationRepo{}
	settingRepo := &mockSettingRepoWithMap{settings: make(map[string]*aggregate.Setting)}
	provider := &mockPushProvider{}
	txMgr := &mockTxManager{}

	handler := command.NewSendPushNotificationHandler(deviceRepo, notifRepo, settingRepo, provider, txMgr, nil)

	t.Run("successful push send", func(t *testing.T) {
		cmd := command.SendPushNotificationCommand{
			UserID: "usr-1",
			Title:  "Test Title",
			Body:   "Test Body",
			Data:   map[string]string{"key": "val"},
		}

		res, err := handler.Handle(context.Background(), cmd)
		if err != nil {
			t.Fatalf("got unexpected error: %v", err)
		}

		if got, want := res.Status, "SENT"; got != want {
			t.Errorf("got status %s, want %s", got, want)
		}
	})

	t.Run("suppressed push when enable_push is false", func(t *testing.T) {
		s, _ := aggregate.NewDefaultSetting("usr-disabled")
		_ = s.Update(false, true, false, "", "")
		_ = settingRepo.Save(context.Background(), s)

		cmd := command.SendPushNotificationCommand{
			UserID: "usr-disabled",
			Title:  "Test",
			Body:   "Test",
		}

		res, err := handler.Handle(context.Background(), cmd)
		if err != nil {
			t.Fatalf("got unexpected error: %v", err)
		}

		if got, want := res.Status, "PUSH_DISABLED_BY_USER"; got != want {
			t.Errorf("got status %s, want %s", got, want)
		}
	})

	t.Run("suppressed push during quiet hours", func(t *testing.T) {
		nowStr := time.Now().UTC().Format("15:04")
		s, _ := aggregate.NewDefaultSetting("usr-quiet")
		// Set quiet hours window matching current time
		_ = s.Update(true, true, false, "00:00", "23:59")
		_ = settingRepo.Save(context.Background(), s)

		cmd := command.SendPushNotificationCommand{
			UserID: "usr-quiet",
			Title:  "Test",
			Body:   "Test (now=" + nowStr + ")",
		}

		res, err := handler.Handle(context.Background(), cmd)
		if err != nil {
			t.Fatalf("got unexpected error: %v", err)
		}

		if got, want := res.Status, "QUIET_HOURS_ACTIVE"; got != want {
			t.Errorf("got status %s, want %s", got, want)
		}
	})
}
