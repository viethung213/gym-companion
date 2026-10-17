package consumer_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/transport/consumer"
)

type mockOutboxLogRepo struct {
	processedMap map[string]bool
	savedLogs    []*port.OutboxLogRecord
}

func newMockOutboxLogRepo() *mockOutboxLogRepo {
	return &mockOutboxLogRepo{
		processedMap: make(map[string]bool),
		savedLogs:    make([]*port.OutboxLogRecord, 0),
	}
}

func (m *mockOutboxLogRepo) SaveLog(ctx context.Context, record *port.OutboxLogRecord) error {
	m.savedLogs = append(m.savedLogs, record)
	if record.Status == "PROCESSED" {
		m.processedMap[record.EventID] = true
	}
	return nil
}

func (m *mockOutboxLogRepo) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	return m.processedMap[eventID], nil
}

func TestWorkoutEventConsumer_ProcessMessage_InvalidJSON(t *testing.T) {
	logRepo := newMockOutboxLogRepo()
	c := consumer.NewWorkoutEventConsumer(nil, nil, logRepo)

	msg := kafka.Message{
		Value: []byte("not-valid-json"),
	}

	err := c.ProcessMessage(context.Background(), &msg)
	assert.Error(t, err)
}

func TestWorkoutEventConsumer_ProcessMessage_UnrelatedEventIgnored(t *testing.T) {
	logRepo := newMockOutboxLogRepo()
	c := consumer.NewWorkoutEventConsumer(nil, nil, logRepo)

	envelope := consumer.CloudEvent{
		ID:   "evt-1",
		Type: "contracts.core.workout.v1.unrelatedEvent",
	}
	bytes, _ := json.Marshal(envelope)

	err := c.ProcessMessage(context.Background(), &kafka.Message{Value: bytes})
	assert.NoError(t, err)
}

func TestWorkoutEventConsumer_ProcessMessage_IdempotentDuplicate(t *testing.T) {
	logRepo := newMockOutboxLogRepo()
	logRepo.processedMap["evt-dup"] = true

	c := consumer.NewWorkoutEventConsumer(nil, nil, logRepo)

	envelope := consumer.CloudEvent{
		ID:   "evt-dup",
		Type: "contracts.core.workout_execution.v1.workoutSessionCompleted",
	}
	bytes, _ := json.Marshal(envelope)

	err := c.ProcessMessage(context.Background(), &kafka.Message{Value: bytes})
	assert.NoError(t, err)
}

func TestNutritionEventConsumer_ProcessMessage_IdempotentDuplicate(t *testing.T) {
	logRepo := newMockOutboxLogRepo()
	logRepo.processedMap["evt-nut-dup"] = true

	c := consumer.NewNutritionEventConsumer(nil, nil, nil, logRepo)

	envelope := consumer.CloudEvent{
		ID:   "evt-nut-dup",
		Type: "contracts.core.nutrition.v1.mealLogged",
	}
	bytes, _ := json.Marshal(envelope)

	err := c.ProcessMessage(context.Background(), &kafka.Message{Value: bytes})
	assert.NoError(t, err)
}

func TestUserRegisteredConsumer_ProcessMessage_IdempotentDuplicate(t *testing.T) {
	logRepo := newMockOutboxLogRepo()
	logRepo.processedMap["evt-auth-dup"] = true

	c := consumer.NewUserRegisteredConsumer(nil, nil, logRepo)

	envelope := consumer.CloudEvent{
		ID:   "evt-auth-dup",
		Type: "contracts.core.auth.v1.userRegistered",
	}
	bytes, _ := json.Marshal(envelope)

	err := c.ProcessMessage(context.Background(), &kafka.Message{Value: bytes})
	assert.NoError(t, err)
}

func TestConsumers_StartWithNilReader(t *testing.T) {
	logRepo := newMockOutboxLogRepo()
	wc := consumer.NewWorkoutEventConsumer(nil, nil, logRepo)
	nc := consumer.NewNutritionEventConsumer(nil, nil, nil, logRepo)
	uc := consumer.NewUserRegisteredConsumer(nil, nil, logRepo)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Calling Start with nil reader should return immediately without blocking
	wc.Start(ctx)
	nc.Start(ctx)
	uc.Start(ctx)
	pic := consumer.NewUserIdentityUpdatedConsumer(nil, nil, logRepo)
	pic.Start(ctx)
}

func TestUserIdentityUpdatedConsumer_ProcessMessage_IdempotentDuplicate(t *testing.T) {
	logRepo := newMockOutboxLogRepo()
	logRepo.processedMap["evt-profile-dup"] = true

	c := consumer.NewUserIdentityUpdatedConsumer(nil, nil, logRepo)

	envelope := consumer.CloudEvent{
		ID:   "evt-profile-dup",
		Type: "contracts.supporting.profile.v1.event.UserIdentityUpdated",
	}
	bytes, _ := json.Marshal(envelope)

	err := c.ProcessMessage(context.Background(), &kafka.Message{Value: bytes})
	assert.NoError(t, err)
}

func TestPersonalRecordConsumer_ProcessMessage_IdempotentDuplicate(t *testing.T) {
	logRepo := newMockOutboxLogRepo()
	logRepo.processedMap["evt-pr-dup"] = true

	c := consumer.NewPersonalRecordConsumer(nil, nil, nil, nil, nil, logRepo)

	envelope := consumer.CloudEvent{
		ID:   "evt-pr-dup",
		Type: "contracts.core.workout_execution.v1.newPersonalRecordAchieved",
	}
	bytes, _ := json.Marshal(envelope)

	err := c.ProcessMessage(context.Background(), &kafka.Message{Value: bytes})
	assert.NoError(t, err)
}

func TestResolveLocalDate(t *testing.T) {
	// 2026-10-13 22:30 UTC -> 2026-10-14 05:30 Asia/Ho_Chi_Minh (UTC+7)
	utcTime := time.Date(2026, 10, 13, 22, 30, 0, 0, time.UTC)
	localDate := consumer.ResolveLocalDate(utcTime)
	assert.Equal(t, "2026-10-14", localDate)
}
