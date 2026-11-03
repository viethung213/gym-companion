package port

import "context"

type BrokerPublisher interface {
	PublishBatch(ctx context.Context, records []OutboxRecord) error
	Close() error
}
