package port

import "context"

// BrokerPublisher is the port for pushing raw outbox event payloads to a message broker.
type BrokerPublisher interface {
	PublishBatch(ctx context.Context, records []*OutboxRecord) error
}
