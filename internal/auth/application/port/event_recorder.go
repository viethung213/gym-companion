package port

import (
	"context"

	"github.com/viethung213/gym-companion/internal/auth/domain/event"
)

// EventRecorder is the port for serializing and persisting domain events
// into the transactional outbox table within the same database transaction.
// It is used exclusively by Application Layer command handlers.
type EventRecorder interface {
	Write(ctx context.Context, ev event.DomainEvent) error
}
