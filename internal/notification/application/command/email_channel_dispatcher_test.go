package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/email"
)

type mockSettingRepoForEmail struct {
	setting *aggregate.Setting
}

func (m *mockSettingRepoForEmail) GetByUserID(ctx context.Context, userID string) (*aggregate.Setting, error) {
	return m.setting, nil
}

func (m *mockSettingRepoForEmail) GetByUserIDs(ctx context.Context, userIDs []string) ([]*aggregate.Setting, error) {
	if m.setting != nil {
		return []*aggregate.Setting{m.setting}, nil
	}
	return nil, nil
}

func (m *mockSettingRepoForEmail) Save(ctx context.Context, setting *aggregate.Setting) error {
	m.setting = setting
	return nil
}

func TestEmailChannelDispatcher_Channel(t *testing.T) {
	dispatcher := command.NewEmailChannelDispatcher(nil, nil)
	if dispatcher.Channel() != "EMAIL" {
		t.Errorf("expected channel EMAIL, got %s", dispatcher.Channel())
	}
}

func TestEmailChannelDispatcher_DirectSuccess(t *testing.T) {
	mockEmail := email.NewMockEmailProvider()
	repo := &mockSettingRepoForEmail{}

	dispatcher := command.NewEmailChannelDispatcher(mockEmail, repo)
	_, err := dispatcher.Dispatch(context.Background(), port.NotificationTarget{
		Scope:  port.ScopeDirect,
		UserID: "user-1",
	}, command.NotificationMessage{
		Title: "Welcome",
		Body:  "Welcome to Gym Companion!",
		Data:  map[string]string{"email": "user1@example.com"},
	})

	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	sent := mockEmail.GetSentEmails()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent email, got %d", len(sent))
	}
	if sent[0].To != "user1@example.com" || sent[0].Subject != "Welcome" || sent[0].Body != "Welcome to Gym Companion!" {
		t.Errorf("sent email mismatch: %+v", sent[0])
	}
}

func TestEmailChannelDispatcher_SkipNonDirectScopes(t *testing.T) {
	mockEmail := email.NewMockEmailProvider()
	dispatcher := command.NewEmailChannelDispatcher(mockEmail, nil)

	// 1. ScopeGroup must be skipped
	_, err := dispatcher.Dispatch(context.Background(), port.NotificationTarget{
		Scope:   port.ScopeGroup,
		UserIDs: []string{"user-1", "user-2"},
	}, command.NotificationMessage{
		Title: "Group Announcement",
		Body:  "Group notification",
		Data:  map[string]string{"email": "group@example.com"},
	})
	if err != nil {
		t.Fatalf("expected nil error on group scope skip, got: %v", err)
	}
	if len(mockEmail.GetSentEmails()) != 0 {
		t.Errorf("expected 0 emails sent for ScopeGroup, got %d", len(mockEmail.GetSentEmails()))
	}

	// 2. ScopeBroadcast must be skipped
	_, err = dispatcher.Dispatch(context.Background(), port.NotificationTarget{
		Scope: port.ScopeBroadcast,
	}, command.NotificationMessage{
		Title: "Broadcast Announcement",
		Body:  "Broadcast notification",
		Data:  map[string]string{"email": "all@example.com"},
	})
	if err != nil {
		t.Fatalf("expected nil error on broadcast scope skip, got: %v", err)
	}
	if len(mockEmail.GetSentEmails()) != 0 {
		t.Errorf("expected 0 emails sent for ScopeBroadcast, got %d", len(mockEmail.GetSentEmails()))
	}
}

func TestEmailChannelDispatcher_EmptyEmailOrNilProvider(t *testing.T) {
	mockEmail := email.NewMockEmailProvider()
	dispatcher := command.NewEmailChannelDispatcher(mockEmail, nil)

	// Missing email in data
	_, err := dispatcher.Dispatch(context.Background(), port.NotificationTarget{
		Scope:  port.ScopeDirect,
		UserID: "user-1",
	}, command.NotificationMessage{
		Title: "Test",
		Body:  "No email test",
		Data:  map[string]string{},
	})
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(mockEmail.GetSentEmails()) != 0 {
		t.Errorf("expected 0 emails sent, got %d", len(mockEmail.GetSentEmails()))
	}

	// Nil provider
	nilDispatcher := command.NewEmailChannelDispatcher(nil, nil)
	_, err = nilDispatcher.Dispatch(context.Background(), port.NotificationTarget{
		Scope:  port.ScopeDirect,
		UserID: "user-1",
	}, command.NotificationMessage{
		Title: "Test",
		Body:  "Nil provider test",
		Data:  map[string]string{"email": "user@example.com"},
	})
	if err != nil {
		t.Fatalf("expected nil error with nil provider, got: %v", err)
	}
}

func TestEmailChannelDispatcher_DisabledByUser(t *testing.T) {
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

	dispatcher := command.NewEmailChannelDispatcher(mockEmail, repo)
	_, err := dispatcher.Dispatch(context.Background(), port.NotificationTarget{
		Scope:  port.ScopeDirect,
		UserID: "user-1",
	}, command.NotificationMessage{
		Title: "Weekly Digest",
		Body:  "Hello",
		Data:  map[string]string{"recipient_email": "user1@example.com"},
	})

	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(mockEmail.GetSentEmails()) != 0 {
		t.Errorf("expected 0 emails sent when disabled, got %d", len(mockEmail.GetSentEmails()))
	}

	// High priority email must bypass disabled settings
	_, err = dispatcher.Dispatch(context.Background(), port.NotificationTarget{
		Scope:  port.ScopeDirect,
		UserID: "user-1",
	}, command.NotificationMessage{
		Title:          "Security Alert",
		Body:           "Your security alert",
		IsHighPriority: true,
		Data:           map[string]string{"to_email": "user1@example.com"},
	})
	if err != nil {
		t.Fatalf("expected nil error on high priority email, got: %v", err)
	}
	if len(mockEmail.GetSentEmails()) != 1 {
		t.Errorf("expected 1 email sent for high priority, got %d", len(mockEmail.GetSentEmails()))
	}
}
