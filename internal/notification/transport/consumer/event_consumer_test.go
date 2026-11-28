package consumer_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
	"github.com/viethung213/gym-companion/internal/notification/domain/vo"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/email"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/sms"
	"github.com/viethung213/gym-companion/internal/notification/transport/consumer"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
)

type mockConsumerReader struct {
	msg       kafka.Message
	called    bool
	committed bool
	cancel    context.CancelFunc
}

func (m *mockConsumerReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	if m.called {
		if m.cancel != nil {
			m.cancel()
		}
		<-ctx.Done()
		return kafka.Message{}, ctx.Err()
	}
	m.called = true
	return m.msg, nil
}

func (m *mockConsumerReader) CommitMessages(ctx context.Context, msgs ...kafka.Message) error {
	m.committed = true
	return nil
}

type mockDLQWriter struct {
	written []kafka.Message
}

func (m *mockDLQWriter) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	m.written = append(m.written, msgs...)
	return nil
}

type mockDeviceRepo struct {
	devices []*aggregate.Device
}

func (m *mockDeviceRepo) Save(ctx context.Context, device *aggregate.Device) error { return nil }
func (m *mockDeviceRepo) GetActiveDevicesByUserID(ctx context.Context, userID string) ([]*aggregate.Device, error) {
	dt, _ := vo.NewDeviceType("IOS")
	dev := aggregate.ReconstituteDevice("dev-1", userID, "token-1", dt, true, time.Now(), time.Now(), time.Now())
	return []*aggregate.Device{dev}, nil
}
func (m *mockDeviceRepo) GetActiveDevicesByUserIDs(ctx context.Context, userIDs []string) ([]*aggregate.Device, error) {
	result := make([]*aggregate.Device, 0, len(userIDs))
	for _, uid := range userIDs {
		dt, _ := vo.NewDeviceType("IOS")
		dev := aggregate.ReconstituteDevice("dev-"+uid, uid, "token-"+uid, dt, true, time.Now(), time.Now(), time.Now())
		result = append(result, dev)
	}
	return result, nil
}
func (m *mockDeviceRepo) DeactivateTokens(ctx context.Context, tokens []string) error { return nil }

type mockNotificationRepo struct {
	items              []*aggregate.InAppNotification
	announcements      []*aggregate.SystemAnnouncement
	markedReadAnnounce []string
}

type mockFailingNotificationRepo struct {
	mockNotificationRepo
}

func (m *mockFailingNotificationRepo) Save(ctx context.Context, notif *aggregate.InAppNotification) error {
	return errors.New("database disk full")
}

type mockRetryableNotificationRepo struct {
	mockNotificationRepo
	failTimes int
	attempts  int
}

func (m *mockRetryableNotificationRepo) Save(ctx context.Context, notif *aggregate.InAppNotification) error {
	m.attempts++
	if m.attempts <= m.failTimes {
		return errors.New("database temporary connection error")
	}
	return m.mockNotificationRepo.Save(ctx, notif)
}

