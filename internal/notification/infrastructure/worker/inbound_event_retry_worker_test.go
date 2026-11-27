package worker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/worker"
)

type mockDispatcher struct {
	processedCount int
	shouldFail     bool
}

func (m *mockDispatcher) ProcessMessage(ctx context.Context, msgValue []byte) error {
	m.processedCount++
	if m.shouldFail {
		return errors.New("dispatch failure")
	}
	return nil
}

type mockFailedOutboxLogRepo struct {
	failedLogs []*port.OutboxLogRecord
}

func (m *mockFailedOutboxLogRepo) LogProcessed(ctx context.Context, eventID, eventType, partitionKey string, payload []byte, status, errMsg string) (bool, error) {
	return true, nil
}

func (m *mockFailedOutboxLogRepo) SaveLog(ctx context.Context, record *port.OutboxLogRecord) error {
	m.failedLogs = append(m.failedLogs, record)
	return nil
}

func (m *mockFailedOutboxLogRepo) FetchFailedLogs(ctx context.Context, limit int) ([]*port.OutboxLogRecord, error) {
	return m.failedLogs, nil
}

func (m *mockFailedOutboxLogRepo) UpdateLogStatus(ctx context.Context, id string, status string, errMsg string) error {
	for _, l := range m.failedLogs {
		if l.ID == id {
			l.Status = status
			l.ErrorMessage = errMsg
		}
	}
	return nil
}

func (m *mockFailedOutboxLogRepo) UpdateRetryLog(ctx context.Context, id string, retryCount int, nextRetryAt time.Time, status, errMsg string) error {
	for _, l := range m.failedLogs {
		if l.ID == id {
			l.RetryCount = retryCount
			l.NextRetryAt = nextRetryAt
			l.Status = status
			l.ErrorMessage = errMsg
		}
	}
	return nil
}

func TestInboundEventRetryWorker(t *testing.T) {
	t.Parallel()

	repo := &mockFailedOutboxLogRepo{
		failedLogs: []*port.OutboxLogRecord{
			{
				ID:           "log-1",
				EventID:      "e1",
				EventType:    "contracts.core.nutrition.v1.event.UpcomingMealReminder",
				Payload:      []byte(`{"id":"e1","type":"contracts.core.nutrition.v1.event.UpcomingMealReminder","data":{"userId":"u1","title":"Meal"}}`),
				PartitionKey: "u1",
				Status:       "FAILED",
				ErrorMessage: "transient network error",
			},
		},
	}

	w := worker.NewInboundEventRetryWorker(repo, nil, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_ = w.Start(ctx)
}

func TestInboundEventRetryWorker_RetryProgressionToDeadLetter(t *testing.T) {
	t.Parallel()

	// Initial record: 1st failure (retry_count = 0)
	rec := &port.OutboxLogRecord{
		ID:           "log-deadletter",
		EventID:      "e-dl",
		EventType:    "contracts.core.nutrition.v1.event.UpcomingMealReminder",
		Payload:      []byte(`{"id":"e-dl","type":"contracts.core.nutrition.v1.event.UpcomingMealReminder","data":{"userId":"u1","title":"Meal"}}`),
		PartitionKey: "u1",
		Status:       "FAILED",
		ErrorMessage: "push failed",
		RetryCount:   0,
	}

	repo := &mockFailedOutboxLogRepo{
		failedLogs: []*port.OutboxLogRecord{rec},
	}

	disp := &mockDispatcher{shouldFail: true}
	w := worker.NewInboundEventRetryWorker(repo, disp, 5*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	_ = w.Start(ctx)

	if disp.processedCount == 0 {
		t.Errorf("expected dispatcher to be called for retry")
	}
}

func TestInboundEventRetryWorker_RetrySuccessUpdatesStatus(t *testing.T) {
	t.Parallel()

	rec := &port.OutboxLogRecord{
		ID:           "log-success",
		EventID:      "e-succ",
		EventType:    "contracts.core.nutrition.v1.event.UpcomingMealReminder",
		Payload:      []byte(`{"id":"e-succ","type":"contracts.core.nutrition.v1.event.UpcomingMealReminder","data":{"userId":"u1","title":"Meal"}}`),
		PartitionKey: "u1",
		Status:       "FAILED",
		ErrorMessage: "push failed",
		RetryCount:   0,
	}

	repo := &mockFailedOutboxLogRepo{
		failedLogs: []*port.OutboxLogRecord{rec},
	}

	disp := &mockDispatcher{shouldFail: false}
	w := worker.NewInboundEventRetryWorker(repo, disp, 5*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	_ = w.Start(ctx)

	if rec.Status != "SUCCESS" {
		t.Errorf("expected status to be updated to SUCCESS, got %s", rec.Status)
	}
}
