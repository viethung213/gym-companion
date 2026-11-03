package port

import "context"

// BrokerPublisher sends serialized outbox events to Kafka.
type BrokerPublisher interface {
	PublishBatch(ctx context.Context, events []*OutboxRecord) error
}
