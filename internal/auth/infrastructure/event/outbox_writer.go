// Package event provides CloudEvent serialization and outbox recording adapters
// for all domain events produced by the auth bounded context.
package event

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
	domainEvent "github.com/viethung213/gym-companion/internal/auth/domain/event"
	authv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/event"
	notificationv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/notification/v1/event"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// OutboxWriter implements port.EventRecorder by serializing domain events
// into CloudEvent envelopes and persisting them to the outbox table.
type OutboxWriter struct {
	outboxRepo port.OutboxRepository
}

// Compile-time interface check
var _ port.EventRecorder = (*OutboxWriter)(nil)

// NewOutboxWriter creates a new OutboxWriter instance.
func NewOutboxWriter(outboxRepo port.OutboxRepository) *OutboxWriter {
	return &OutboxWriter{outboxRepo: outboxRepo}
}

// RecordEvents records a slice of domain events into the outbox repository.
func (p *OutboxWriter) RecordEvents(ctx context.Context, events []any) error {
	for _, ev := range events {
		dev, ok := ev.(domainEvent.DomainEvent)
		if !ok {
			return fmt.Errorf("unsupported domain event: %T", ev)
		}
		if err := p.Write(ctx, dev); err != nil {
			return err
		}
	}
	return nil
}

// Write serializes and stores the domain event as a CloudEvent into the outbox repository.
func (p *OutboxWriter) Write(ctx context.Context, ev domainEvent.DomainEvent) error {
	switch e := ev.(type) {
	case domainEvent.UserRegisteredEvent:
		return p.recordUserRegistered(ctx, e)
	case domainEvent.UserRoleUpdatedEvent:
		return p.recordUserRoleUpdated(ctx, e)
	case domainEvent.OTPSentEvent:
		return p.recordOTPSent(ctx, e)
	default:
		return fmt.Errorf("unsupported domain event: %T", ev)
	}
}

func (p *OutboxWriter) recordUserRegistered(ctx context.Context, ev domainEvent.UserRegisteredEvent) error {
	userRegisteredProto := &authv1event.UserRegistered{
		UserId:       ev.UserID,
		IdentityType: ev.IdentityType,
		Identifier:   ev.Identifier,
		FullName:     ev.FullName,
		Gender:       ev.Gender,
		DateOfBirth:  ev.DateOfBirth,
		AvatarUrl:    ev.AvatarURL,
		Role:         ev.Role,
		RegisteredAt: timestamppb.New(ev.RegisteredAt),
	}

	payloadBytes, err := protojson.Marshal(userRegisteredProto)
	if err != nil {
		return fmt.Errorf("marshal user registered proto: %w", err)
	}

	return p.writeCloudEvent(ctx, "User", ev.UserID, ev.EventName(), ev.UserID, ev.RegisteredAt, payloadBytes)
}

func (p *OutboxWriter) recordOTPSent(ctx context.Context, ev domainEvent.OTPSentEvent) error {
	channel := "SMS"
	dataMap := map[string]string{
		"code":             ev.Code,
		"otp":              ev.Code,
		"identifier":       ev.Identifier,
		"expiresInSeconds": fmt.Sprintf("%d", ev.ExpiresInSeconds),
	}
	if strings.Contains(ev.Identifier, "@") {
		channel = "EMAIL"
		dataMap["email"] = ev.Identifier
	} else {
		dataMap["phone"] = ev.Identifier
	}

	highPriorityProto := &notificationv1event.HighPriorityNotificationRequested{
		Target: &notificationv1event.NotificationTarget{
			Recipient: &notificationv1event.NotificationTarget_UserId{
				UserId: ev.Identifier,
			},
		},
		Title:       "Mã xác thực OTP Gym Companion",
		Body:        fmt.Sprintf("Mã OTP của bạn là: %s. Mã có hiệu lực trong %d giây. Vui lòng không chia sẻ mã này cho bất kỳ ai.", ev.Code, ev.ExpiresInSeconds),
		Data:        dataMap,
		Channels:    []string{channel},
		RequestedAt: timestamppb.New(ev.SentAt),
	}

	payloadBytes, err := protojson.Marshal(highPriorityProto)
	if err != nil {
		return fmt.Errorf("marshal high priority notification proto: %w", err)
	}

	sentAt := ev.SentAt
	if sentAt.IsZero() {
		sentAt = time.Now().UTC()
	}

	return p.writeCloudEvent(ctx, "OTP", ev.Identifier, ev.EventName(), ev.Identifier, sentAt, payloadBytes)
}

func (p *OutboxWriter) recordUserRoleUpdated(ctx context.Context, ev domainEvent.UserRoleUpdatedEvent) error {
	roleUpdatedProto := &authv1event.UserRoleUpdated{
		UserId:    ev.UserID,
		OldRole:   ev.OldRole,
		NewRole:   ev.NewRole,
		UpdatedAt: timestamppb.New(ev.UpdatedAt),
	}

	payloadBytes, err := protojson.Marshal(roleUpdatedProto)
	if err != nil {
		return fmt.Errorf("marshal user role updated proto: %w", err)
	}

	return p.writeCloudEvent(ctx, "User", ev.UserID, ev.EventName(), ev.UserID, ev.UpdatedAt, payloadBytes)
}

func (p *OutboxWriter) writeCloudEvent(ctx context.Context, aggregateType, aggregateID, eventType, partitionKey string, occurredAt time.Time, payloadBytes []byte) error {
	eventID := uuid.New().String()

	cloudEvent := map[string]interface{}{
		"specversion":     "1.0",
		"id":              eventID,
		"source":          "services/auth-service",
		"type":            eventType,
		"time":            occurredAt.Format(time.RFC3339),
		"datacontenttype": "application/json",
		"data":            json.RawMessage(payloadBytes),
	}

	envelopeBytes, err := json.Marshal(cloudEvent)
	if err != nil {
		return fmt.Errorf("marshal cloudevent envelope: %w", err)
	}

	record := &port.OutboxRecord{
		ID:            uuid.New().String(),
		EventID:       eventID,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       envelopeBytes,
		PartitionKey:  partitionKey,
		Published:     false,
		CreatedAt:     occurredAt,
	}
	return p.outboxRepo.Save(ctx, record)
}
