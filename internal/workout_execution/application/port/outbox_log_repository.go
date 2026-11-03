package port

import "context"

// OutboxLogRecord represents an entry in the consumer outbox_log idempotency table.
type OutboxLogRecord struct {
	ID           string
	EventID      string
	EventType    string
	Payload      []byte
	PartitionKey string
	Status       string
	ErrorMessage string
}

// OutboxLogRepository defines persistence for tracking consumer event idempotency.
type OutboxLogRepository interface {
	IsProcessed(ctx context.Context, eventID string) (bool, error)
	SaveLog(ctx context.Context, record *OutboxLogRecord) error
}
