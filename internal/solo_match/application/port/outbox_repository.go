package port

import (
	"context"
	"time"
)

// OutboxRecord represents an outbox table entry to be delivered asynchronously.
type OutboxRecord struct {
	ID           string
	EventID      string
	EventType    string
	Payload      []byte
	PartitionKey string
}

// OutboxRepository defines the persistence port for transactional outbox records.
type OutboxRepository interface {
	Save(ctx context.Context, record *OutboxRecord) error
	SaveBatch(ctx context.Context, records []*OutboxRecord) error
	FetchUnpublished(ctx context.Context, limit int) ([]*OutboxRecord, error)
	ClaimBatch(ctx context.Context, limit int, lockDuration time.Duration) ([]*OutboxRecord, error)
	MarkAsPublished(ctx context.Context, ids []string) error
}
