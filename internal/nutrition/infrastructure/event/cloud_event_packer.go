package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	nutritionv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/nutrition/v1/event"
	domainEvent "github.com/viethung213/gym-companion/internal/nutrition/domain/event"
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

// PackCloudEvent packs a nutrition domain event into CloudEvents 1.0 JSON format.
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
		occurredAt    = time.Now().UTC()
		err           error
	)

	switch e := ev.(type) {
	case *domainEvent.NutritionPlanGeneratedEvent:
		payloadProto := &nutritionv1event.NutritionPlanGenerated{
			PlanId:    e.PlanID(),
			UserId:    e.UserID(),
			PlanDate:  e.PlanDate().Format("2006-01-02"),
			Timestamp: timestamppb.New(e.Timestamp()),
		}
		payloadBytes, err = protojson.Marshal(payloadProto)
		if err != nil {
			return nil, fmt.Errorf("marshal NutritionPlanGenerated proto: %w", err)
		}
		aggregateType = "NutritionPlan"
		aggregateID = e.PlanID()
		eventType = "contracts.core.nutrition.v1.event.NutritionPlanGenerated"
		partitionKey = e.UserID()
		occurredAt = e.Timestamp()

	case *domainEvent.NutritionPlanRecalibratedEvent:
		payloadProto := &nutritionv1event.NutritionPlanRecalibrated{
			PlanId:    e.PlanID(),
			UserId:    e.UserID(),
			Reason:    e.Reason(),
			Timestamp: timestamppb.New(e.Timestamp()),
		}
		payloadBytes, err = protojson.Marshal(payloadProto)
		if err != nil {
			return nil, fmt.Errorf("marshal NutritionPlanRecalibrated proto: %w", err)
		}
		aggregateType = "NutritionPlan"
		aggregateID = e.PlanID()
		eventType = "contracts.core.nutrition.v1.event.NutritionPlanRecalibrated"
		partitionKey = e.UserID()
		occurredAt = e.Timestamp()

	case *domainEvent.MealLoggedEvent:
		payloadProto := &nutritionv1event.MealLogged{
			UserId:    e.UserID(),
			MealLogId: e.MealLogID(),
			MealName:  e.MealName(),
			MealType:  e.MealType(),
			Calories:  float32(e.Calories()),
			LoggedAt:  timestamppb.New(e.LoggedAt()),
		}
		payloadBytes, err = protojson.Marshal(payloadProto)
		if err != nil {
			return nil, fmt.Errorf("marshal MealLogged proto: %w", err)
		}
		aggregateType = "MealLog"
		aggregateID = e.MealLogID()
		eventType = "contracts.core.nutrition.v1.event.MealLogged"
		partitionKey = e.UserID()
		occurredAt = e.LoggedAt()

	case *domainEvent.LockoutAppliedEvent:
		payloadProto := &nutritionv1event.LockoutApplied{
			UserId:     e.UserID(),
			ItemType:   e.ItemType(),
			ItemName:   e.ItemName(),
			UnlockedAt: timestamppb.New(e.UnlockedAt()),
		}
		payloadBytes, err = protojson.Marshal(payloadProto)
		if err != nil {
			return nil, fmt.Errorf("marshal LockoutApplied proto: %w", err)
		}
		aggregateType = "Lockout"
		aggregateID = e.UserID()
		eventType = "contracts.core.nutrition.v1.event.LockoutApplied"
		partitionKey = e.UserID()
		occurredAt = e.UnlockedAt()

	case *domainEvent.UpcomingMealReminderEvent:
		payloadProto := &nutritionv1event.UpcomingMealReminder{
			UserId:              e.UserID(),
			MealName:            e.MealName(),
			MealType:            e.MealType(),
			ScheduledTime:       e.ScheduledTime(),
			RemindBeforeMinutes: e.RemindBeforeMinutes(),
		}
		payloadBytes, err = protojson.Marshal(payloadProto)
		if err != nil {
			return nil, fmt.Errorf("marshal UpcomingMealReminder proto: %w", err)
		}
		aggregateType = "NutritionPlan"
		aggregateID = e.UserID()
		eventType = "contracts.core.nutrition.v1.event.UpcomingMealReminder"
		partitionKey = e.UserID()

	default:
		return nil, fmt.Errorf("unsupported nutrition domain event: %T", ev)
	}

	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	eventID := uuid.New().String()
	cloudEvent := CloudEventEnvelope{
		SpecVersion:     "1.0",
		ID:              eventID,
		Source:          "services/nutrition-service",
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
