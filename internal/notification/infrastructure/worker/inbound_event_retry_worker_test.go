package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/worker"
)

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
