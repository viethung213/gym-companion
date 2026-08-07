package event

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	nutritionv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/nutrition/v1/event"
	"github.com/viethung213/gym-companion/internal/nutrition/application/port"
	domainEvent "github.com/viethung213/gym-companion/internal/nutrition/domain/event"
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
	case *domainEvent.NutritionPlanGeneratedEvent:
		payloadProto := &nutritionv1event.NutritionPlanGenerated{
			PlanId:    e.PlanID(),
			UserId:    e.UserID(),
			PlanDate:  e.PlanDate().Format("2006-01-02"),
			Timestamp: e.Timestamp().Format(time.RFC3339),
		}
		payloadBytes, err := protojson.Marshal(payloadProto)
		if err != nil {
			return fmt.Errorf("marshal NutritionPlanGenerated proto payload: %w", err)
		}
		return w.publishCloudEvent(ctx, "contracts.core.nutrition.v1.event.NutritionPlanGenerated", e.UserID(), payloadBytes)

	case *domainEvent.NutritionPlanRecalibratedEvent:
		payloadProto := &nutritionv1event.NutritionPlanRecalibrated{
			PlanId:    e.PlanID(),
			UserId:    e.UserID(),
			Reason:    e.Reason(),
			Timestamp: e.Timestamp().Format(time.RFC3339),
		}
		payloadBytes, err := protojson.Marshal(payloadProto)
		if err != nil {
			return fmt.Errorf("marshal NutritionPlanRecalibrated proto payload: %w", err)
		}
		return w.publishCloudEvent(ctx, "contracts.core.nutrition.v1.event.NutritionPlanRecalibrated", e.UserID(), payloadBytes)

	case *domainEvent.MealLoggedEvent:
		payloadProto := &nutritionv1event.MealLogged{
			UserId:    e.UserID(),
			MealLogId: e.MealLogID(),
			MealName:  e.MealName(),
			MealType:  e.MealType(),
			Calories:  float32(e.Calories()),
			LoggedAt:  e.LoggedAt().Format(time.RFC3339),
		}
		payloadBytes, err := protojson.Marshal(payloadProto)
		if err != nil {
			return fmt.Errorf("marshal MealLogged proto payload: %w", err)
		}
		return w.publishCloudEvent(ctx, "contracts.core.nutrition.v1.event.MealLogged", e.UserID(), payloadBytes)

	case *domainEvent.LockoutAppliedEvent:
		payloadProto := &nutritionv1event.LockoutApplied{
			UserId:     e.UserID(),
			ItemType:   e.ItemType(),
			ItemName:   e.ItemName(),
			UnlockedAt: e.UnlockedAt().Format(time.RFC3339),
		}
		payloadBytes, err := protojson.Marshal(payloadProto)
		if err != nil {
			return fmt.Errorf("marshal LockoutApplied proto payload: %w", err)
		}
		return w.publishCloudEvent(ctx, "contracts.core.nutrition.v1.event.LockoutApplied", e.UserID(), payloadBytes)

	case *domainEvent.UpcomingMealReminderEvent:
		payloadProto := &nutritionv1event.UpcomingMealReminder{
			UserId:              e.UserID(),
			MealName:            e.MealName(),
			MealType:            e.MealType(),
			ScheduledTime:       e.ScheduledTime(),
			RemindBeforeMinutes: e.RemindBeforeMinutes(),
		}
		payloadBytes, err := protojson.Marshal(payloadProto)
		if err != nil {
			return fmt.Errorf("marshal UpcomingMealReminder proto payload: %w", err)
		}
		return w.publishCloudEvent(ctx, "contracts.core.nutrition.v1.event.UpcomingMealReminder", e.UserID(), payloadBytes)

	default:
		return fmt.Errorf("unsupported nutrition domain event: %T", ev)
	}
}

func (w *OutboxWriter) publishCloudEvent(ctx context.Context, eventType, partitionKey string, payloadBytes []byte) error {
	eventID := uuid.New().String()
	now := time.Now()

	cloudEvent := map[string]interface{}{
		"specversion":     "1.0",
		"id":              eventID,
		"source":          "services/nutrition-service",
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
		ID:           uuid.New().String(),
		EventID:      eventID,
		EventType:    eventType,
		Payload:      cloudEventBytes,
		PartitionKey: partitionKey,
		CreatedAt:    now,
	}

	return w.outboxRepo.Save(ctx, record)
}
