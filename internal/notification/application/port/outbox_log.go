package port

import (
	"context"
	"time"
)

type OutboxLogRecord struct {
	ID           string
	EventID      string
	EventType    string
	Payload      []byte
	PartitionKey string
	ProcessedAt  time.Time
	Status       string
	ErrorMessage string
}

type OutboxLogRepository interface {
	// LogProcessed records an inbound event in notification.outbox_log. Returns (fresh=true, nil) on first record, (fresh=false, nil) on duplicate event_id.
	LogProcessed(ctx context.Context, eventID, eventType, partitionKey string, payload []byte, status, errMsg string) (bool, error)
	// SaveLog records an outbound worker log entry in notification.outbox_log.
	SaveLog(ctx context.Context, record *OutboxLogRecord) error
	// FetchFailedLogs fetches inbound failed event logs for retry.
	FetchFailedLogs(ctx context.Context, limit int) ([]*OutboxLogRecord, error)
	// UpdateLogStatus updates the status and error message of an outbox log record.
	UpdateLogStatus(ctx context.Context, id string, status string, errMsg string) error
}
