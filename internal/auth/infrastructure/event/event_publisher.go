// Package event provides CloudEvent serialization and outbox publishing adapters
// for all domain events produced by the auth bounded context.
package event

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
	domainEvent "github.com/viethung213/gym-companion/internal/auth/domain/event"
	authv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/event"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// OutboxWriter implements port.EventPublisher by serializing domain events
// into CloudEvent envelopes and persisting them to the outbox table.
type OutboxWriter struct {
	outboxRepo port.OutboxRepository
}

// Compile-time interface check
var _ port.OutboxWriter = (*OutboxWriter)(nil)

// NewOutboxWriter creates a new OutboxWriter instance.
func NewOutboxWriter(outboxRepo port.OutboxRepository) *OutboxWriter {
	return &OutboxWriter{outboxRepo: outboxRepo}
}

// Write serializes and stores the domain event as a CloudEvent into the outbox repository.
func (p *OutboxWriter) Write(ctx context.Context, ev domainEvent.DomainEvent) error {
	switch e := ev.(type) {
	case domainEvent.UserRegisteredEvent:
		return p.publishUserRegistered(ctx, e)
	default:
		return fmt.Errorf("unsupported domain event: %T", ev)
	}
}

func (p *OutboxWriter) publishUserRegistered(ctx context.Context, ev domainEvent.UserRegisteredEvent) error {
	userRegisteredProto := &authv1event.UserRegistered{
		UserId:       ev.UserID,
		Email:        ev.Email,
		FullName:     ev.FullName,
		RegisteredAt: timestamppb.New(ev.RegisteredAt),
	}

	payloadBytes, err := protojson.Marshal(userRegisteredProto)
	if err != nil {
		return fmt.Errorf("marshal user registered proto: %w", err)
	}

	eventID := uuid.New().String()
	eventType := "contracts.generic.auth.v1.userRegistered"

	cloudEvent := map[string]interface{}{
		"specversion":     "1.0",
		"id":              eventID,
		"source":          "services/auth-service",
		"type":            eventType,
		"time":            ev.RegisteredAt.Format(time.RFC3339),
		"datacontenttype": "application/json",
		"data":            json.RawMessage(payloadBytes),
	}

	envelopeBytes, err := json.Marshal(cloudEvent)
	if err != nil {
		return fmt.Errorf("marshal cloudevent envelope: %w", err)
	}

	return p.outboxRepo.SaveEvent(ctx, eventID, eventType, envelopeBytes, ev.UserID)
}
