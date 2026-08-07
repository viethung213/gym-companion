package transport_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	notificationv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/notification/v1/message"
	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/application/query"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/transport"
)

type mockDeviceRepo struct {
	devices []string
}

func (m *mockDeviceRepo) Save(ctx context.Context, device *aggregate.Device) error {
	m.devices = append(m.devices, device.DeviceToken())
	return nil
}

func (m *mockDeviceRepo) GetActiveDevicesByUserID(ctx context.Context, userID string) ([]*aggregate.Device, error) {
	return nil, nil
}

func (m *mockDeviceRepo) DeactivateTokens(ctx context.Context, tokens []string) error {
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

type mockPushProvider struct{}

func (m *mockPushProvider) SendPush(ctx context.Context, tokens []string, title, body string, data map[string]string) (*port.PushResponse, error) {
	return &port.PushResponse{SuccessCount: len(tokens)}, nil
}

type mockTxManager struct{}

func (m *mockTxManager) ExecTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func TestGRPCHandlerAndConnectAdapter(t *testing.T) {
	t.Parallel()

	devRepo := &mockDeviceRepo{}
	notifRepo := &mockNotificationRepo{}
	settingRepo := &mockSettingRepo{settings: make(map[string]*aggregate.Setting)}
	provider := &mockPushProvider{}
	txMgr := &mockTxManager{}

	regDeviceH := command.NewRegisterDeviceTokenHandler(devRepo)
	sendPushH := command.NewSendPushNotificationHandler(devRepo, notifRepo, settingRepo, provider, txMgr, nil)
	updateSetH := command.NewUpdateNotificationSettingsHandler(settingRepo)
	markReadH := command.NewMarkNotificationAsReadHandler(notifRepo)

	getSetH := query.NewGetNotificationSettingsHandler(settingRepo)
	listNotifH := query.NewListNotificationsHandler(notifRepo)

	grpcHandler := transport.NewGRPCHandler(
		sendPushH,
		regDeviceH,
		updateSetH,
		markReadH,
		getSetH,
		listNotifH,
	)

	connectHandler := transport.NewConnectNotificationHandler(grpcHandler)

	ctx := context.Background()

	t.Run("RegisterDeviceToken gRPC & Connect", func(t *testing.T) {
		req := &notificationv1message.RegisterDeviceTokenRequest{
			UserId:      "usr-grpc-1",
			DeviceToken: "token-grpc-1",
			DeviceType:  "IOS",
		}

		res, err := grpcHandler.RegisterDeviceToken(ctx, req)
		if err != nil {
			t.Fatalf("gRPC RegisterDeviceToken error: %v", err)
		}
		if !res.GetSuccess() {
			t.Errorf("got success = false, want true")
		}

		connRes, err := connectHandler.RegisterDeviceToken(ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("Connect RegisterDeviceToken error: %v", err)
		}
		if !connRes.Msg.GetSuccess() {
			t.Errorf("got Connect success = false, want true")
		}
	})

	t.Run("SendPushNotification gRPC & Connect", func(t *testing.T) {
		req := &notificationv1message.SendPushNotificationRequest{
			UserId: "usr-grpc-1",
			Title:  "Title",
			Body:   "Body",
		}

		res, err := grpcHandler.SendPushNotification(ctx, req)
		if err != nil {
			t.Fatalf("gRPC SendPushNotification error: %v", err)
		}
		if res.GetNotificationId() == "" {
			t.Errorf("got empty NotificationId")
		}

		connRes, err := connectHandler.SendPushNotification(ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("Connect SendPushNotification error: %v", err)
		}
		if connRes.Msg.GetNotificationId() == "" {
			t.Errorf("got empty Connect NotificationId")
		}
	})

	t.Run("Get & Update Notification Settings gRPC & Connect", func(t *testing.T) {
		getReq := &notificationv1message.GetNotificationSettingsRequest{UserId: "usr-set-1"}
		getRes, err := grpcHandler.GetNotificationSettings(ctx, getReq)
		if err != nil {
			t.Fatalf("gRPC GetNotificationSettings error: %v", err)
		}
		if !getRes.GetEnablePush() {
			t.Errorf("got default EnablePush = false, want true")
		}

		upReq := &notificationv1message.UpdateNotificationSettingsRequest{
			UserId:          "usr-set-1",
			EnablePush:      false,
			EnableEmail:     true,
			EnableSms:       false,
			QuietHoursStart: "22:00",
			QuietHoursEnd:   "07:00",
		}

		upRes, err := grpcHandler.UpdateNotificationSettings(ctx, upReq)
		if err != nil {
			t.Fatalf("gRPC UpdateNotificationSettings error: %v", err)
		}
		if !upRes.GetSuccess() {
			t.Errorf("got Update success = false, want true")
		}

		connGetRes, err := connectHandler.GetNotificationSettings(ctx, connect.NewRequest(getReq))
		if err != nil {
			t.Fatalf("Connect GetNotificationSettings error: %v", err)
		}
		if connGetRes.Msg.GetEnablePush() {
			t.Errorf("after update, got EnablePush = true, want false")
		}
	})

	t.Run("ListNotifications & MarkNotificationAsRead gRPC & Connect", func(t *testing.T) {
		listReq := &notificationv1message.ListNotificationsRequest{
			UserId: "usr-grpc-1",
			Limit:  10,
			Offset: 0,
		}

		listRes, err := grpcHandler.ListNotifications(ctx, listReq)
		if err != nil {
			t.Fatalf("gRPC ListNotifications error: %v", err)
		}
		if listRes.GetTotalCount() < 1 {
			t.Errorf("got TotalCount = %d, want >= 1", listRes.GetTotalCount())
		}

		markReq := &notificationv1message.MarkNotificationAsReadRequest{
			UserId:         "usr-grpc-1",
			NotificationId: "all",
		}

		markRes, err := grpcHandler.MarkNotificationAsRead(ctx, markReq)
		if err != nil {
			t.Fatalf("gRPC MarkNotificationAsRead error: %v", err)
		}
		if !markRes.GetSuccess() {
			t.Errorf("got Mark success = false, want true")
		}

		connListRes, err := connectHandler.ListNotifications(ctx, connect.NewRequest(listReq))
		if err != nil {
			t.Fatalf("Connect ListNotifications error: %v", err)
		}
		if connListRes.Msg.GetTotalCount() < 1 {
			t.Errorf("got Connect TotalCount = %d, want >= 1", connListRes.Msg.GetTotalCount())
		}
	})
}
