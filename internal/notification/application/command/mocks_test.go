package command_test

import (
	"context"

	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
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

func (m *mockDeviceRepo) GetActiveDevicesByUserIDs(ctx context.Context, userIDs []string) ([]*aggregate.Device, error) {
	userMap := make(map[string]bool, len(userIDs))
	for _, uid := range userIDs {
		userMap[uid] = true
	}
	var result []*aggregate.Device
	for _, d := range m.devices {
		if userMap[d.UserID()] && d.IsActive() {
			result = append(result, d)
		}
	}
	return result, nil
}

func (m *mockDeviceRepo) GetAllActiveUserIDs(ctx context.Context) ([]string, error) {
	var userIDs []string
	seen := make(map[string]bool)
	for _, d := range m.devices {
		if d.IsActive() && !seen[d.UserID()] {
			seen[d.UserID()] = true
			userIDs = append(userIDs, d.UserID())
		}
	}
	return userIDs, nil
}

func (m *mockDeviceRepo) DeactivateTokens(ctx context.Context, tokens []string) error {
	m.deactivatedTokens = append(m.deactivatedTokens, tokens...)
	return nil
}

type mockNotificationRepo struct {
	items         []*aggregate.InAppNotification
	announcements []*aggregate.SystemAnnouncement
}

func (m *mockNotificationRepo) Save(ctx context.Context, item *aggregate.InAppNotification) error {
	m.items = append(m.items, item)
	return nil
}

func (m *mockNotificationRepo) BatchSave(ctx context.Context, items []*aggregate.InAppNotification) error {
	m.items = append(m.items, items...)
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

func (m *mockNotificationRepo) FindByUserAndEventID(ctx context.Context, userID, eventID string) (*aggregate.InAppNotification, error) {
	for _, it := range m.items {
		if it.UserID() == userID && it.EventID() == eventID {
			return it, nil
		}
	}
	return nil, nil
}

func (m *mockNotificationRepo) SaveSystemAnnouncement(ctx context.Context, announcement *aggregate.SystemAnnouncement) error {
	m.announcements = append(m.announcements, announcement)
	return nil
}

func (m *mockNotificationRepo) MarkAnnouncementAsRead(ctx context.Context, announcementID, userID string) error {
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

func (m *mockSettingRepoWithMap) GetByUserIDs(ctx context.Context, userIDs []string) ([]*aggregate.Setting, error) {
	var result []*aggregate.Setting
	for _, uid := range userIDs {
		if s, exists := m.settings[uid]; exists {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockSettingRepoWithMap) Save(ctx context.Context, setting *aggregate.Setting) error {
	m.settings[setting.UserID()] = setting
	return nil
}

type mockPushProvider struct {
	err              error
	subErr           error
	resp             *port.PushResponse
	subscribedTokens []string
	subscribedTopic  string
	unsubbedTokens   []string
	topicPushes      []string
	callCount        int
}

func (m *mockPushProvider) SendPush(ctx context.Context, tokens []string, title, body string, data map[string]string) (*port.PushResponse, error) {
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	if m.resp != nil {
		return m.resp, nil
	}
	return &port.PushResponse{
		SuccessCount: len(tokens),
		FailureCount: 0,
	}, nil
}

func (m *mockPushProvider) SendPushToTopic(ctx context.Context, topic string, title, body string, data map[string]string) error {
	if m.err != nil {
		return m.err
	}
	m.topicPushes = append(m.topicPushes, topic)
	return nil
}

func (m *mockPushProvider) SubscribeToTopic(ctx context.Context, tokens []string, topic string) error {
	if m.subErr != nil {
		return m.subErr
	}
	m.subscribedTokens = append(m.subscribedTokens, tokens...)
	m.subscribedTopic = topic
	return nil
}

func (m *mockPushProvider) UnsubscribeFromTopic(ctx context.Context, tokens []string, topic string) error {
	m.unsubbedTokens = append(m.unsubbedTokens, tokens...)
	return nil
}

type mockTxManager struct{}

func (m *mockTxManager) ExecTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func (m *mockTxManager) WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return fn(ctx)
}

type mockEventRecorder struct {
	events []any
}

func (m *mockEventRecorder) RecordEvents(ctx context.Context, events []any) error {
	m.events = append(m.events, events...)
	return nil
}
