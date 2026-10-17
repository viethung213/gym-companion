package port

import (
	"context"
	"time"
)

// OutboxRecord represents an event stored in the gamification.outbox table.
type OutboxRecord struct {
	ID            string     `json:"id"`
	EventID       string     `json:"eventId"`
	AggregateType string     `json:"aggregateType"`
	AggregateID   string     `json:"aggregateId"`
	EventType     string     `json:"eventType"`
	Payload       []byte     `json:"payload"`
	PartitionKey  string     `json:"partitionKey"`
	Published     bool       `json:"published"`
	CreatedAt     time.Time  `json:"createdAt"`
	PublishedAt   *time.Time `json:"publishedAt,omitempty"`
}

// OutboxRepository defines methods for managing outbox entries.
type OutboxRepository interface {
	Save(ctx context.Context, record *OutboxRecord) error
	ClaimBatch(ctx context.Context, limit int, lockDuration time.Duration) ([]*OutboxRecord, error)
	MarkAsPublished(ctx context.Context, ids []string) error
	MarkAsFailed(ctx context.Context, ids []string, lastErr error) error
	ProcessBatch(ctx context.Context, limit int, publishFn func(ctx context.Context, records []*OutboxRecord) error) error
}

// BrokerPublisher sends serialized outbox events to Kafka broker.
type BrokerPublisher interface {
	PublishBatch(ctx context.Context, events []*OutboxRecord) error
}

// OutboxWriter serializes domain events to CloudEvents and writes to Outbox.
type OutboxWriter interface {
	WriteEvents(ctx context.Context, aggregateType, aggregateID string, events []any) error
}

// OutboxLogRecord represents an entry in the consumer outbox_log idempotency table.
type OutboxLogRecord struct {
	ID           string `json:"id"`
	EventID      string `json:"eventId"`
	EventType    string `json:"eventType"`
	Payload      []byte `json:"payload"`
	PartitionKey string `json:"partitionKey"`
	Status       string `json:"status"`
	ErrorMessage string `json:"errorMessage"`
}

// OutboxLogRepository defines persistence for tracking consumer event idempotency.
type OutboxLogRepository interface {
	IsProcessed(ctx context.Context, eventID string) (bool, error)
	SaveLog(ctx context.Context, record *OutboxLogRecord) error
}
