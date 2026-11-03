package event

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	profilev1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/event"
	"github.com/viethung213/gym-companion/internal/profile/application/port"
	domainEvent "github.com/viethung213/gym-companion/internal/profile/domain/event"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type OutboxWriter struct {
	outboxRepo port.OutboxRepository
}

var _ port.EventRecorder = (*OutboxWriter)(nil)

func NewOutboxWriter(outboxRepo port.OutboxRepository) *OutboxWriter {
	return &OutboxWriter{outboxRepo: outboxRepo}
}

// RecordEvents stores domain events as CloudEvent outbox records in the database.
func (w *OutboxWriter) RecordEvents(ctx context.Context, events []any) error {
	for _, ev := range events {
		if err := w.recordSingleEvent(ctx, ev); err != nil {
			return err
		}
	}
	return nil
}

// PublishEvents delegates to RecordEvents for backward compatibility.
func (w *OutboxWriter) PublishEvents(ctx context.Context, events []any) error {
	return w.RecordEvents(ctx, events)
}

func (w *OutboxWriter) recordSingleEvent(ctx context.Context, ev any) error {
	switch e := ev.(type) {
	case *domainEvent.ProfileCompletedEvent:
		return w.recordProfileCompleted(ctx, e)
	case *domainEvent.ProfileUpdatedEvent:
		return w.recordProfileUpdated(ctx, e)
	case *domainEvent.InjuryReportedEvent:
		return w.recordInjuryReported(ctx, e)
	case *domainEvent.InjuryRecoveredEvent:
		return w.recordInjuryRecovered(ctx, e)
	case *domainEvent.UserIdentityUpdatedEvent:
		return w.recordUserIdentityUpdated(ctx, e)
	default:
		return fmt.Errorf("unsupported profile domain event: %T", ev)
	}
}

func (w *OutboxWriter) recordProfileUpdated(ctx context.Context, ev *domainEvent.ProfileUpdatedEvent) error {
	bio := ev.BiologicalMetrics()
	dobStr := ""
	if !bio.DateOfBirth().IsZero() {
		dobStr = bio.DateOfBirth().Format("2006-01-02")
	}

	payloadProto := &profilev1event.ProfileUpdated{
		UserId:         ev.UserID(),
		WeightKg:       float32(bio.WeightKg()),
		HeightCm:       float32(bio.HeightCm()),
		DateOfBirth:    dobStr,
		Gender:         bio.Gender(),
		Goals:          ev.Goals(),
		CompletionRate: float32(ev.CompletionRate()),
		UpdatedAt:      timestamppb.New(ev.UpdatedAt()),
	}

	payloadBytes, err := protojson.Marshal(payloadProto)
	if err != nil {
		return fmt.Errorf("marshal ProfileUpdated payload proto: %w", err)
	}

	return w.writeCloudEvent(ctx, "UserProfile", ev.UserID(), "contracts.supporting.profile.v1.event.ProfileUpdated", ev.UserID(), ev.UpdatedAt(), payloadBytes)
}

func (w *OutboxWriter) recordProfileCompleted(ctx context.Context, ev *domainEvent.ProfileCompletedEvent) error {
	bio := ev.BiologicalMetrics()
	registeredInjuries := make([]string, 0, len(ev.Injuries()))
	for _, inj := range ev.Injuries() {
		registeredInjuries = append(registeredInjuries, inj.MuscleGroup())
	}

	payloadProto := &profilev1event.ProfileCompleted{
		UserId: ev.UserID(),
		BiologicalMetrics: &profilev1event.BiologicalMetrics{
			WeightKg: float32(bio.WeightKg()),
			HeightCm: float32(bio.HeightCm()),
			Age:      bio.Age(),
			Gender:   bio.Gender(),
		},
		Goals:                 ev.Goals(),
		RegisteredInjuries:    registeredInjuries,
		PreferredWorkoutTimes: ev.PreferredWorkoutTimes(),
		CompletedAt:           timestamppb.New(ev.CompletedAt()),
	}

	payloadBytes, err := protojson.Marshal(payloadProto)
	if err != nil {
		return fmt.Errorf("marshal ProfileCompleted payload proto: %w", err)
	}

	eventType := "contracts.supporting.profile.v1.event.ProfileCompleted"
	return w.writeCloudEvent(ctx, "UserProfile", ev.UserID(), eventType, ev.UserID(), ev.CompletedAt(), payloadBytes)
}

