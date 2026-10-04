package consumer_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/vo"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/email"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/sms"
	"github.com/viethung213/gym-companion/internal/notification/transport/consumer"
)

type mockDeviceRepo struct {
	devices []*aggregate.Device
}

func (m *mockDeviceRepo) Save(ctx context.Context, device *aggregate.Device) error { return nil }
func (m *mockDeviceRepo) GetActiveDevicesByUserID(ctx context.Context, userID string) ([]*aggregate.Device, error) {
	dt, _ := vo.NewDeviceType("IOS")
	dev := aggregate.ReconstituteDevice("dev-1", userID, "token-1", dt, true, time.Now(), time.Now(), time.Now())
	return []*aggregate.Device{dev}, nil
}
func (m *mockDeviceRepo) DeactivateTokens(ctx context.Context, tokens []string) error { return nil }

type mockNotificationRepo struct{}

func (m *mockNotificationRepo) Save(ctx context.Context, notif *aggregate.InAppNotification) error {
	return nil
}
func (m *mockNotificationRepo) FindByID(ctx context.Context, id string) (*aggregate.InAppNotification, error) {
	return nil, nil
}
func (m *mockNotificationRepo) ListByUserID(ctx context.Context, userID string, limit, offset int32) ([]*aggregate.InAppNotification, int32, error) {
	return nil, 0, nil
}
func (m *mockNotificationRepo) MarkAsRead(ctx context.Context, id, userID string) error {
	return nil
}
func (m *mockNotificationRepo) MarkAllAsRead(ctx context.Context, userID string) error {
	return nil
}

type mockOutboxLogRepo struct {
	savedLogs       []*port.OutboxLogRecord
	processedMap    map[string]bool
	logProcessedErr error
}

type mockSettingRepoForConsumer struct {
	setting *aggregate.Setting
}

func (m *mockSettingRepoForConsumer) Save(ctx context.Context, setting *aggregate.Setting) error {
	m.setting = setting
	return nil
}

func (m *mockSettingRepoForConsumer) GetByUserID(ctx context.Context, userID string) (*aggregate.Setting, error) {
	if m.setting != nil {
		return m.setting, nil
	}
	return nil, nil
}

func newMockOutboxLogRepo() *mockOutboxLogRepo {
	return &mockOutboxLogRepo{
		savedLogs:    make([]*port.OutboxLogRecord, 0),
		processedMap: make(map[string]bool),
	}
}

func (m *mockOutboxLogRepo) SaveLog(ctx context.Context, logRec *port.OutboxLogRecord) error {
	m.savedLogs = append(m.savedLogs, logRec)
	return nil
}

func (m *mockOutboxLogRepo) LogProcessed(ctx context.Context, eventID, eventType, partitionKey string, payload []byte, status, errorMessage string) (bool, error) {
	if m.logProcessedErr != nil {
		return false, m.logProcessedErr
	}
	if m.processedMap[eventID] {
		return false, nil
	}
	m.processedMap[eventID] = true
	return true, nil
}

func (m *mockOutboxLogRepo) FetchFailedLogs(ctx context.Context, limit int) ([]*port.OutboxLogRecord, error) {
	return nil, nil
}

func (m *mockOutboxLogRepo) UpdateLogStatus(ctx context.Context, id string, status string, errMsg string) error {
	return nil
}

type mockPushProvider struct {
	shouldFail bool
}

func (m *mockPushProvider) SendPush(ctx context.Context, tokens []string, title, body string, data map[string]string) (*port.PushResponse, error) {
	if m.shouldFail {
		return nil, errors.New("push failure")
	}
	return &port.PushResponse{SuccessCount: len(tokens)}, nil
}

