package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	profilev1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/event"
	domainEvent "github.com/viethung213/gym-companion/internal/profile/domain/event"
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

// PackCloudEvent packs a profile domain event into CloudEvents 1.0 JSON format.
func PackCloudEvent(ev any) (*OutboxRecordData, error) {
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
	case *domainEvent.ProfileUpdatedEvent:
		bio := e.BiologicalMetrics()
		dobStr := ""
		if !bio.DateOfBirth().IsZero() {
			dobStr = bio.DateOfBirth().Format("2006-01-02")
		}

		payloadProto := &profilev1event.ProfileUpdated{
			UserId:         e.UserID(),
			WeightKg:       float32(bio.WeightKg()),
			HeightCm:       float32(bio.HeightCm()),
			DateOfBirth:    dobStr,
			Gender:         bio.Gender(),
			Goals:          e.Goals(),
			CompletionRate: float32(e.CompletionRate()),
			UpdatedAt:      timestamppb.New(e.UpdatedAt()),
		}

		payloadBytes, err = protojson.Marshal(payloadProto)
		if err != nil {
			return nil, fmt.Errorf("marshal ProfileUpdated proto: %w", err)
		}
		aggregateType = "UserProfile"
		aggregateID = e.UserID()
		eventType = "contracts.supporting.profile.v1.event.ProfileUpdated"
		partitionKey = e.UserID()
		occurredAt = e.UpdatedAt()

	case *domainEvent.ProfileCompletedEvent:
		bio := e.BiologicalMetrics()
		registeredInjuries := make([]string, 0, len(e.Injuries()))
		for _, inj := range e.Injuries() {
			registeredInjuries = append(registeredInjuries, inj.MuscleGroup())
		}

		payloadProto := &profilev1event.ProfileCompleted{
			UserId: e.UserID(),
			BiologicalMetrics: &profilev1event.BiologicalMetrics{
				WeightKg: float32(bio.WeightKg()),
				HeightCm: float32(bio.HeightCm()),
				Age:      bio.Age(),
				Gender:   bio.Gender(),
			},
			Goals:                 e.Goals(),
			RegisteredInjuries:    registeredInjuries,
			PreferredWorkoutTimes: e.PreferredWorkoutTimes(),
			CompletedAt:           timestamppb.New(e.CompletedAt()),
		}

		payloadBytes, err = protojson.Marshal(payloadProto)
		if err != nil {
			return nil, fmt.Errorf("marshal ProfileCompleted proto: %w", err)
		}
		aggregateType = "UserProfile"
		aggregateID = e.UserID()
		eventType = "contracts.supporting.profile.v1.event.ProfileCompleted"
		partitionKey = e.UserID()
		occurredAt = e.CompletedAt()

	case *domainEvent.InjuryReportedEvent:
		inj := e.Injury()
		payloadProto := &profilev1event.InjuryReported{
			UserId: e.UserID(),
			Injury: &profilev1event.InjuryDetails{
				InjuryId:    inj.ID(),
				MuscleGroup: inj.MuscleGroup(),
				Severity:    inj.Severity(),
				Notes:       inj.Notes(),
				ReportedAt:  timestamppb.New(inj.ReportedAt()),
			},
		}

		payloadBytes, err = protojson.Marshal(payloadProto)
		if err != nil {
			return nil, fmt.Errorf("marshal InjuryReported proto: %w", err)
		}
		aggregateType = "UserProfile"
		aggregateID = e.UserID()
		eventType = "contracts.supporting.profile.v1.event.InjuryReported"
		partitionKey = e.UserID()
		occurredAt = e.OccurredAt()

	case *domainEvent.InjuryRecoveredEvent:
		inj := e.Injury()
		payloadProto := &profilev1event.InjuryRecoveredData{
			UserId:      e.UserID(),
			InjuryId:    inj.ID(),
			MuscleGroup: inj.MuscleGroup(),
			RecoveredAt: timestamppb.New(e.RecoveredAt()),
		}

		payloadBytes, err = protojson.Marshal(payloadProto)
		if err != nil {
			return nil, fmt.Errorf("marshal InjuryRecovered proto: %w", err)
		}
		aggregateType = "UserProfile"
		aggregateID = e.UserID()
		eventType = "contracts.supporting.profile.v1.event.InjuryRecovered"
		partitionKey = e.UserID()
		occurredAt = e.RecoveredAt()

	case *domainEvent.UserIdentityUpdatedEvent:
		payloadProto := &profilev1event.UserIdentityUpdated{
			UserId:    e.UserID(),
			FullName:  e.FullName(),
			AvatarUrl: e.AvatarURL(),
			UpdatedAt: timestamppb.New(e.UpdatedAt()),
		}

		payloadBytes, err = protojson.Marshal(payloadProto)
		if err != nil {
			return nil, fmt.Errorf("marshal UserIdentityUpdated proto: %w", err)
		}
		aggregateType = "UserProfile"
		aggregateID = e.UserID()
		eventType = "contracts.supporting.profile.v1.event.UserIdentityUpdated"
		partitionKey = e.UserID()
		occurredAt = e.UpdatedAt()

	default:
		return nil, fmt.Errorf("unsupported profile domain event: %T", ev)
	}

	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	eventID := uuid.New().String()
	cloudEvent := CloudEventEnvelope{
		SpecVersion:     "1.0",
		ID:              eventID,
		Source:          "services/profile-service",
		Type:            eventType,
		Time:            occurredAt.Format(time.RFC3339),
		DataContentType: "application/json",
		Data:            json.RawMessage(payloadBytes),
	}

	cloudEventBytes, err := json.Marshal(cloudEvent)
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
		Payload:       cloudEventBytes,
	}, nil
}
