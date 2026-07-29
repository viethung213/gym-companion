// Package port defines all outbound interfaces the Coaching application layer
// depends on. Concrete adapters live under infrastructure/.
package port

import (
	"context"
	"github.com/viethung213/gym-companion/internal/coaching/domain/event"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"time"
)

// Clock abstracts time.Now() for deterministic tests.
type Clock interface {
	Now() time.Time
}

// IDGenerator abstracts UUID/ULID generation.
type IDGenerator interface {
	NewID() string
}

// TransactionManager runs fn inside a database transaction. The context is
// wrapped so that repositories can pick up the tx handle from it.
type TransactionManager interface {
	WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error
}

// TxManager is an alias kept for parity with other bounded contexts.
type TxManager = TransactionManager

// RoadmapRepository persists the Roadmap aggregate tree.
type RoadmapRepository interface {
	Save(ctx context.Context, r *roadmap.Roadmap) error
	FindByID(ctx context.Context, roadmapID string) (*roadmap.Roadmap, error)
	FindActiveByUser(ctx context.Context, userID string) (*roadmap.Roadmap, error)
	ListByUser(ctx context.Context, userID string, status roadmap.Status, limit, offset int) ([]*roadmap.Roadmap, error)
	FindSessionByID(ctx context.Context, sessionPlanID string) (*roadmap.Roadmap, error)
}

// OutboxRecord mirrors coaching.outbox row structure.
type OutboxRecord struct {
	ID           string
	EventID      string
	EventType    string
	Payload      []byte
	PartitionKey string
	CreatedAt    time.Time
	Published    bool
	PublishedAt  *time.Time
}

// OutboxRepository manages rows in coaching.outbox and coaching.outbox_log.
type OutboxRepository interface {
	Save(ctx context.Context, record *OutboxRecord) error
	FetchUnpublished(ctx context.Context, limit int) ([]*OutboxRecord, error)
	MarkPublished(ctx context.Context, ids []string) error
	ExecuteInLock(ctx context.Context, lockID int64, fn func(txCtx context.Context) error) error
	// LogProcessed records that an inbound event has been consumed (D9 idempotency).
	// Returns (true, nil) if this is a new event that was recorded, (false, nil) if
	// the event_id was already present (duplicate delivery), or an error.
	LogProcessed(ctx context.Context, eventID, eventType, partitionKey string, payload []byte) (fresh bool, err error)
}

// OutboxWriter is a convenience port for enqueueing domain events into the outbox
// as CloudEvents envelopes, always inside the caller's transaction.
type OutboxWriter interface {
	Enqueue(ctx context.Context, partitionKey string, events ...event.Event) error
}

// BrokerPublisher publishes outbox events to Kafka.
type BrokerPublisher interface {
	PublishBatch(ctx context.Context, records []*OutboxRecord) error
}