func TestNotificationEventConsumer(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("Nil Reader Start", func(t *testing.T) {
		c := consumer.NewNotificationEventConsumer(nil, nil, nil, nil, nil)
		if c == nil {
			t.Fatal("got nil consumer, want non-nil")
		}
		c.Start(ctx)
	})

	t.Run("Invalid JSON CloudEvent", func(t *testing.T) {
		c := consumer.NewNotificationEventConsumer(nil, nil, nil, nil, nil)
		err := c.ProcessMessage(ctx, []byte("invalid-json"))
		if err == nil {
			t.Errorf("expected error for invalid json, got nil")
		}
	})

	t.Run("Non-whitelisted Event Type Ignored", func(t *testing.T) {
		c := consumer.NewNotificationEventConsumer(nil, nil, nil, nil, nil)
		payload := []byte(`{
			"id": "e-1",
			"type": "contracts.unrelated.domain.v1.event.SomethingHappened",
			"data": {"userId": "usr-1"}
		}`)
		err := c.ProcessMessage(ctx, payload)
		if err != nil {
			t.Errorf("expected nil error for ignored event, got %v", err)
		}
	})

	t.Run("Invalid CloudEvent Data Payload", func(t *testing.T) {
		c := consumer.NewNotificationEventConsumer(nil, nil, nil, nil, nil)
		payload := []byte(`{
			"id": "e-2",
			"type": "contracts.generic.notification.v1.event.NormalPriorityNotificationRequested",
			"data": "invalid-data-string"
		}`)
		err := c.ProcessMessage(ctx, payload)
		if err == nil {
			t.Errorf("expected error for invalid data payload, got nil")
		}
	})

	t.Run("Empty UserID Ignored", func(t *testing.T) {
		c := consumer.NewNotificationEventConsumer(nil, nil, nil, nil, nil)
		payload := []byte(`{
			"id": "e-3",
			"type": "contracts.generic.notification.v1.event.NormalPriorityNotificationRequested",
			"data": {"title": "No User"}
		}`)
		err := c.ProcessMessage(ctx, payload)
		if err != nil {
			t.Errorf("expected nil error for empty userId, got %v", err)
		}
	})

	t.Run("Whitelisted Core Events Processing & Success Outbox Log", func(t *testing.T) {
		devRepo := &mockDeviceRepo{}
		notifRepo := &mockNotificationRepo{}
		provider := &mockPushProvider{shouldFail: false}
		sendPushH := command.NewSendPushNotificationHandler(devRepo, notifRepo, nil, provider, nil, nil)
		outboxLogRepo := newMockOutboxLogRepo()

		c := consumer.NewNotificationEventConsumer(nil, sendPushH, nil, nil, outboxLogRepo)

		events := []string{
			`{"id":"e-pr","type":"contracts.core.workout_execution.v1.event.NewPersonalRecordAchieved","data":{"userId":"usr-1","title":"PR","message":"New PR!"}}`,
			`{"id":"e-meal","type":"contracts.core.nutrition.v1.event.UpcomingMealReminder","data":{"userId":"usr-1"}}`,
			`{"id":"e-workout","type":"contracts.core.coaching.v1.event.UpcomingWorkoutReminder","data":{"userId":"usr-1"}}`,
			`{"id":"e-req","type":"contracts.generic.notification.v1.event.NormalPriorityNotificationRequested","data":{"userId":"usr-1","body":"Custom Body"}}`,
		}

		for _, evStr := range events {
			err := c.ProcessMessage(ctx, []byte(evStr))
			if err != nil {
				t.Errorf("got unexpected error processing event: %v", err)
			}
		}

		if len(outboxLogRepo.savedLogs) != len(events) {
			t.Errorf("got %d saved logs, want %d", len(outboxLogRepo.savedLogs), len(events))
		}
		for _, l := range outboxLogRepo.savedLogs {
			if l.Status != "SUCCESS" {
				t.Errorf("got status %s, want SUCCESS", l.Status)
			}
		}
	})

	t.Run("Duplicate Event Idempotency Check Skips Duplicate", func(t *testing.T) {
		devRepo := &mockDeviceRepo{}
		notifRepo := &mockNotificationRepo{}
		provider := &mockPushProvider{shouldFail: false}
		sendPushH := command.NewSendPushNotificationHandler(devRepo, notifRepo, nil, provider, nil, nil)
		outboxLogRepo := newMockOutboxLogRepo()

		c := consumer.NewNotificationEventConsumer(nil, sendPushH, nil, nil, outboxLogRepo)
		evStr := []byte(`{"id":"e-dup","type":"contracts.generic.notification.v1.event.NormalPriorityNotificationRequested","data":{"userId":"usr-1"}}`)

		err1 := c.ProcessMessage(ctx, evStr)
		if err1 != nil {
			t.Fatalf("first process error: %v", err1)
		}

		err2 := c.ProcessMessage(ctx, evStr)
		if err2 != nil {
			t.Fatalf("second process error: %v", err2)
		}

		if len(outboxLogRepo.savedLogs) != 1 {
			t.Errorf("got %d saved logs, want 1 (duplicate should be skipped)", len(outboxLogRepo.savedLogs))
		}
	})

	t.Run("Push Provider Failure Saves Log Status FAILED", func(t *testing.T) {
		devRepo := &mockDeviceRepo{}
		notifRepo := &mockNotificationRepo{}
		provider := &mockPushProvider{shouldFail: true}
		sendPushH := command.NewSendPushNotificationHandler(devRepo, notifRepo, nil, provider, nil, nil)
		outboxLogRepo := newMockOutboxLogRepo()

		c := consumer.NewNotificationEventConsumer(nil, sendPushH, nil, nil, outboxLogRepo)
		evStr := []byte(`{"id":"e-fail","type":"contracts.generic.notification.v1.event.NormalPriorityNotificationRequested","data":{"userId":"usr-1"}}`)

		err := c.ProcessMessage(ctx, evStr)
		if err == nil {
			t.Fatalf("expected push failure error, got nil")
		}

		if len(outboxLogRepo.savedLogs) != 1 {
			t.Fatalf("got %d saved logs, want 1", len(outboxLogRepo.savedLogs))
		}
		if got, want := outboxLogRepo.savedLogs[0].Status, "FAILED"; got != want {
			t.Errorf("got status %s, want %s", got, want)
		}
	})

	t.Run("Multi-Channel Event with PUSH, EMAIL and SMS Dispatch", func(t *testing.T) {
		devRepo := &mockDeviceRepo{}
		notifRepo := &mockNotificationRepo{}
		pushProvider := &mockPushProvider{shouldFail: false}
		sendPushH := command.NewSendPushNotificationHandler(devRepo, notifRepo, nil, pushProvider, nil, nil)

		mockEmail := email.NewMockEmailProvider()
		sendEmailH := command.NewSendEmailNotificationHandler(nil, mockEmail)

		mockSMS := sms.NewMockSMSProvider()
		sendSMSH := command.NewSendSMSNotificationHandler(nil, mockSMS)

		outboxLogRepo := newMockOutboxLogRepo()
		c := consumer.NewNotificationEventConsumer(nil, sendPushH, sendEmailH, sendSMSH, outboxLogRepo)

		eventJSON := map[string]interface{}{
			"id":     "evt-multi-123",
			"source": "services/custom-service",
			"type":   "contracts.generic.notification.v1.event.HighPriorityNotificationRequested",
			"data": map[string]interface{}{
				"target": map[string]interface{}{
					"userId": "user-456",
				},
				"title":    "Account Alert",
				"body":     "Your account security alert",
				"channels": []string{"PUSH", "EMAIL", "SMS"},
				"data": map[string]string{
					"email": "user456@example.com",
					"phone": "+84988776655",
				},
			},
		}
		evBytes, _ := json.Marshal(eventJSON)

		err := c.ProcessMessage(ctx, evBytes)
		if err != nil {
			t.Fatalf("expected success processing multi-channel event, got: %v", err)
		}

		// Verify Email Sent
		sentEmails := mockEmail.GetSentEmails()
		if len(sentEmails) != 1 {
			t.Fatalf("expected 1 email sent, got %d", len(sentEmails))
		}
		if sentEmails[0].To != "user456@example.com" || sentEmails[0].Subject != "Account Alert" {
			t.Errorf("unexpected email content: %+v", sentEmails[0])
		}

		// Verify SMS Sent
		sentSMS := mockSMS.GetSentMessages()
		if len(sentSMS) != 1 {
			t.Fatalf("expected 1 SMS sent, got %d", len(sentSMS))
		}
		if sentSMS[0].To != "+84988776655" || sentSMS[0].Message != "Your account security alert" {
			t.Errorf("unexpected SMS content: %+v", sentSMS[0])
		}

		// Verify Outbox Log
		if len(outboxLogRepo.savedLogs) != 1 {
			t.Fatalf("expected 1 outbox log, got %d", len(outboxLogRepo.savedLogs))
		}
		if outboxLogRepo.savedLogs[0].Status != "SUCCESS" {
			t.Errorf("expected outbox log status SUCCESS, got: %s", outboxLogRepo.savedLogs[0].Status)
		}
	})

	t.Run("External_Event_Handler_Custom_Service_Registration", func(t *testing.T) {
		deviceRepo := &mockDeviceRepo{}
		notifRepo := &mockNotificationRepo{}
		pushProvider := &mockPushProvider{}
		sendPushH := command.NewSendPushNotificationHandler(deviceRepo, notifRepo, nil, pushProvider, nil, nil)
		outboxLogRepo := newMockOutboxLogRepo()

		extHandler := consumer.NewExternalEventHandler(sendPushH, outboxLogRepo)

		// Register a new service event (e.g., Billing Service)
		newEventType := "contracts.core.billing.v1.event.SubscriptionRenewed"
		extHandler.RegisterRule(newEventType, consumer.ExternalEventRule{
			DefaultTitle: "Gia hạn gói tập thành công! 🎉",
			DefaultBody:  "Gói hội viên của bạn đã được gia hạn tự động.",
		})

		if !extHandler.CanHandle(newEventType) {
			t.Fatalf("expected extHandler to handle newly registered event %s", newEventType)
		}

		eventJSON := map[string]interface{}{
			"id":     "evt-bill-999",
			"source": "services/billing",
			"type":   newEventType,
			"data": map[string]interface{}{
				"userId": "user-789",
			},
		}
		evBytes, _ := json.Marshal(eventJSON)

		c := consumer.NewNotificationEventConsumer(nil, sendPushH, nil, nil, outboxLogRepo)
		c.ExternalHandler().RegisterRule(newEventType, consumer.ExternalEventRule{
			DefaultTitle: "Gia hạn gói tập thành công! 🎉",
			DefaultBody:  "Gói hội viên của bạn đã được gia hạn tự động.",
		})

		cErr := c.ProcessMessage(ctx, evBytes)
		if cErr != nil {
			t.Fatalf("unexpected error: %v", cErr)
		}
		if len(outboxLogRepo.savedLogs) != 1 {
			t.Fatalf("expected 1 log for custom service event, got %d", len(outboxLogRepo.savedLogs))
		}
		if outboxLogRepo.savedLogs[0].Status != "SUCCESS" {
			t.Errorf("expected status SUCCESS, got %s", outboxLogRepo.savedLogs[0].Status)
		}
	})

	t.Run("HighPriorityNotificationRequested bypasses disabled notification settings", func(t *testing.T) {
		ctx := context.Background()
		userID := "user-suppressed-all"

		// Settings: all channels disabled and in quiet hours
		disabledSetting, _ := aggregate.NewDefaultSetting(userID)
		_ = disabledSetting.Update(false, false, false, "00:00", "23:59")
		settingRepo := &mockSettingRepoForConsumer{setting: disabledSetting}

		devRepo := &mockDeviceRepo{}
		notifRepo := &mockNotificationRepo{}
		pushProvider := &mockPushProvider{shouldFail: false}
		sendPushH := command.NewSendPushNotificationHandler(devRepo, notifRepo, settingRepo, pushProvider, nil, nil)

		mockEmail := email.NewMockEmailProvider()
		sendEmailH := command.NewSendEmailNotificationHandler(settingRepo, mockEmail)

		mockSMS := sms.NewMockSMSProvider()
		sendSMSH := command.NewSendSMSNotificationHandler(settingRepo, mockSMS)

		outboxLogRepo := newMockOutboxLogRepo()
		c := consumer.NewNotificationEventConsumer(nil, sendPushH, sendEmailH, sendSMSH, outboxLogRepo)

		// 1. First test: NormalPriorityNotificationRequested should be suppressed!
		normalEventJSON := map[string]interface{}{
			"id":     "evt-normal-suppressed",
			"source": "services/custom-service",
			"type":   "contracts.generic.notification.v1.event.NormalPriorityNotificationRequested",
			"data": map[string]interface{}{
				"target": map[string]interface{}{
					"userId": userID,
				},
				"title":    "Daily Tip",
				"body":     "Drink more water",
				"channels": []string{"PUSH", "EMAIL", "SMS"},
				"data": map[string]string{
					"email": "user@example.com",
					"phone": "+84999888777",
				},
			},
		}
		normalBytes, _ := json.Marshal(normalEventJSON)
		if err := c.ProcessMessage(ctx, normalBytes); err != nil {
			t.Fatalf("unexpected error processing normal event: %v", err)
		}
		if len(mockEmail.GetSentEmails()) != 0 {
			t.Errorf("expected 0 emails sent for normal priority with disabled settings, got %d", len(mockEmail.GetSentEmails()))
		}
		if len(mockSMS.GetSentMessages()) != 0 {
			t.Errorf("expected 0 SMS sent for normal priority with disabled settings, got %d", len(mockSMS.GetSentMessages()))
		}

		// 2. Second test: HighPriorityNotificationRequested must BYPASS and send all channels!
		highEventJSON := map[string]interface{}{
			"id":     "evt-high-bypass",
			"source": "services/auth-service",
			"type":   "contracts.generic.notification.v1.event.HighPriorityNotificationRequested",
			"data": map[string]interface{}{
				"target": map[string]interface{}{
					"userId": userID,
				},
				"title":    "Security Alert",
				"body":     "New login from unknown location",
				"channels": []string{"PUSH", "EMAIL", "SMS"},
				"data": map[string]string{
					"email": "user@example.com",
					"phone": "+84999888777",
				},
			},
		}
		highBytes, _ := json.Marshal(highEventJSON)
		if err := c.ProcessMessage(ctx, highBytes); err != nil {
			t.Fatalf("unexpected error processing high event: %v", err)
		}

		if len(mockEmail.GetSentEmails()) != 1 {
			t.Errorf("expected 1 email sent for high priority bypassing settings, got %d", len(mockEmail.GetSentEmails()))
		}
		if len(mockSMS.GetSentMessages()) != 1 {
			t.Errorf("expected 1 SMS sent for high priority bypassing settings, got %d", len(mockSMS.GetSentMessages()))
		}
	})
}
