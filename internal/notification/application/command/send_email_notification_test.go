package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/email"
)

type mockSettingRepoForEmail struct {
	setting *aggregate.Setting
}

func (m *mockSettingRepoForEmail) GetByUserID(ctx context.Context, userID string) (*aggregate.Setting, error) {
	return m.setting, nil
}

func (m *mockSettingRepoForEmail) Save(ctx context.Context, setting *aggregate.Setting) error {
	m.setting = setting
	return nil
}

func TestSendEmailNotificationHandler_Success(t *testing.T) {
	mockEmail := email.NewMockEmailProvider()
	repo := &mockSettingRepoForEmail{}

	handler := command.NewSendEmailNotificationHandler(repo, mockEmail)
	resp, err := handler.Handle(context.Background(), command.SendEmailNotificationCommand{
		UserID:  "user-1",
		To:      "user1@example.com",
		Subject: "Welcome",
		Body:    "Welcome to Gym Companion!",
	})

	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if resp.Status != "SENT" {
		t.Errorf("expected SENT status, got: %s", resp.Status)
	}

	sent := mockEmail.GetSentEmails()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent email, got %d", len(sent))
	}
	if sent[0].To != "user1@example.com" {
		t.Errorf("expected recipient user1@example.com, got %s", sent[0].To)
	}
}

func TestSendEmailNotificationHandler_DisabledByUser(t *testing.T) {
	mockEmail := email.NewMockEmailProvider()
	setting := aggregate.ReconstituteSetting(
		"user-1",
		true,
		false, // email disabled
		false,
		"",
		"",
		time.Now(),
		time.Now(),
	)
	repo := &mockSettingRepoForEmail{setting: setting}

	handler := command.NewSendEmailNotificationHandler(repo, mockEmail)
	resp, err := handler.Handle(context.Background(), command.SendEmailNotificationCommand{
		UserID:  "user-1",
		To:      "user1@example.com",
		Subject: "Welcome",
		Body:    "Hello",
	})

	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if resp.Status != "EMAIL_DISABLED_BY_USER" {
		t.Errorf("expected EMAIL_DISABLED_BY_USER, got %s", resp.Status)
	}
	if len(mockEmail.GetSentEmails()) != 0 {
		t.Errorf("expected 0 emails sent, got %d", len(mockEmail.GetSentEmails()))
	}

	// High priority email must bypass disabled settings
	respHigh, err := handler.Handle(context.Background(), command.SendEmailNotificationCommand{
		UserID:         "user-1",
		To:             "user1@example.com",
		Subject:        "High Priority Security Alert",
		Body:           "Your security alert",
		IsHighPriority: true,
	})
	if err != nil {
		t.Fatalf("expected nil error on high priority email, got: %v", err)
	}
	if respHigh.Status != "SENT" {
		t.Errorf("expected SENT for high priority email, got %s", respHigh.Status)
	}
	if len(mockEmail.GetSentEmails()) != 1 {
		t.Errorf("expected 1 email sent, got %d", len(mockEmail.GetSentEmails()))
	}
}

func TestSendEmailNotificationHandler_EmptyRecipient(t *testing.T) {
	handler := command.NewSendEmailNotificationHandler(nil, email.NewMockEmailProvider())
	_, err := handler.Handle(context.Background(), command.SendEmailNotificationCommand{
		UserID: "user-1",
		To:     "",
	})
	if err == nil {
		t.Fatal("expected error on empty recipient, got nil")
	}
}
