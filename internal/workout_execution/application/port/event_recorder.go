package port

import "context"

// EventRecorder is the port for persisting domain events into the transactional outbox table.
// It is used by Application Layer command handlers.
type EventRecorder interface {
	RecordEvents(ctx context.Context, events []any) error
}