func (m *mockNotificationRepo) Save(ctx context.Context, notif *aggregate.InAppNotification) error {
	m.items = append(m.items, notif)
	return nil
}
func (m *mockNotificationRepo) BatchSave(ctx context.Context, notifs []*aggregate.InAppNotification) error {
	m.items = append(m.items, notifs...)
	return nil
}
func (m *mockNotificationRepo) SaveSystemAnnouncement(ctx context.Context, announcement *aggregate.SystemAnnouncement) error {
	m.announcements = append(m.announcements, announcement)
	return nil
}
func (m *mockNotificationRepo) MarkAnnouncementAsRead(ctx context.Context, announcementID, userID string) error {
	m.markedReadAnnounce = append(m.markedReadAnnounce, announcementID+":"+userID)
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
func (m *mockNotificationRepo) FindByUserAndEventID(ctx context.Context, userID, eventID string) (*aggregate.InAppNotification, error) {
	for _, it := range m.items {
		if it.UserID() == userID && it.EventID() == eventID {
			return it, nil
		}
	}
	return nil, nil
}

type mockOutboxLogRepo struct {
	savedLogs       []*port.OutboxLogRecord
	processedMap    map[string]bool
	statusMap       map[string]string
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

func (m *mockSettingRepoForConsumer) GetByUserIDs(ctx context.Context, userIDs []string) ([]*aggregate.Setting, error) {
	if m.setting != nil {
		return []*aggregate.Setting{m.setting}, nil
	}
	return nil, nil
}

func newMockOutboxLogRepo() *mockOutboxLogRepo {
	return &mockOutboxLogRepo{
		savedLogs:    make([]*port.OutboxLogRecord, 0),
		processedMap: make(map[string]bool),
		statusMap:    make(map[string]string),
	}
}

func (m *mockOutboxLogRepo) SaveLog(ctx context.Context, logRec *port.OutboxLogRecord) error {
	m.savedLogs = append(m.savedLogs, logRec)
	if m.statusMap == nil {
		m.statusMap = make(map[string]string)
	}
	m.statusMap[logRec.EventID] = logRec.Status
	return nil
}

func (m *mockOutboxLogRepo) LogProcessed(ctx context.Context, eventID, eventType, partitionKey string, payload []byte) (*port.OutboxLogRecord, error) {
	if m.logProcessedErr != nil {
		return nil, m.logProcessedErr
	}
	var existingRec *port.OutboxLogRecord
	for i := len(m.savedLogs) - 1; i >= 0; i-- {
		if m.savedLogs[i].EventID == eventID {
			existingRec = m.savedLogs[i]
			break
		}
	}
	if m.statusMap != nil && m.statusMap[eventID] == "FAILED" {
		m.statusMap[eventID] = "PROCESSING"
		if existingRec != nil {
			recCopy := *existingRec
			recCopy.Status = "PROCESSING"
			return &recCopy, nil
		}
		return &port.OutboxLogRecord{EventID: eventID, Status: "PROCESSING"}, nil
	}
	if m.processedMap[eventID] {
		return nil, nil
	}
	m.processedMap[eventID] = true
	if existingRec != nil {
		recCopy := *existingRec
		recCopy.Status = "PROCESSING"
		return &recCopy, nil
	}
	return &port.OutboxLogRecord{EventID: eventID, Status: "PROCESSING"}, nil
}

func (m *mockOutboxLogRepo) FetchFailedLogs(ctx context.Context, limit int) ([]*port.OutboxLogRecord, error) {
	return nil, nil
}

func (m *mockOutboxLogRepo) UpdateLogStatus(ctx context.Context, id string, status string, errMsg string) error {
	return nil
}

func (m *mockOutboxLogRepo) UpdateRetryLog(ctx context.Context, id string, retryCount int, nextRetryAt time.Time, status, errMsg string) error {
	return nil
}

type mockPushProvider struct {
	shouldFail         bool
	sendCount          int
	multicastTokens    [][]string
	topicPushes        []string
	subscribedTokens   []string
	unsubscribedTokens []string
}

func (m *mockPushProvider) SendPush(ctx context.Context, tokens []string, title, body string, data map[string]string) (*port.PushResponse, error) {
	m.sendCount++
	m.multicastTokens = append(m.multicastTokens, tokens)
	if m.shouldFail {
		return nil, errors.New("push failure")
	}
	return &port.PushResponse{SuccessCount: len(tokens)}, nil
}

func (m *mockPushProvider) SendPushToTopic(ctx context.Context, topic string, title, body string, data map[string]string) error {
	m.sendCount++
	m.topicPushes = append(m.topicPushes, topic)
	if m.shouldFail {
		return errors.New("push failure")
	}
	return nil
}

func (m *mockPushProvider) SubscribeToTopic(ctx context.Context, tokens []string, topic string) error {
	m.subscribedTokens = append(m.subscribedTokens, tokens...)
	return nil
}

func (m *mockPushProvider) UnsubscribeFromTopic(ctx context.Context, tokens []string, topic string) error {
	m.unsubscribedTokens = append(m.unsubscribedTokens, tokens...)
	return nil
}

func newTestConsumer(
	reader sharedKafka.MessageReader,
	devRepo repository.DeviceRepository,
	pushProvider port.PushProvider,
	notifRepo repository.NotificationRepository,
	settingRepo repository.SettingRepository,
	emailProvider port.EmailProvider,
	smsProvider port.SMSProvider,
	outboxLogRepo port.OutboxLogRepository,
) *consumer.NotificationEventConsumer {
	pushChannel := command.NewPushChannelDispatcher(devRepo, pushProvider, settingRepo, nil)
	inAppChannel := command.NewInAppChannelDispatcher(notifRepo)
	emailChannel := command.NewEmailChannelDispatcher(emailProvider, settingRepo)
	smsChannel := command.NewSMSChannelDispatcher(smsProvider, settingRepo)

	sendNotificationH := command.NewSendNotificationHandler(pushChannel, inAppChannel, emailChannel, smsChannel)
	selfH := consumer.NewSelfEventHandler(sendNotificationH, outboxLogRepo)
	extH := consumer.NewExternalEventHandler(sendNotificationH, outboxLogRepo)
	return consumer.NewNotificationEventConsumer(reader, selfH, extH)
}

func TestNotificationEventConsumer(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("Nil Reader Start", func(t *testing.T) {
		c := consumer.NewNotificationEventConsumer(nil, nil, nil)
		if c == nil {
			t.Fatal("got nil consumer, want non-nil")
		}
		c.Start(ctx)
	})

	t.Run("Invalid JSON CloudEvent", func(t *testing.T) {
		c := consumer.NewNotificationEventConsumer(nil, nil, nil)
		err := c.ProcessMessage(ctx, []byte("invalid-json"))
		if err == nil {
			t.Errorf("expected error for invalid json, got nil")
		}
	})

	t.Run("Non-whitelisted Event Type Ignored", func(t *testing.T) {
		c := consumer.NewNotificationEventConsumer(nil, nil, nil)
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
		selfH := consumer.NewSelfEventHandler(nil, newMockOutboxLogRepo())
		c := consumer.NewNotificationEventConsumer(nil, selfH, nil)
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
		selfH := consumer.NewSelfEventHandler(nil, newMockOutboxLogRepo())
		c := consumer.NewNotificationEventConsumer(nil, selfH, nil)
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

	t.Run("Missing CloudEvent ID Returns Error", func(t *testing.T) {
		selfH := consumer.NewSelfEventHandler(nil, newMockOutboxLogRepo())
		c := consumer.NewNotificationEventConsumer(nil, selfH, nil)
		payload := []byte(`{
			"id": "",
			"type": "contracts.generic.notification.v1.event.NormalPriorityNotificationRequested",
			"data": {"title": "No ID"}
		}`)
		err := c.ProcessMessage(ctx, payload)
		if err == nil {
			t.Errorf("expected error for missing event id, got nil")
		}
	})

	t.Run("Nil OutboxLogRepo Returns Error", func(t *testing.T) {
		selfH := consumer.NewSelfEventHandler(nil, nil)
		c := consumer.NewNotificationEventConsumer(nil, selfH, nil)
		payload := []byte(`{
			"id": "e-repo-nil",
			"type": "contracts.generic.notification.v1.event.NormalPriorityNotificationRequested",
			"data": {"title": "Nil Repo"}
		}`)
		err := c.ProcessMessage(ctx, payload)
		if err == nil {
			t.Errorf("expected error for nil outboxLogRepo, got nil")
		}
	})

	t.Run("Whitelisted Core Events Processing & Success Outbox Log", func(t *testing.T) {
		devRepo := &mockDeviceRepo{}
		notifRepo := &mockNotificationRepo{}
		provider := &mockPushProvider{shouldFail: false}
		outboxLogRepo := newMockOutboxLogRepo()

		c := newTestConsumer(nil, devRepo, provider, notifRepo, nil, nil, nil, outboxLogRepo)
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

	t.Run("NewPersonalRecordAchieved Formatter generates rich title, body, and dataMap", func(t *testing.T) {
		devRepo := &mockDeviceRepo{}
		notifRepo := &mockNotificationRepo{}
		pushProvider := &mockPushProvider{shouldFail: false}
		outboxLogRepo := newMockOutboxLogRepo()

		c := newTestConsumer(nil, devRepo, pushProvider, notifRepo, nil, nil, nil, outboxLogRepo)
		evStr := []byte(`{
			"id":"e-pr-formatted",
			"type":"contracts.core.workout_execution.v1.event.NewPersonalRecordAchieved",
			"data":{
				"userId":"usr-pr-1",
				"exerciseId":"ex-bench-press",
				"weight":100.0,
				"reps":5,
				"oneRepMax":116.7
			}
		}`)

		err := c.ProcessMessage(ctx, evStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(notifRepo.items) != 1 {
			t.Fatalf("expected 1 in-app notification item, got %d", len(notifRepo.items))
		}
		item := notifRepo.items[0]
		expectedTitle := "Kỷ lục cá nhân mới! 🏆"
		expectedBody := "Chúc mừng bạn vừa đạt PR mới: 100.0 kg x 5 reps (1RM ước tính: 116.7 kg)!"
		if item.Title() != expectedTitle {
			t.Errorf("got title %q, want %q", item.Title(), expectedTitle)
		}
		if item.Body() != expectedBody {
			t.Errorf("got body %q, want %q", item.Body(), expectedBody)
		}
		if item.Data()["exerciseId"] != "ex-bench-press" {
			t.Errorf("got exerciseId %q, want ex-bench-press", item.Data()["exerciseId"])
		}
		if item.Data()["weight"] != "100.0" {
			t.Errorf("got weight %q, want 100.0", item.Data()["weight"])
		}
		if item.Data()["reps"] != "5" {
			t.Errorf("got reps %q, want 5", item.Data()["reps"])
		}
		if item.Data()["oneRepMax"] != "116.7" {
			t.Errorf("got oneRepMax %q, want 116.7", item.Data()["oneRepMax"])
		}
	})

	t.Run("External Event Push Failure is Non-Fatal when In-App Succeeds", func(t *testing.T) {
		devRepo := &mockDeviceRepo{}
		notifRepo := &mockNotificationRepo{}
		pushProvider := &mockPushProvider{shouldFail: true} // Push provider will fail!
		outboxLogRepo := newMockOutboxLogRepo()

		c := newTestConsumer(nil, devRepo, pushProvider, notifRepo, nil, nil, nil, outboxLogRepo)
		evStr := []byte(`{
			"id":"e-pr-push-failed",
			"type":"contracts.core.workout_execution.v1.event.NewPersonalRecordAchieved",
			"data":{
				"userId":"usr-pr-push-fail",
				"weight":120.0,
				"reps":3
			}
		}`)

		err := c.ProcessMessage(ctx, evStr)
		if err != nil {
			t.Fatalf("expected nil error because in-app succeeded, got: %v", err)
		}

		if len(notifRepo.items) != 1 {
			t.Fatalf("expected 1 in-app notification item saved, got %d", len(notifRepo.items))
		}
		if len(outboxLogRepo.savedLogs) != 1 {
			t.Fatalf("expected 1 saved log, got %d", len(outboxLogRepo.savedLogs))
		}
		if outboxLogRepo.savedLogs[0].Status != "SUCCESS" {
			t.Errorf("expected outbox log status SUCCESS, got %s", outboxLogRepo.savedLogs[0].Status)
		}
	})

	t.Run("External Event In-App Failure Triggers Retry", func(t *testing.T) {
		devRepo := &mockDeviceRepo{}
		notifRepo := &mockFailingNotificationRepo{} // In-app save will fail!
		pushProvider := &mockPushProvider{shouldFail: false}
		outboxLogRepo := newMockOutboxLogRepo()

		c := newTestConsumer(nil, devRepo, pushProvider, notifRepo, nil, nil, nil, outboxLogRepo)
		evStr := []byte(`{
			"id":"e-pr-inapp-failed",
			"type":"contracts.core.workout_execution.v1.event.NewPersonalRecordAchieved",
			"data":{
				"userId":"usr-pr-inapp-fail",
				"weight":120.0,
				"reps":3
			}
		}`)

		err := c.ProcessMessage(ctx, evStr)
		if err == nil {
			t.Fatalf("expected error when in-app notification fails to save, got nil")
		}

		if len(outboxLogRepo.savedLogs) != 1 {
			t.Fatalf("expected 1 saved log, got %d", len(outboxLogRepo.savedLogs))
		}
		if outboxLogRepo.savedLogs[0].Status != "FAILED" {
			t.Errorf("expected outbox log status FAILED, got %s", outboxLogRepo.savedLogs[0].Status)
		}
	})

	t.Run("Duplicate Event Idempotency Check Skips Duplicate", func(t *testing.T) {
		devRepo := &mockDeviceRepo{}
		notifRepo := &mockNotificationRepo{}
		provider := &mockPushProvider{shouldFail: false}
		outboxLogRepo := newMockOutboxLogRepo()

		c := newTestConsumer(nil, devRepo, provider, notifRepo, nil, nil, nil, outboxLogRepo)
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

	t.Run("Push Provider Failure Slogs Error and Does Not Retry", func(t *testing.T) {
		devRepo := &mockDeviceRepo{}
		notifRepo := &mockNotificationRepo{}
		provider := &mockPushProvider{shouldFail: true}
		outboxLogRepo := newMockOutboxLogRepo()

		c := newTestConsumer(nil, devRepo, provider, notifRepo, nil, nil, nil, outboxLogRepo)
		evStr := []byte(`{"id":"e-fail","type":"contracts.generic.notification.v1.event.NormalPriorityNotificationRequested","data":{"userId":"usr-1"}}`)

		err := c.ProcessMessage(ctx, evStr)
		if err != nil {
			t.Fatalf("expected nil error on push failure (best effort without retry), got: %v", err)
		}

		if len(outboxLogRepo.savedLogs) != 1 {
			t.Fatalf("got %d saved logs, want 1", len(outboxLogRepo.savedLogs))
		}
		if got, want := outboxLogRepo.savedLogs[0].Status, "SUCCESS"; got != want {
			t.Errorf("got status %s, want %s", got, want)
		}
	})

	t.Run("Retry on Failure is Not Blocked by LogProcessed", func(t *testing.T) {
		devRepo := &mockDeviceRepo{}
		notifRepo := &mockRetryableNotificationRepo{failTimes: 1}
		provider := &mockPushProvider{shouldFail: false}
		outboxLogRepo := newMockOutboxLogRepo()

		c := newTestConsumer(nil, devRepo, provider, notifRepo, nil, nil, nil, outboxLogRepo)
		evStr := []byte(`{"id":"e-retry","type":"contracts.generic.notification.v1.event.NormalPriorityNotificationRequested","data":{"userId":"usr-1"}}`)

		// Attempt 1: Provider fails, status becomes FAILED
		err1 := c.ProcessMessage(ctx, evStr)
		if err1 == nil {
			t.Fatalf("expected error on attempt 1, got nil")
		}

		// Attempt 2 (Retry): Provider succeeds, verify it is NOT swallowed by duplicate check
		err2 := c.ProcessMessage(ctx, evStr)
		if err2 != nil {
			t.Fatalf("expected success on attempt 2 retry, got error: %v", err2)
		}

		if len(outboxLogRepo.savedLogs) != 2 {
			t.Fatalf("expected 2 saved logs (FAILED then SUCCESS), got %d", len(outboxLogRepo.savedLogs))
		}
		if outboxLogRepo.savedLogs[0].Status != "FAILED" {
			t.Errorf("expected first log to be FAILED, got %s", outboxLogRepo.savedLogs[0].Status)
		}
		if outboxLogRepo.savedLogs[1].Status != "SUCCESS" {
			t.Errorf("expected second log to be SUCCESS, got %s", outboxLogRepo.savedLogs[1].Status)
		}
	})

	t.Run("Three consecutive failures transition event to DEAD_LETTER and block redelivery", func(t *testing.T) {
		devRepo := &mockDeviceRepo{}
		notifRepo := &mockFailingNotificationRepo{}
		provider := &mockPushProvider{shouldFail: false}
		outboxLogRepo := newMockOutboxLogRepo()

		c := newTestConsumer(nil, devRepo, provider, notifRepo, nil, nil, nil, outboxLogRepo)
		evStr := []byte(`{"id":"e-dl-test","type":"contracts.generic.notification.v1.event.NormalPriorityNotificationRequested","data":{"userId":"usr-1"}}`)

		// Attempt 1: Fails -> status = FAILED, retry_count = 1, next_retry_at is ~10s
		err1 := c.ProcessMessage(ctx, evStr)
		if err1 == nil {
			t.Fatalf("expected error on attempt 1, got nil")
		}
		if len(outboxLogRepo.savedLogs) != 1 {
			t.Fatalf("expected 1 saved log, got %d", len(outboxLogRepo.savedLogs))
		}
		if outboxLogRepo.savedLogs[0].Status != "FAILED" || outboxLogRepo.savedLogs[0].RetryCount != 1 {
			t.Fatalf("attempt 1: got status %s, retry %d; want FAILED, 1", outboxLogRepo.savedLogs[0].Status, outboxLogRepo.savedLogs[0].RetryCount)
		}
		if outboxLogRepo.savedLogs[0].NextRetryAt.IsZero() {
			t.Fatalf("attempt 1: expected next_retry_at to be non-zero")
		}

		// Attempt 2: Fails -> status = FAILED, retry_count = 2, next_retry_at is ~30s
		err2 := c.ProcessMessage(ctx, evStr)
		if err2 == nil {
			t.Fatalf("expected error on attempt 2, got nil")
		}
		if len(outboxLogRepo.savedLogs) != 2 {
			t.Fatalf("expected 2 saved logs, got %d", len(outboxLogRepo.savedLogs))
		}
		if outboxLogRepo.savedLogs[1].Status != "FAILED" || outboxLogRepo.savedLogs[1].RetryCount != 2 {
			t.Fatalf("attempt 2: got status %s, retry %d; want FAILED, 2", outboxLogRepo.savedLogs[1].Status, outboxLogRepo.savedLogs[1].RetryCount)
		}

		// Attempt 3: Fails -> status = DEAD_LETTER, retry_count = 3
		err3 := c.ProcessMessage(ctx, evStr)
		if err3 == nil {
			t.Fatalf("expected error on attempt 3, got nil")
		}
		if len(outboxLogRepo.savedLogs) != 3 {
			t.Fatalf("expected 3 saved logs, got %d", len(outboxLogRepo.savedLogs))
		}
		if outboxLogRepo.savedLogs[2].Status != "DEAD_LETTER" || outboxLogRepo.savedLogs[2].RetryCount != 3 {
			t.Fatalf("attempt 3: got status %s, retry %d; want DEAD_LETTER, 3", outboxLogRepo.savedLogs[2].Status, outboxLogRepo.savedLogs[2].RetryCount)
		}

		// Attempt 4 (Kafka redelivery): Must be immediately skipped because status is DEAD_LETTER!
		providerSendCountBefore := provider.sendCount
		err4 := c.ProcessMessage(ctx, evStr)
		if err4 != nil {
			t.Fatalf("expected nil error on redelivery of DEAD_LETTER event, got: %v", err4)
		}
		if len(outboxLogRepo.savedLogs) != 3 {
			t.Fatalf("expected saved logs to remain 3 after redelivery, got %d", len(outboxLogRepo.savedLogs))
		}
		if provider.sendCount != providerSendCountBefore {
			t.Errorf("provider should not be called on redelivery of dead lettered event")
		}
	})

	t.Run("Broadcast Strategy via FCM Topic and System Announcement (Fast Strategy A)", func(t *testing.T) {
		devRepo := &mockDeviceRepo{}
		notifRepo := &mockNotificationRepo{}
		pushProvider := &mockPushProvider{shouldFail: false}
		outboxLogRepo := newMockOutboxLogRepo()

		c := newTestConsumer(nil, devRepo, pushProvider, notifRepo, nil, nil, nil, outboxLogRepo)

		broadcastJSON := map[string]interface{}{
			"id":   "evt-broadcast-strategy-a",
			"type": "contracts.generic.notification.v1.event.HighPriorityNotificationRequested",
			"data": map[string]interface{}{
				"target": map[string]interface{}{
					"allUsers": true,
				},
				"title":    "Toàn hệ thống bảo trì",
				"body":     "Hệ thống sẽ cập nhật trong 15 phút.",
				"channels": []string{"PUSH", "IN_APP"},
			},
		}
		bBytes, _ := json.Marshal(broadcastJSON)

		err := c.ProcessMessage(ctx, bBytes)
		if err != nil {
			t.Fatalf("unexpected error on broadcast dispatch: %v", err)
		}

		// 1. Verify Push to FCM Topic "all_users" called exactly once
		if len(pushProvider.topicPushes) != 1 || pushProvider.topicPushes[0] != "all_users" {
			t.Fatalf("expected pushProvider.topicPushes ['all_users'], got %v", pushProvider.topicPushes)
		}
		if pushProvider.sendCount != 1 {
			t.Errorf("expected pushProvider.sendCount 1, got %d", pushProvider.sendCount)
		}
		if len(pushProvider.multicastTokens) != 0 {
			t.Errorf("expected 0 multicast calls, got %d", len(pushProvider.multicastTokens))
		}

		// 2. Verify System Announcement saved once
		if len(notifRepo.announcements) != 1 {
			t.Fatalf("expected 1 system announcement saved, got %d", len(notifRepo.announcements))
		}
		if got, want := notifRepo.announcements[0].Title(), "Toàn hệ thống bảo trì"; got != want {
			t.Errorf("announcement title = %s, want %s", got, want)
		}
		if got, want := notifRepo.announcements[0].EventID(), "evt-broadcast-strategy-a"; got != want {
			t.Errorf("announcement eventID = %s, want %s", got, want)
		}

		// 3. Verify Outbox Log contains channels_dispatched: ["PUSH", "IN_APP"]
		if len(outboxLogRepo.savedLogs) != 1 {
			t.Fatalf("expected 1 saved log, got %d", len(outboxLogRepo.savedLogs))
		}
		lastLog := outboxLogRepo.savedLogs[0]
		if lastLog.Status != "SUCCESS" {
			t.Errorf("expected status SUCCESS, got %s", lastLog.Status)
		}
		var meta port.OutboxLogMetadata
		if err := json.Unmarshal(lastLog.Metadata, &meta); err != nil {
			t.Fatalf("failed to unmarshal metadata: %v", err)
		}
		hasPush := meta.ChannelsDispatched[0] == "PUSH" || meta.ChannelsDispatched[1] == "PUSH"
		hasInApp := meta.ChannelsDispatched[0] == "IN_APP" || meta.ChannelsDispatched[1] == "IN_APP"
		if len(meta.ChannelsDispatched) != 2 || !hasPush || !hasInApp {
			t.Errorf("expected dispatched to contain 'PUSH' and 'IN_APP', got %v", meta.ChannelsDispatched)
		}
	})

	t.Run("Group Strategy via Batch Devices and Multicast Push (Optimized Strategy B)", func(t *testing.T) {
		devRepo := &mockDeviceRepo{}
		notifRepo := &mockNotificationRepo{}
		pushProvider := &mockPushProvider{shouldFail: false}
		outboxLogRepo := newMockOutboxLogRepo()

		c := newTestConsumer(nil, devRepo, pushProvider, notifRepo, nil, nil, nil, outboxLogRepo)

		groupJSON := map[string]interface{}{
			"id":   "evt-group-strategy-b",
			"type": "contracts.generic.notification.v1.event.HighPriorityNotificationRequested",
			"data": map[string]interface{}{
				"target": map[string]interface{}{
					"userList": map[string]interface{}{
						"userIds": []string{"u1", "u2", "u3"},
					},
				},
				"title":    "Nhắc nhở nhóm tập",
				"body":     "Buổi tập nhóm sắp diễn ra lúc 18:00.",
				"channels": []string{"PUSH", "IN_APP"},
			},
		}
		gBytes, _ := json.Marshal(groupJSON)

		err := c.ProcessMessage(ctx, gBytes)
		if err != nil {
			t.Fatalf("unexpected error on group dispatch: %v", err)
		}

		// 1. Verify FCM Multicast called once with all 3 device tokens
		if len(pushProvider.multicastTokens) != 1 {
			t.Fatalf("expected 1 multicast push call, got %d", len(pushProvider.multicastTokens))
		}
		if len(pushProvider.multicastTokens[0]) != 3 {
			t.Errorf("expected 3 tokens in multicast call, got %d", len(pushProvider.multicastTokens[0]))
		}

		// 2. Verify BatchSave in-app notifications
		if len(notifRepo.items) != 3 {
			t.Fatalf("expected 3 in-app notifications batch saved, got %d", len(notifRepo.items))
		}
		for _, item := range notifRepo.items {
			if got, want := item.Title(), "Nhắc nhở nhóm tập"; got != want {
				t.Errorf("item title = %s, want %s", got, want)
			}
		}

		// 3. Verify Outbox Log contains channels_dispatched: ["PUSH", "IN_APP"]
		if len(outboxLogRepo.savedLogs) != 1 {
			t.Fatalf("expected 1 saved log, got %d", len(outboxLogRepo.savedLogs))
		}
		lastLog := outboxLogRepo.savedLogs[0]
		if lastLog.Status != "SUCCESS" {
			t.Errorf("expected status SUCCESS, got %s", lastLog.Status)
		}
	})

	t.Run("Multi-Channel Event with PUSH, EMAIL and SMS Dispatch", func(t *testing.T) {
		devRepo := &mockDeviceRepo{}
		notifRepo := &mockNotificationRepo{}
		pushProvider := &mockPushProvider{shouldFail: false}
		mockEmail := email.NewMockEmailProvider()
		mockSMS := sms.NewMockSMSProvider()
		outboxLogRepo := newMockOutboxLogRepo()
		c := newTestConsumer(nil, devRepo, pushProvider, notifRepo, nil, mockEmail, mockSMS, outboxLogRepo)

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

	t.Run("Multi-Channel Retry Skips Already Dispatched Channels (BUG-04 Fix)", func(t *testing.T) {
		devRepo := &mockDeviceRepo{}
		notifRepo := &mockNotificationRepo{}
		pushProvider := &mockPushProvider{shouldFail: false}
		mockEmail := email.NewMockEmailProvider()
		mockEmail.SetError(errors.New("smtp connection timeout")) // Email fails on first attempt
		mockSMS := sms.NewMockSMSProvider()
		outboxLogRepo := newMockOutboxLogRepo()
		c := newTestConsumer(nil, devRepo, pushProvider, notifRepo, nil, mockEmail, mockSMS, outboxLogRepo)

		eventJSON := map[string]interface{}{
			"id":     "evt-bug-04",
			"source": "services/custom-service",
			"type":   "contracts.generic.notification.v1.event.HighPriorityNotificationRequested",
			"data": map[string]interface{}{
				"target": map[string]interface{}{
					"userId": "user-bug-04",
				},
				"title":    "Security Alert",
				"body":     "Your account security alert",
				"channels": []string{"PUSH", "EMAIL", "SMS"},
				"data": map[string]string{
					"email": "user04@example.com",
					"phone": "+84988112233",
				},
			},
		}
		evBytes, _ := json.Marshal(eventJSON)

		// 1. Attempt 1: PUSH succeeds, EMAIL fails
		err1 := c.ProcessMessage(ctx, evBytes)
		if err1 == nil {
			t.Fatalf("expected error on attempt 1 due to email failure, got nil")
		}

		if pushProvider.sendCount != 1 {
			t.Fatalf("expected push to be called 1 time in attempt 1, got %d", pushProvider.sendCount)
		}

		lastSavedLog := outboxLogRepo.savedLogs[len(outboxLogRepo.savedLogs)-1]
		if lastSavedLog.Status != "FAILED" {
			t.Fatalf("expected outbox log status FAILED on attempt 1, got %s", lastSavedLog.Status)
		}

		var meta1 port.OutboxLogMetadata
		if err := json.Unmarshal(lastSavedLog.Metadata, &meta1); err != nil {
			t.Fatalf("failed to unmarshal metadata: %v", err)
		}
		// In attempt 1: PUSH succeeded, EMAIL failed, SMS succeeded -> dispatched contains PUSH and SMS (order independent due to parallel dispatch)
		hasPush := meta1.ChannelsDispatched[0] == "PUSH" || meta1.ChannelsDispatched[1] == "PUSH"
		hasSMS := meta1.ChannelsDispatched[0] == "SMS" || meta1.ChannelsDispatched[1] == "SMS"
		if len(meta1.ChannelsDispatched) != 2 || !hasPush || !hasSMS {
			t.Fatalf("expected metadata channels_dispatched to contain PUSH and SMS, got %+v", meta1.ChannelsDispatched)
		}

		// 2. Attempt 2 (Retry): Email server recovers, SMS succeeds
		mockEmail.SetError(nil)
		err2 := c.ProcessMessage(ctx, evBytes)
		if err2 != nil {
			t.Fatalf("expected success on attempt 2 retry, got error: %v", err2)
		}

		// KEY ASSERTION FOR BUG-04:
		// Push provider sendCount MUST STILL BE 1 (PUSH was skipped and NOT dispatched again)!
		if pushProvider.sendCount != 1 {
			t.Fatalf("CRITICAL BUG-04 REGRESSION: Push was dispatched %d times! Expected exactly 1 (duplicate dispatch)", pushProvider.sendCount)
		}

		// Verify Email and SMS sent successfully
		if len(mockEmail.GetSentEmails()) != 1 {
			t.Fatalf("expected 1 email sent, got %d", len(mockEmail.GetSentEmails()))
		}
		if len(mockSMS.GetSentMessages()) != 1 {
			t.Fatalf("expected 1 SMS sent, got %d", len(mockSMS.GetSentMessages()))
		}

		lastSavedLog2 := outboxLogRepo.savedLogs[len(outboxLogRepo.savedLogs)-1]
		if lastSavedLog2.Status != "SUCCESS" {
			t.Fatalf("expected outbox log status SUCCESS on attempt 2, got %s", lastSavedLog2.Status)
		}

		var meta2 port.OutboxLogMetadata
		if err := json.Unmarshal(lastSavedLog2.Metadata, &meta2); err != nil {
			t.Fatalf("failed to unmarshal retry metadata: %v", err)
		}
		if len(meta2.ChannelsDispatched) != 3 {
			t.Fatalf("expected all 3 channels dispatched in metadata, got %+v", meta2.ChannelsDispatched)
		}
	})

	t.Run("External_Event_Handler_Custom_Service_Registration", func(t *testing.T) {
		deviceRepo := &mockDeviceRepo{}
		notifRepo := &mockNotificationRepo{}
		pushProvider := &mockPushProvider{}
		outboxLogRepo := newMockOutboxLogRepo()

		pushChannel := command.NewPushChannelDispatcher(deviceRepo, pushProvider, nil, nil)
		inAppChannel := command.NewInAppChannelDispatcher(notifRepo)
		sendNotificationH := command.NewSendNotificationHandler(pushChannel, inAppChannel)
		extHandler := consumer.NewExternalEventHandler(sendNotificationH, outboxLogRepo)

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

		c := consumer.NewNotificationEventConsumer(nil, nil, extHandler)
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
		mockEmail := email.NewMockEmailProvider()
		mockSMS := sms.NewMockSMSProvider()
		outboxLogRepo := newMockOutboxLogRepo()
		c := newTestConsumer(nil, devRepo, pushProvider, notifRepo, settingRepo, mockEmail, mockSMS, outboxLogRepo)

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

	t.Run("Start With DLQ Writer Routes Failure To DLQ And Commits", func(t *testing.T) {
		runCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mockR := &mockConsumerReader{
			msg: kafka.Message{
				Topic: "notification.events.high-priority",
				Value: []byte("invalid-json-payload"),
			},
			cancel: cancel,
		}
		mockDLQ := &mockDLQWriter{}

		c := consumer.NewNotificationEventConsumer(mockR, nil, nil).WithDLQWriter(mockDLQ)
		c.Start(runCtx)

		if len(mockDLQ.written) != 1 {
			t.Errorf("expected 1 message in DLQ, got %d", len(mockDLQ.written))
		}
		if !mockR.committed {
			t.Errorf("expected message to be committed after DLQ routing")
		}
	})

	t.Run("Start Without DLQ Aborts Commit On Failure", func(t *testing.T) {
		runCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mockR := &mockConsumerReader{
			msg: kafka.Message{
				Topic: "notification.events.normal-priority",
				Value: []byte("invalid-json-payload"),
			},
			cancel: cancel,
		}

		c := consumer.NewNotificationEventConsumer(mockR, nil, nil)
		c.Start(runCtx)

		if mockR.committed {
			t.Errorf("expected message NOT to be committed when failure occurs without DLQ")
		}
	})
}
