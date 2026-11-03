package port

import "context"

// EventRecorder records domain events into the persistent outbox table in the database.
type EventRecorder interface {
	RecordEvents(ctx context.Context, events []any) error
}

// OutboxWriter is an alias for EventRecorder.
type OutboxWriter = EventRecorder
