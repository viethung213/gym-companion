// Package event provides CloudEvent serialization and outbox publishing adapters
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
	case domainEvent.UserRoleUpdatedEvent:
		return p.publishUserRoleUpdated(ctx, e)
	case domainEvent.OTPSentEvent:
		return p.publishOTPSent(ctx, e)
	default:
		return fmt.Errorf("unsupported domain event: %T", ev)
	}
}

func (p *OutboxWriter) publishUserRegistered(ctx context.Context, ev domainEvent.UserRegisteredEvent) error {
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

	eventID := uuid.New().String()
	eventType := ev.EventName()

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

func (p *OutboxWriter) publishOTPSent(ctx context.Context, ev domainEvent.OTPSentEvent) error {
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

	eventID := uuid.New().String()
	eventType := ev.EventName()

	cloudEvent := map[string]interface{}{
		"specversion":     "1.0",
		"id":              eventID,
		"source":          "services/auth-service",
		"type":            eventType,
		"time":            ev.SentAt.Format(time.RFC3339),
		"datacontenttype": "application/json",
		"data":            json.RawMessage(payloadBytes),
	}

	envelopeBytes, err := json.Marshal(cloudEvent)
	if err != nil {
		return fmt.Errorf("marshal cloudevent envelope: %w", err)
	}

	return p.outboxRepo.SaveEvent(ctx, eventID, eventType, envelopeBytes, ev.Identifier)
}

func (p *OutboxWriter) publishUserRoleUpdated(ctx context.Context, ev domainEvent.UserRoleUpdatedEvent) error {
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

	eventID := uuid.New().String()
	eventType := ev.EventName()

	cloudEvent := map[string]interface{}{
		"specversion":     "1.0",
		"id":              eventID,
		"source":          "services/auth-service",
		"type":            eventType,
		"time":            ev.UpdatedAt.Format(time.RFC3339),
		"datacontenttype": "application/json",
		"data":            json.RawMessage(payloadBytes),
	}

	envelopeBytes, err := json.Marshal(cloudEvent)
	if err != nil {
		return fmt.Errorf("marshal cloudevent envelope: %w", err)
	}

	return p.outboxRepo.SaveEvent(ctx, eventID, eventType, envelopeBytes, ev.UserID)
}
