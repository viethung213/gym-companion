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
	RetryCount   int
	NextRetryAt  time.Time
	Metadata     []byte
}

type OutboxLogMetadata struct {
	ChannelsDispatched []string `json:"channels_dispatched,omitempty"`
}

type OutboxLogRepository interface {
	// LogProcessed records an inbound event in notification.outbox_log. Returns non-nil record on successful claim lock, (nil, nil) on duplicate event_id.
	LogProcessed(ctx context.Context, eventID, eventType, partitionKey string, payload []byte) (*OutboxLogRecord, error)
	// SaveLog updates the processing outcome (status, error message, metadata) of an inbound event in notification.outbox_log.
	SaveLog(ctx context.Context, record *OutboxLogRecord) error
	// FetchFailedLogs fetches inbound failed event logs for retry.
	FetchFailedLogs(ctx context.Context, limit int) ([]*OutboxLogRecord, error)
	// UpdateLogStatus updates the status and error message of an outbox log record.
	UpdateLogStatus(ctx context.Context, id string, status string, errMsg string) error
	// UpdateRetryLog updates retry attempt count, scheduled next retry time, status, and error message.
	UpdateRetryLog(ctx context.Context, id string, retryCount int, nextRetryAt time.Time, status, errMsg string) error
}
