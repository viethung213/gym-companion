package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	domainEvent "github.com/viethung213/gym-companion/internal/auth/domain/event"
	authv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/event"
	notificationv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/notification/v1/event"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CloudEventEnvelope represents the standard CloudEvents 1.0 JSON format envelope.
type CloudEventEnvelope struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Source          string          `json:"source"`
	Type            string          `json:"type"`
	Time            string          `json:"time"`
	DataContentType string          `json:"datacontenttype"`
	Data            json.RawMessage `json:"data"`
}

// OutboxRecordData holds the packaged event metadata and payload ready for Outbox persistence.
type OutboxRecordData struct {
	EventID       string
	EventType     string
	AggregateType string
	AggregateID   string
	PartitionKey  string
	OccurredAt    time.Time
	Payload       []byte
}

// PackCloudEvent packs an auth domain event into CloudEvents 1.0 JSON format.
func PackCloudEvent(ev domainEvent.DomainEvent) (*OutboxRecordData, error) {
	if ev == nil {
		return nil, errors.New("nil domain event")
	}

	var (
		payloadBytes  []byte
		aggregateType string
		aggregateID   string
		eventType     string
		partitionKey  string
		occurredAt    time.Time
		err           error
	)

	switch e := ev.(type) {
	case domainEvent.UserRegisteredEvent:
		userRegisteredProto := &authv1event.UserRegistered{
			UserId:       e.UserID,
			IdentityType: e.IdentityType,
			Identifier:   e.Identifier,
			FullName:     e.FullName,
			Gender:       e.Gender,
			DateOfBirth:  e.DateOfBirth,
			AvatarUrl:    e.AvatarURL,
			Role:         e.Role,
			RegisteredAt: timestamppb.New(e.RegisteredAt),
		}

		payloadBytes, err = protojson.Marshal(userRegisteredProto)
		if err != nil {
			return nil, fmt.Errorf("marshal user registered proto: %w", err)
		}
		aggregateType = "User"
		aggregateID = e.UserID
		eventType = e.EventName()
		partitionKey = e.UserID
		occurredAt = e.RegisteredAt

	case domainEvent.OTPSentEvent:
		channel := "SMS"
		dataMap := map[string]string{
			"code":             e.Code,
			"otp":              e.Code,
			"identifier":       e.Identifier,
			"expiresInSeconds": fmt.Sprintf("%d", e.ExpiresInSeconds),
		}
		if strings.Contains(e.Identifier, "@") {
			channel = "EMAIL"
			dataMap["email"] = e.Identifier
		} else {
			dataMap["phone"] = e.Identifier
		}

		highPriorityProto := &notificationv1event.HighPriorityNotificationRequested{
			Target: &notificationv1event.NotificationTarget{
				Recipient: &notificationv1event.NotificationTarget_UserId{
					UserId: e.Identifier,
				},
			},
			Title: "Mã xác thực OTP Gym Companion",
			Body: fmt.Sprintf(
				"Mã OTP của bạn là: %s. Mã có hiệu lực trong %d giây. Vui lòng không chia sẻ mã này cho bất kỳ ai.",
				e.Code, e.ExpiresInSeconds,
			),
			Data:        dataMap,
			Channels:    []string{channel},
			RequestedAt: timestamppb.New(e.SentAt),
		}

		payloadBytes, err = protojson.Marshal(highPriorityProto)
		if err != nil {
			return nil, fmt.Errorf("marshal high priority notification proto: %w", err)
		}
		occurredAt = e.SentAt
		if occurredAt.IsZero() {
			occurredAt = time.Now().UTC()
		}
		aggregateType = "OTP"
		aggregateID = e.Identifier
		eventType = e.EventName()
		partitionKey = e.Identifier

	case domainEvent.UserRoleUpdatedEvent:
		roleUpdatedProto := &authv1event.UserRoleUpdated{
			UserId:    e.UserID,
			OldRole:   e.OldRole,
			NewRole:   e.NewRole,
			UpdatedAt: timestamppb.New(e.UpdatedAt),
		}

		payloadBytes, err = protojson.Marshal(roleUpdatedProto)
		if err != nil {
			return nil, fmt.Errorf("marshal user role updated proto: %w", err)
		}
		aggregateType = "User"
		aggregateID = e.UserID
		eventType = e.EventName()
		partitionKey = e.UserID
		occurredAt = e.UpdatedAt

	default:
		return nil, fmt.Errorf("unsupported domain event: %T", ev)
	}

	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	eventID := uuid.New().String()
	cloudEvent := CloudEventEnvelope{
		SpecVersion:     "1.0",
		ID:              eventID,
		Source:          "services/auth-service",
		Type:            eventType,
		Time:            occurredAt.Format(time.RFC3339),
		DataContentType: "application/json",
		Data:            json.RawMessage(payloadBytes),
	}

	envelopeBytes, err := json.Marshal(cloudEvent)
	if err != nil {
		return nil, fmt.Errorf("marshal cloudevent envelope: %w", err)
	}

	return &OutboxRecordData{
		EventID:       eventID,
		EventType:     eventType,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		PartitionKey:  partitionKey,
		OccurredAt:    occurredAt,
		Payload:       envelopeBytes,
	}, nil
}
