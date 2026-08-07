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
	Published    bool
	CreatedAt    time.Time
	PublishedAt  *time.Time
}

type OutboxRepository interface {
	Save(ctx context.Context, record *OutboxRecord) error
	FetchUnpublished(ctx context.Context, limit int) ([]*OutboxRecord, error)
	ClaimBatch(ctx context.Context, limit int, lockDuration time.Duration) ([]*OutboxRecord, error)
	MarkPublished(ctx context.Context, ids []string) error
	ProcessBatch(ctx context.Context, limit int, publishFn func(ctx context.Context, records []*OutboxRecord) error) error
}

type BrokerPublisher interface {
	PublishBatch(ctx context.Context, records []*OutboxRecord) error
}
