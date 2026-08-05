package port

import (
	"context"
	"time"
)

type OutboxRecord struct {
	ID           string
	EventID      string
	EventType    string
	Payload      []byte
	PartitionKey string
	CreatedAt    time.Time
	Published    bool
}

type OutboxRepository interface {
	Save(ctx context.Context, record *OutboxRecord) error
	SaveEvent(ctx context.Context, eventID, eventType, partitionKey string, payload interface{}) error
	FetchUnpublished(ctx context.Context, limit int) ([]OutboxRecord, error)
	ClaimBatch(ctx context.Context, limit int, lockDuration time.Duration) ([]OutboxRecord, error)
	MarkAsPublished(ctx context.Context, ids []string) error
	ProcessBatch(ctx context.Context, limit int, publishFn func(ctx context.Context, records []OutboxRecord) error) error
}
