package port

import (
	"context"

	"github.com/viethung213/gym-companion/internal/auth/domain/event"
)

// OutboxWriter is the port for serializing and persisting domain events
// into the transactional outbox table within the same database transaction.
// It is used exclusively by Application Layer command handlers.
type OutboxWriter interface {
	Write(ctx context.Context, ev event.DomainEvent) error
}

// BrokerPublisher is the port for pushing raw outbox event payloads to a message broker.
// It is used exclusively by the OutboxWorker background process.
type BrokerPublisher interface {
	PublishBatch(ctx context.Context, records []*OutboxRecord) error
}
