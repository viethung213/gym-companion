package port

import "context"

type EventPublisher interface {
	PublishEvents(ctx context.Context, events []any) error
}

type OutboxWriter = EventPublisher

type BrokerPublisher interface {
	PublishBatch(ctx context.Context, records []*OutboxRecord) error
}
