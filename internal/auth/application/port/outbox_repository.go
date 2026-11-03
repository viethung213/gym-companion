package port

import (
	"context"
	"time"
)

// OutboxRecord is a plain data transfer object representing a database outbox log entry.
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

// OutboxRepository defines the database persistence port for outbox event logging.
type OutboxRepository interface {
	Save(ctx context.Context, record *OutboxRecord) error
	SaveEvent(ctx context.Context, eventID string, eventType string, payload []byte, partitionKey string) error
	FetchUnpublished(ctx context.Context, limit int) ([]*OutboxRecord, error)
	ClaimBatch(ctx context.Context, limit int, lockDuration time.Duration) ([]*OutboxRecord, error)
	MarkPublished(ctx context.Context, ids []string) error
	ProcessBatch(ctx context.Context, limit int, publishFn func(ctx context.Context, records []*OutboxRecord) error) error
}
