package port

import "context"

type EventRecorder interface {
	RecordEvents(ctx context.Context, events []any) error
}

type BrokerPublisher interface {
	PublishBatch(ctx context.Context, records []OutboxRecord) error
	Close() error
}
