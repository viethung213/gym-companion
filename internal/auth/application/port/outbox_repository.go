package port

import "context"

// OutboxRecord is a plain data transfer object representing a database outbox log entry.
type OutboxRecord struct {
	ID           string
	EventID      string
	EventType    string
	Payload      []byte
	PartitionKey string
}

// OutboxRepository defines the database persistence port for outbox event logging.
type OutboxRepository interface {
	SaveEvent(ctx context.Context, eventID string, eventType string, payload []byte, partitionKey string) error
	FetchUnpublished(ctx context.Context, limit int) ([]*OutboxRecord, error)
	MarkPublished(ctx context.Context, ids []string) error
	ExecuteInLock(ctx context.Context, lockID int64, fn func(ctx context.Context) error) error
}
