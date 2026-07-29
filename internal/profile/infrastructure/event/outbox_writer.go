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
)

type OutboxWriter struct {
	outboxRepo port.OutboxRepository
}

var _ port.EventPublisher = (*OutboxWriter)(nil)

func NewOutboxWriter(outboxRepo port.OutboxRepository) *OutboxWriter {
	return &OutboxWriter{outboxRepo: outboxRepo}
}

func (w *OutboxWriter) PublishEvents(ctx context.Context, events []any) error {
	for _, ev := range events {
		if err := w.publishSingleEvent(ctx, ev); err != nil {
			return err
		}
	}
	return nil
}

func (w *OutboxWriter) publishSingleEvent(ctx context.Context, ev any) error {
	switch e := ev.(type) {
	case *domainEvent.ProfileCompletedEvent:
		return w.publishProfileCompleted(ctx, e)
	case *domainEvent.ProfileUpdatedEvent:
		return w.publishProfileUpdated(ctx, e)
	case *domainEvent.InjuryReportedEvent:
		return w.publishInjuryReported(ctx, e)
	case *domainEvent.InjuryRecoveredEvent:
		return w.publishInjuryRecovered(ctx, e)
	default:
		return fmt.Errorf("unsupported profile domain event: %T", ev)
	}
}

func (w *OutboxWriter) publishProfileUpdated(ctx context.Context, ev *domainEvent.ProfileUpdatedEvent) error {
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
	}

	payloadBytes, err := protojson.Marshal(payloadProto)
	if err != nil {
		return fmt.Errorf("marshal ProfileUpdated payload proto: %w", err)
	}

	eventID := uuid.New().String()
	eventType := "contracts.supporting.profile.v1.event.ProfileUpdated"

	cloudEvent := map[string]interface{}{
		"specversion":     "1.0",
		"id":              eventID,
		"source":          "services/profile-service",
		"type":            eventType,
		"time":            ev.UpdatedAt().Format(time.RFC3339),
		"datacontenttype": "application/json",
		"data":            json.RawMessage(payloadBytes),
	}

	cloudEventBytes, err := json.Marshal(cloudEvent)
	if err != nil {
		return fmt.Errorf("marshal cloudevent envelope: %w", err)
	}

	record := &port.OutboxRecord{
		ID:           uuid.New().String(),
		EventID:      eventID,
		EventType:    eventType,
		Payload:      cloudEventBytes,
		PartitionKey: ev.UserID(),
	}

	return w.outboxRepo.Save(ctx, record)
}

func (w *OutboxWriter) publishProfileCompleted(ctx context.Context, ev *domainEvent.ProfileCompletedEvent) error {
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
		CompletedAt:           ev.CompletedAt().Format(time.RFC3339),
	}

	payloadBytes, err := protojson.Marshal(payloadProto)
	if err != nil {
		return fmt.Errorf("marshal ProfileCompleted payload proto: %w", err)
	}

	eventID := uuid.New().String()
	eventType := "contracts.supporting.profile.v1.event.ProfileCompleted"

	cloudEvent := map[string]interface{}{
		"specversion":     "1.0",
		"id":              eventID,
		"source":          "services/profile-service",
		"type":            eventType,
		"time":            ev.CompletedAt().Format(time.RFC3339),
		"datacontenttype": "application/json",
		"data":            json.RawMessage(payloadBytes),
	}

	cloudEventBytes, err := json.Marshal(cloudEvent)
	if err != nil {
		return fmt.Errorf("marshal cloudevent envelope: %w", err)
	}

	record := &port.OutboxRecord{
		ID:           uuid.New().String(),
		EventID:      eventID,
		EventType:    eventType,
		Payload:      cloudEventBytes,
		PartitionKey: ev.UserID(),
	}

	return w.outboxRepo.Save(ctx, record)
}

func (w *OutboxWriter) publishInjuryReported(ctx context.Context, ev *domainEvent.InjuryReportedEvent) error {
	inj := ev.Injury()
	payloadProto := &profilev1event.InjuryReported{
		UserId: ev.UserID(),
		Injury: &profilev1event.InjuryDetails{
			InjuryId:    inj.ID(),
			MuscleGroup: inj.MuscleGroup(),
			Severity:    inj.Severity(),
			Notes:       inj.Notes(),
			ReportedAt:  inj.ReportedAt().Format(time.RFC3339),
		},
	}

	payloadBytes, err := protojson.Marshal(payloadProto)
	if err != nil {
		return fmt.Errorf("marshal InjuryReported payload proto: %w", err)
	}

	eventID := uuid.New().String()
	eventType := "contracts.supporting.profile.v1.event.InjuryReported"

	cloudEvent := map[string]interface{}{
		"specversion":     "1.0",
		"id":              eventID,
		"source":          "services/profile-service",
		"type":            eventType,
		"time":            ev.OccurredAt().Format(time.RFC3339),
		"datacontenttype": "application/json",
		"data":            json.RawMessage(payloadBytes),
	}

	cloudEventBytes, err := json.Marshal(cloudEvent)
	if err != nil {
		return fmt.Errorf("marshal cloudevent envelope: %w", err)
	}

	record := &port.OutboxRecord{
		ID:           uuid.New().String(),
		EventID:      eventID,
		EventType:    eventType,
		Payload:      cloudEventBytes,
		PartitionKey: ev.UserID(),
	}

	return w.outboxRepo.Save(ctx, record)
}

func (w *OutboxWriter) publishInjuryRecovered(ctx context.Context, ev *domainEvent.InjuryRecoveredEvent) error {
	inj := ev.Injury()
	payloadProto := &profilev1event.InjuryRecoveredData{
		UserId:      ev.UserID(),
		InjuryId:    inj.ID(),
		MuscleGroup: inj.MuscleGroup(),
		RecoveredAt: ev.RecoveredAt().Format(time.RFC3339),
	}

	payloadBytes, err := protojson.Marshal(payloadProto)
	if err != nil {
		return fmt.Errorf("marshal InjuryRecovered payload proto: %w", err)
	}

	eventID := uuid.New().String()
	eventType := "contracts.supporting.profile.v1.event.InjuryRecovered"

	cloudEvent := map[string]interface{}{
		"specversion":     "1.0",
		"id":              eventID,
		"source":          "services/profile-service",
		"type":            eventType,
		"time":            ev.RecoveredAt().Format(time.RFC3339),
		"datacontenttype": "application/json",
		"data":            json.RawMessage(payloadBytes),
	}

	cloudEventBytes, err := json.Marshal(cloudEvent)
	if err != nil {
		return fmt.Errorf("marshal cloudevent envelope: %w", err)
	}

	record := &port.OutboxRecord{
		ID:           uuid.New().String(),
		EventID:      eventID,
		EventType:    eventType,
		Payload:      cloudEventBytes,
		PartitionKey: ev.UserID(),
	}

	return w.outboxRepo.Save(ctx, record)
}
