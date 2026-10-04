package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/sms"
)

type mockSettingRepoForSMS struct {
	setting *aggregate.Setting
}

func (m *mockSettingRepoForSMS) GetByUserID(ctx context.Context, userID string) (*aggregate.Setting, error) {
	return m.setting, nil
}

func (m *mockSettingRepoForSMS) Save(ctx context.Context, setting *aggregate.Setting) error {
	m.setting = setting
	return nil
}

func TestSendSMSNotificationHandler_Success(t *testing.T) {
	mockSMS := sms.NewMockSMSProvider()
	repo := &mockSettingRepoForSMS{}

	handler := command.NewSendSMSNotificationHandler(repo, mockSMS)
	resp, err := handler.Handle(context.Background(), command.SendSMSNotificationCommand{
		UserID:      "user-1",
		PhoneNumber: "+84912345678",
		Message:     "Your workout is scheduled for 8:00 AM",
	})

	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if resp.Status != "SENT" {
		t.Errorf("expected SENT status, got: %s", resp.Status)
	}

	sent := mockSMS.GetSentMessages()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	if sent[0].To != "+84912345678" || sent[0].Message != "Your workout is scheduled for 8:00 AM" {
		t.Errorf("sent message mismatch: %+v", sent[0])
	}
}

func TestSendSMSNotificationHandler_DisabledByUser(t *testing.T) {
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

	handler := command.NewSendSMSNotificationHandler(repo, mockSMS)
	resp, err := handler.Handle(context.Background(), command.SendSMSNotificationCommand{
		UserID:      "user-1",
		PhoneNumber: "+84912345678",
		Message:     "Hello",
	})

	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if resp.Status != "SMS_DISABLED_BY_USER" {
		t.Errorf("expected SMS_DISABLED_BY_USER, got %s", resp.Status)
	}
	if len(mockSMS.GetSentMessages()) != 0 {
		t.Errorf("expected 0 SMS sent, got %d", len(mockSMS.GetSentMessages()))
	}

	// High priority SMS must bypass disabled settings
	respHigh, err := handler.Handle(context.Background(), command.SendSMSNotificationCommand{
		UserID:         "user-1",
		PhoneNumber:    "+84912345678",
		Message:        "OTP: 123456",
		IsHighPriority: true,
	})
	if err != nil {
		t.Fatalf("expected nil error on high priority SMS, got: %v", err)
	}
	if respHigh.Status != "SENT" {
		t.Errorf("expected SENT for high priority SMS, got %s", respHigh.Status)
	}
	if len(mockSMS.GetSentMessages()) != 1 {
		t.Errorf("expected 1 SMS sent, got %d", len(mockSMS.GetSentMessages()))
	}
}

func TestSendSMSNotificationHandler_EmptyRecipient(t *testing.T) {
	handler := command.NewSendSMSNotificationHandler(nil, sms.NewMockSMSProvider())
	_, err := handler.Handle(context.Background(), command.SendSMSNotificationCommand{
		UserID:      "user-1",
		PhoneNumber: "",
	})
	if err == nil {
		t.Fatal("expected error on empty phone number, got nil")
	}
}