func (w *OutboxWriter) recordInjuryReported(ctx context.Context, ev *domainEvent.InjuryReportedEvent) error {
	inj := ev.Injury()
	payloadProto := &profilev1event.InjuryReported{
		UserId: ev.UserID(),
		Injury: &profilev1event.InjuryDetails{
			InjuryId:    inj.ID(),
			MuscleGroup: inj.MuscleGroup(),
			Severity:    inj.Severity(),
			Notes:       inj.Notes(),
			ReportedAt:  timestamppb.New(inj.ReportedAt()),
		},
	}

	payloadBytes, err := protojson.Marshal(payloadProto)
	if err != nil {
		return fmt.Errorf("marshal InjuryReported payload proto: %w", err)
	}

	return w.writeCloudEvent(ctx, "UserProfile", ev.UserID(), "contracts.supporting.profile.v1.event.InjuryReported", ev.UserID(), ev.OccurredAt(), payloadBytes)
}

func (w *OutboxWriter) recordInjuryRecovered(ctx context.Context, ev *domainEvent.InjuryRecoveredEvent) error {
	inj := ev.Injury()
	payloadProto := &profilev1event.InjuryRecoveredData{
		UserId:      ev.UserID(),
		InjuryId:    inj.ID(),
		MuscleGroup: inj.MuscleGroup(),
		RecoveredAt: timestamppb.New(ev.RecoveredAt()),
	}

	payloadBytes, err := protojson.Marshal(payloadProto)
	if err != nil {
		return fmt.Errorf("marshal InjuryRecovered payload proto: %w", err)
	}

	return w.writeCloudEvent(ctx, "UserProfile", ev.UserID(), "contracts.supporting.profile.v1.event.InjuryRecovered", ev.UserID(), ev.RecoveredAt(), payloadBytes)
}

func (w *OutboxWriter) recordUserIdentityUpdated(ctx context.Context, ev *domainEvent.UserIdentityUpdatedEvent) error {
	payloadProto := &profilev1event.UserIdentityUpdated{
		UserId:    ev.UserID(),
		FullName:  ev.FullName(),
		AvatarUrl: ev.AvatarURL(),
		UpdatedAt: timestamppb.New(ev.UpdatedAt()),
	}

	payloadBytes, err := protojson.Marshal(payloadProto)
	if err != nil {
		return fmt.Errorf("marshal UserIdentityUpdated payload proto: %w", err)
	}

	eventType := "contracts.supporting.profile.v1.event.UserIdentityUpdated"
	return w.writeCloudEvent(ctx, "UserProfile", ev.UserID(), eventType, ev.UserID(), ev.UpdatedAt(), payloadBytes)
}

func (w *OutboxWriter) writeCloudEvent(
	ctx context.Context,
	aggregateType, aggregateID, eventType, partitionKey string,
	occurredAt time.Time,
	payloadBytes []byte,
) error {
	eventID := uuid.New().String()
	now := occurredAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	cloudEvent := map[string]interface{}{
		"specversion":     "1.0",
		"id":              eventID,
		"source":          "services/profile-service",
		"type":            eventType,
		"time":            now.Format(time.RFC3339),
		"datacontenttype": "application/json",
		"data":            json.RawMessage(payloadBytes),
	}

	cloudEventBytes, err := json.Marshal(cloudEvent)
	if err != nil {
		return fmt.Errorf("marshal cloudevent envelope: %w", err)
	}

	record := &port.OutboxRecord{
		ID:            uuid.New().String(),
		EventID:       eventID,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       cloudEventBytes,
		PartitionKey:  partitionKey,
		Published:     false,
		CreatedAt:     now,
	}

	return w.outboxRepo.Save(ctx, record)
}
