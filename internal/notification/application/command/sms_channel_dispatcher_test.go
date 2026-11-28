package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/sms"
)

type mockSettingRepoForSMS struct {
	setting *aggregate.Setting
}

func (m *mockSettingRepoForSMS) GetByUserID(ctx context.Context, userID string) (*aggregate.Setting, error) {
	return m.setting, nil
}

func (m *mockSettingRepoForSMS) GetByUserIDs(ctx context.Context, userIDs []string) ([]*aggregate.Setting, error) {
	if m.setting != nil {
		return []*aggregate.Setting{m.setting}, nil
	}
	return nil, nil
}

func (m *mockSettingRepoForSMS) Save(ctx context.Context, setting *aggregate.Setting) error {
	m.setting = setting
	return nil
}

func TestSMSChannelDispatcher_Channel(t *testing.T) {
	dispatcher := command.NewSMSChannelDispatcher(nil, nil)
	if dispatcher.Channel() != "SMS" {
		t.Errorf("expected channel SMS, got %s", dispatcher.Channel())
	}
}

func TestSMSChannelDispatcher_DirectSuccess(t *testing.T) {
	mockSMS := sms.NewMockSMSProvider()
	repo := &mockSettingRepoForSMS{}

	dispatcher := command.NewSMSChannelDispatcher(mockSMS, repo)
	_, err := dispatcher.Dispatch(context.Background(), port.NotificationTarget{
		Scope:  port.ScopeDirect,
		UserID: "user-1",
	}, command.NotificationMessage{
		Body: "Your workout is scheduled for 8:00 AM",
		Data: map[string]string{"phone": "+84912345678"},
	})

	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	sent := mockSMS.GetSentMessages()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	if sent[0].To != "+84912345678" || sent[0].Message != "Your workout is scheduled for 8:00 AM" {
		t.Errorf("sent message mismatch: %+v", sent[0])
	}
}

func TestSMSChannelDispatcher_SkipNonDirectScopes(t *testing.T) {
	mockSMS := sms.NewMockSMSProvider()
	dispatcher := command.NewSMSChannelDispatcher(mockSMS, nil)

	// 1. ScopeGroup must be skipped
	_, err := dispatcher.Dispatch(context.Background(), port.NotificationTarget{
		Scope:   port.ScopeGroup,
		UserIDs: []string{"user-1", "user-2"},
	}, command.NotificationMessage{
		Body: "Group notification",
		Data: map[string]string{"phone": "+84912345678"},
	})
	if err != nil {
		t.Fatalf("expected nil error on group scope skip, got: %v", err)
	}
	if len(mockSMS.GetSentMessages()) != 0 {
		t.Errorf("expected 0 messages sent for ScopeGroup, got %d", len(mockSMS.GetSentMessages()))
	}

	// 2. ScopeBroadcast must be skipped
	_, err = dispatcher.Dispatch(context.Background(), port.NotificationTarget{
		Scope: port.ScopeBroadcast,
	}, command.NotificationMessage{
		Body: "Broadcast notification",
		Data: map[string]string{"phone": "+84912345678"},
	})
	if err != nil {
		t.Fatalf("expected nil error on broadcast scope skip, got: %v", err)
	}
	if len(mockSMS.GetSentMessages()) != 0 {
		t.Errorf("expected 0 messages sent for ScopeBroadcast, got %d", len(mockSMS.GetSentMessages()))
	}
}

func TestSMSChannelDispatcher_EmptyPhoneOrNilProvider(t *testing.T) {
	mockSMS := sms.NewMockSMSProvider()
	dispatcher := command.NewSMSChannelDispatcher(mockSMS, nil)

	// Missing phone in data
	_, err := dispatcher.Dispatch(context.Background(), port.NotificationTarget{
		Scope:  port.ScopeDirect,
		UserID: "user-1",
	}, command.NotificationMessage{
		Body: "No phone test",
		Data: map[string]string{},
	})
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(mockSMS.GetSentMessages()) != 0 {
		t.Errorf("expected 0 messages sent, got %d", len(mockSMS.GetSentMessages()))
	}

	// Nil provider
	nilDispatcher := command.NewSMSChannelDispatcher(nil, nil)
	_, err = nilDispatcher.Dispatch(context.Background(), port.NotificationTarget{
		Scope:  port.ScopeDirect,
		UserID: "user-1",
	}, command.NotificationMessage{
		Body: "Nil provider test",
		Data: map[string]string{"phone": "+84912345678"},
	})
	if err != nil {
		t.Fatalf("expected nil error with nil provider, got: %v", err)
	}
}

func TestSMSChannelDispatcher_DisabledByUser(t *testing.T) {
	mockSMS := sms.NewMockSMSProvider()
	setting := aggregate.ReconstituteSetting(
		"user-1",
		true,
		true,
		false, // SMS disabled
		"",
		"",
		time.Now(),
		time.Now(),
	)
	repo := &mockSettingRepoForSMS{setting: setting}

	dispatcher := command.NewSMSChannelDispatcher(mockSMS, repo)
	_, err := dispatcher.Dispatch(context.Background(), port.NotificationTarget{
		Scope:  port.ScopeDirect,
		UserID: "user-1",
	}, command.NotificationMessage{
		Body: "Hello",
		Data: map[string]string{"phone_number": "+84912345678"},
	})

	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(mockSMS.GetSentMessages()) != 0 {
		t.Errorf("expected 0 SMS sent when disabled, got %d", len(mockSMS.GetSentMessages()))
	}

	// High priority SMS must bypass disabled settings
	_, err = dispatcher.Dispatch(context.Background(), port.NotificationTarget{
		Scope:  port.ScopeDirect,
		UserID: "user-1",
	}, command.NotificationMessage{
		Body:           "OTP: 123456",
		IsHighPriority: true,
		Data:           map[string]string{"recipient_phone": "+84912345678"},
	})
	if err != nil {
		t.Fatalf("expected nil error on high priority SMS, got: %v", err)
	}
	if len(mockSMS.GetSentMessages()) != 1 {
		t.Errorf("expected 1 SMS sent for high priority, got %d", len(mockSMS.GetSentMessages()))
	}
}
