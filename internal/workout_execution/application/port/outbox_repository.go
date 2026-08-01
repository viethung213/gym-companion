package port

import (
	"context"
	"time"
)

// OutboxRecord represents an event stored in the database outbox table.

type OutboxRecord struct {
	ID string `json:"id"`

	EventID string `json:"eventId"`

	AggregateType string `json:"aggregateType"`

	AggregateID string `json:"aggregateId"`

	EventType string `json:"eventType"`

	Payload []byte `json:"payload"`

	PartitionKey string `json:"partitionKey"`

	Published bool `json:"published"`

	CreatedAt time.Time `json:"createdAt"`

	PublishedAt *time.Time `json:"publishedAt,omitempty"`
}

// OutboxRepository defines methods for managing outbox entries.

type OutboxRepository interface {
	Save(ctx context.Context, record *OutboxRecord) error

	FetchUnpublished(ctx context.Context, limit int) ([]*OutboxRecord, error)

	MarkPublished(ctx context.Context, ids []string) error

	ProcessBatch(ctx context.Context, limit int, publishFn func(ctx context.Context, records []*OutboxRecord) error) error
}

// BrokerPublisher sends serialized outbox events to Kafka.

type BrokerPublisher interface {
	PublishBatch(ctx context.Context, events []*OutboxRecord) error
}

// OutboxWriter serializes domain events to CloudEvents and writes to Outbox.

type OutboxWriter interface {
	WriteEvents(ctx context.Context, aggregateType, aggregateID string, events []interface{}) error
}

// OutboxLogRecord represents an entry in the consumer outbox_log idempotency table.

type OutboxLogRecord struct {
	ID string

	EventID string

	EventType string

	Payload []byte

	PartitionKey string

	Status string

	ErrorMessage string
}

// OutboxLogRepository defines persistence for tracking consumer event idempotency.

type OutboxLogRepository interface {
	IsProcessed(ctx context.Context, eventID string) (bool, error)

	SaveLog(ctx context.Context, record *OutboxLogRecord) error
}
