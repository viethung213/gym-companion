package kafka_test

import (
	"context"
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/vo"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/kafka"
)

type mockOutboxLogRepo struct {
	processedEvents map[string]bool
	savedLogs       []*port.OutboxLogRecord
	logProcessedErr error
}

func newMockOutboxLogRepo() *mockOutboxLogRepo {
	return &mockOutboxLogRepo{
		processedEvents: make(map[string]bool),
	}
}

func (m *mockOutboxLogRepo) LogProcessed(ctx context.Context, eventID, eventType, partitionKey string, payload []byte, status, errMsg string) (bool, error) {
	if m.logProcessedErr != nil {
		return false, m.logProcessedErr
	}
	if m.processedEvents[eventID] {
		return false, nil
	}
	m.processedEvents[eventID] = true
	return true, nil
}

func (m *mockOutboxLogRepo) SaveLog(ctx context.Context, record *port.OutboxLogRecord) error {
	m.savedLogs = append(m.savedLogs, record)
	return nil
}

func (m *mockOutboxLogRepo) FetchFailedLogs(ctx context.Context, limit int) ([]*port.OutboxLogRecord, error) {
	return nil, nil
}

func (m *mockOutboxLogRepo) UpdateLogStatus(ctx context.Context, id string, status string, errMsg string) error {
	return nil
}

type mockDeviceRepo struct{}

func (m *mockDeviceRepo) Save(ctx context.Context, device *aggregate.Device) error { return nil }
func (m *mockDeviceRepo) GetActiveDevicesByUserID(ctx context.Context, userID string) ([]*aggregate.Device, error) {
	dev, _ := aggregate.NewDevice("dev-1", userID, "token-123", vo.DeviceTypeAndroid)
	return []*aggregate.Device{dev}, nil
}
func (m *mockDeviceRepo) DeactivateTokens(ctx context.Context, tokens []string) error { return nil }

type mockNotificationRepo struct{}

func (m *mockNotificationRepo) Save(ctx context.Context, item *aggregate.InAppNotification) error {
	return nil
}
func (m *mockNotificationRepo) ListByUserID(ctx context.Context, userID string, limit, offset int32) ([]*aggregate.InAppNotification, int32, error) {
	return nil, 0, nil
}
func (m *mockNotificationRepo) MarkAsRead(ctx context.Context, userID, notificationID string) error {
	return nil
}
func (m *mockNotificationRepo) MarkAllAsRead(ctx context.Context, userID string) error { return nil }

type mockPushProvider struct {
	shouldFail bool
}

func (m *mockPushProvider) SendPush(ctx context.Context, tokens []string, title, body string, data map[string]string) (*port.PushResponse, error) {
	if m.shouldFail {
		return nil, errors.New("provider network failure")
	}
	return &port.PushResponse{SuccessCount: len(tokens)}, nil
}

func TestNotificationEventConsumer(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("Nil Reader Start", func(t *testing.T) {
		consumer := kafka.NewNotificationEventConsumer(nil, nil, nil)
		if consumer == nil {
			t.Fatal("got nil consumer, want non-nil")
		}
		consumer.Start(ctx)
	})

	t.Run("Invalid JSON CloudEvent", func(t *testing.T) {
		consumer := kafka.NewNotificationEventConsumer(nil, nil, nil)
		err := consumer.ProcessMessage(ctx, []byte("invalid-json"))
		if err == nil {
			t.Errorf("expected error for invalid json, got nil")
		}
	})

	t.Run("Non-whitelisted Event Type Ignored", func(t *testing.T) {
		consumer := kafka.NewNotificationEventConsumer(nil, nil, nil)
		payload := []byte(`{
			"id": "e-1",
			"type": "contracts.unrelated.domain.v1.event.SomethingHappened",
			"data": {"userId": "usr-1"}
		}`)
		err := consumer.ProcessMessage(ctx, payload)
		if err != nil {
			t.Errorf("expected nil error for ignored event, got %v", err)
		}
	})

	t.Run("Invalid CloudEvent Data Payload", func(t *testing.T) {
		consumer := kafka.NewNotificationEventConsumer(nil, nil, nil)
		payload := []byte(`{
			"id": "e-2",
			"type": "contracts.generic.notification.v1.event.NotificationRequested",
			"data": "invalid-data-string"
		}`)
		err := consumer.ProcessMessage(ctx, payload)
		if err == nil {
			t.Errorf("expected error for invalid data payload, got nil")
		}
	})

	t.Run("Empty UserID Ignored", func(t *testing.T) {
		consumer := kafka.NewNotificationEventConsumer(nil, nil, nil)
		payload := []byte(`{
			"id": "e-3",
			"type": "contracts.generic.notification.v1.event.NotificationRequested",
			"data": {"title": "No User"}
		}`)
		err := consumer.ProcessMessage(ctx, payload)
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

		consumer := kafka.NewNotificationEventConsumer(nil, sendPushH, outboxLogRepo)

		events := []string{
			`{"id":"e-pr","type":"contracts.core.workout_execution.v1.event.NewPersonalRecordAchieved","data":{"userId":"usr-1","title":"PR","message":"New PR!"}}`,
			`{"id":"e-meal","type":"contracts.core.nutrition.v1.event.UpcomingMealReminder","data":{"userId":"usr-1"}}`,
			`{"id":"e-workout","type":"contracts.core.coaching.v1.event.UpcomingWorkoutReminder","data":{"userId":"usr-1"}}`,
			`{"id":"e-req","type":"contracts.generic.notification.v1.event.NotificationRequested","data":{"userId":"usr-1","body":"Custom Body"}}`,
		}

		for _, evStr := range events {
			err := consumer.ProcessMessage(ctx, []byte(evStr))
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

		consumer := kafka.NewNotificationEventConsumer(nil, sendPushH, outboxLogRepo)
		evStr := []byte(`{"id":"e-dup","type":"contracts.generic.notification.v1.event.NotificationRequested","data":{"userId":"usr-1"}}`)

		// First process -> success
		err1 := consumer.ProcessMessage(ctx, evStr)
		if err1 != nil {
			t.Fatalf("first process error: %v", err1)
		}

		// Second process (duplicate ID) -> skipped by idempotency check
		err2 := consumer.ProcessMessage(ctx, evStr)
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

		consumer := kafka.NewNotificationEventConsumer(nil, sendPushH, outboxLogRepo)
		evStr := []byte(`{"id":"e-fail","type":"contracts.generic.notification.v1.event.NotificationRequested","data":{"userId":"usr-1"}}`)

		err := consumer.ProcessMessage(ctx, evStr)
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

	t.Run("LogProcessed Error Handled Gracefully", func(t *testing.T) {
		devRepo := &mockDeviceRepo{}
		notifRepo := &mockNotificationRepo{}
		provider := &mockPushProvider{shouldFail: false}
		sendPushH := command.NewSendPushNotificationHandler(devRepo, notifRepo, nil, provider, nil, nil)
		outboxLogRepo := newMockOutboxLogRepo()
		outboxLogRepo.logProcessedErr = errors.New("db error")

		consumer := kafka.NewNotificationEventConsumer(nil, sendPushH, outboxLogRepo)
		evStr := []byte(`{"id":"e-err","type":"contracts.generic.notification.v1.event.NotificationRequested","data":{"userId":"usr-1"}}`)

		err := consumer.ProcessMessage(ctx, evStr)
		if err != nil {
			t.Fatalf("got unexpected error: %v", err)
		}
	})
}
