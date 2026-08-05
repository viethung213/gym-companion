package port

import "context"

type EventPublisher interface {
	PublishEvents(ctx context.Context, events []any) error
}

type BrokerPublisher interface {
	PublishBatch(ctx context.Context, records []OutboxRecord) error
	Close() error
}
