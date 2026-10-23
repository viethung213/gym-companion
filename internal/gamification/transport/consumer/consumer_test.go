package consumer_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/viethung213/gym-companion/internal/gamification/application/command"
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

type mockNutritionHandler struct {
	called bool
	err    error
}

func (m *mockNutritionHandler) Handle(ctx context.Context, cmd command.ProcessNutritionActivityCommand) error {
	m.called = true
	return m.err
}

type mockPRHandler struct {
	called bool
	err    error
}

func (m *mockPRHandler) Handle(ctx context.Context, cmd command.ProcessPersonalRecordCommand) error {
	m.called = true
	return m.err
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

	c := consumer.NewNutritionEventConsumer(nil, nil, logRepo)

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
	nc := consumer.NewNutritionEventConsumer(nil, nil, logRepo)
	uc := consumer.NewUserRegisteredConsumer(nil, nil, logRepo)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	wc.Start(ctx)
	nc.Start(ctx)
	uc.Start(ctx)
	pic := consumer.NewUserIdentityUpdatedConsumer(nil, nil, logRepo)
	pic.Start(ctx)
	prc := consumer.NewPersonalRecordConsumer(nil, nil, logRepo)
	prc.Start(ctx)
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

	c := consumer.NewPersonalRecordConsumer(nil, nil, logRepo)

	envelope := consumer.CloudEvent{
		ID:   "evt-pr-dup",
		Type: "contracts.core.workout_execution.v1.newPersonalRecordAchieved",
	}
	bytes, _ := json.Marshal(envelope)

	err := c.ProcessMessage(context.Background(), &kafka.Message{Value: bytes})
	assert.NoError(t, err)
}

func TestNutritionEventConsumer_ProcessMessage_Success(t *testing.T) {
	logRepo := newMockOutboxLogRepo()
	handler := &mockNutritionHandler{}

	c := consumer.NewNutritionEventConsumer(nil, handler, logRepo)

	payload := map[string]interface{}{
		"userId":       "usr-nut-1",
		"activityDate": "2026-10-22",
	}
	pBytes, _ := json.Marshal(payload)
	envelope := consumer.CloudEvent{
		ID:   "evt-nut-100",
		Type: "contracts.core.nutrition.v1.mealLogged",
		Data: pBytes,
	}
	bytes, _ := json.Marshal(envelope)

	err := c.ProcessMessage(context.Background(), &kafka.Message{
		Key:   []byte("usr-nut-1"),
		Value: bytes,
	})
	assert.NoError(t, err)
	assert.True(t, handler.called)
	assert.True(t, logRepo.processedMap["evt-nut-100"])
}

func TestNutritionEventConsumer_ProcessMessage_ErrorLogged(t *testing.T) {
	logRepo := newMockOutboxLogRepo()
	handler := &mockNutritionHandler{err: assert.AnError}

	c := consumer.NewNutritionEventConsumer(nil, handler, logRepo)

	payload := map[string]interface{}{
		"userId":       "usr-nut-2",
		"activityDate": "2026-10-22",
	}
	pBytes, _ := json.Marshal(payload)
	envelope := consumer.CloudEvent{
		ID:   "evt-nut-200",
		Type: "contracts.core.nutrition.v1.mealLogged",
		Data: pBytes,
	}
	bytes, _ := json.Marshal(envelope)

	err := c.ProcessMessage(context.Background(), &kafka.Message{
		Key:   []byte("usr-nut-2"),
		Value: bytes,
	})
	assert.ErrorIs(t, err, assert.AnError)
	assert.True(t, handler.called)
	assert.False(t, logRepo.processedMap["evt-nut-200"])
	assert.Len(t, logRepo.savedLogs, 1)
	assert.Equal(t, "FAILED", logRepo.savedLogs[0].Status)
}

func TestPersonalRecordConsumer_ProcessMessage_Success(t *testing.T) {
	logRepo := newMockOutboxLogRepo()
	handler := &mockPRHandler{}

	c := consumer.NewPersonalRecordConsumer(nil, handler, logRepo)

	payload := map[string]interface{}{
		"userId":     "usr-pr-1",
		"exerciseId": "ex-squat",
		"weight":     120.0,
		"reps":       5,
	}
	pBytes, _ := json.Marshal(payload)
	envelope := consumer.CloudEvent{
		ID:   "evt-pr-100",
		Type: "contracts.core.workout_execution.v1.newPersonalRecordAchieved",
		Data: pBytes,
	}
	bytes, _ := json.Marshal(envelope)

	err := c.ProcessMessage(context.Background(), &kafka.Message{
		Key:   []byte("usr-pr-1"),
		Value: bytes,
	})
	assert.NoError(t, err)
	assert.True(t, handler.called)
	assert.True(t, logRepo.processedMap["evt-pr-100"])
}

func TestPersonalRecordConsumer_ProcessMessage_ErrorLogged(t *testing.T) {
	logRepo := newMockOutboxLogRepo()
	handler := &mockPRHandler{err: assert.AnError}

	c := consumer.NewPersonalRecordConsumer(nil, handler, logRepo)

	payload := map[string]interface{}{
		"userId":     "usr-pr-2",
		"exerciseId": "ex-bench",
		"weight":     100.0,
		"reps":       3,
	}
	pBytes, _ := json.Marshal(payload)
	envelope := consumer.CloudEvent{
		ID:   "evt-pr-200",
		Type: "contracts.core.workout_execution.v1.newPersonalRecordAchieved",
		Data: pBytes,
	}
	bytes, _ := json.Marshal(envelope)

	err := c.ProcessMessage(context.Background(), &kafka.Message{
		Key:   []byte("usr-pr-2"),
		Value: bytes,
	})
	assert.ErrorIs(t, err, assert.AnError)
	assert.True(t, handler.called)
	assert.False(t, logRepo.processedMap["evt-pr-200"])
	assert.Len(t, logRepo.savedLogs, 1)
	assert.Equal(t, "FAILED", logRepo.savedLogs[0].Status)
}
