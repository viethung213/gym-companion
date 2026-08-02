package event

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"
	workoutexecutionv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/event"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	domainEvent "github.com/viethung213/gym-companion/internal/workout_execution/domain/event"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// NameProvider interface to extract event name string.

type NameProvider interface {
	EventName() string
}

// OutboxWriter implements port.OutboxWriter.

type OutboxWriter struct {
	outboxRepo port.OutboxRepository
}

var _ port.OutboxWriter = (*OutboxWriter)(nil)

// NewOutboxWriter constructs OutboxWriter.

func NewOutboxWriter(outboxRepo port.OutboxRepository) *OutboxWriter {

	return &OutboxWriter{outboxRepo: outboxRepo}

}

// WriteEvents serializes domain events to CloudEvents and writes to outbox.

func (w *OutboxWriter) WriteEvents(ctx context.Context, aggregateType, aggregateID string, events []interface{}) error {

	for _, ev := range events {

		if err := w.writeSingleEvent(ctx, aggregateType, aggregateID, ev); err != nil {

			return err

		}

	}

	return nil

}

func (w *OutboxWriter) writeSingleEvent(ctx context.Context, aggregateType, aggregateID string, ev interface{}) error {

	eventType := "contracts.core.workout_execution.v1.unknownEvent"

	if nameProvider, ok := ev.(NameProvider); ok {

		eventType = nameProvider.EventName()

	} else if ev != nil && reflect.TypeOf(ev).Kind() == reflect.Struct {

		vp := reflect.New(reflect.TypeOf(ev))

		vp.Elem().Set(reflect.ValueOf(ev))

		if nameProvider, ok := vp.Interface().(NameProvider); ok {

			eventType = nameProvider.EventName()

		}

	}

	var payloadBytes []byte

	var err error

	if protoMsg := mapToProtoEvent(ev); protoMsg != nil {

		payloadBytes, err = protojson.Marshal(protoMsg)

		if err != nil {

			return fmt.Errorf("failed to marshal proto event payload: %w", err)

		}

	} else {

		payloadBytes, err = json.Marshal(ev)

		if err != nil {

			return fmt.Errorf("failed to marshal event payload: %w", err)

		}

	}

	outboxID := uuid.NewString()

	eventID := uuid.NewString()

	now := time.Now().UTC().Format(time.RFC3339)

	cloudEvent := map[string]interface{}{

		"specversion": "1.0",

		"id": eventID,

		"source": "services/workout-execution-service",

		"type": eventType,

		"time": now,

		"datacontenttype": "application/json",

		"data": json.RawMessage(payloadBytes),
	}

	envelopeBytes, err := json.Marshal(cloudEvent)

	if err != nil {

		return fmt.Errorf("failed to marshal cloud event envelope: %w", err)

	}

	record := &port.OutboxRecord{

		ID: outboxID,

		EventID: eventID,

		AggregateType: aggregateType,

		AggregateID: aggregateID,

		EventType: eventType,

		Payload: envelopeBytes,

		PartitionKey: aggregateID,

		Published: false,

		CreatedAt: time.Now().UTC(),
	}

	if err := w.outboxRepo.Save(ctx, record); err != nil {

		return fmt.Errorf("failed to save outbox record: %w", err)

	}

	return nil

}

func mapToProtoEvent(ev interface{}) proto.Message {

	switch e := ev.(type) {

	case domainEvent.WorkoutSessionStarted:

		return &workoutexecutionv1event.WorkoutSessionStarted{

			SessionId: e.SessionID,

			UserId: e.UserID,

			PlanId: e.PlanID,

			StartedAt: timestamppb.New(e.StartedAt),
		}

	case *domainEvent.WorkoutSessionStarted:

		if e == nil {

			return nil

		}

		return &workoutexecutionv1event.WorkoutSessionStarted{

			SessionId: e.SessionID,

			UserId: e.UserID,

			PlanId: e.PlanID,

			StartedAt: timestamppb.New(e.StartedAt),
		}

	case domainEvent.WorkoutSessionCompleted:

		return mapWorkoutSessionCompletedToProto(&e)

	case *domainEvent.WorkoutSessionCompleted:

		if e == nil {

			return nil

		}

		return mapWorkoutSessionCompletedToProto(e)

	case domainEvent.WorkoutSessionAborted:

		return &workoutexecutionv1event.WorkoutSessionAborted{

			SessionId: e.SessionID,

			UserId: e.UserID,

			AbortedAt: timestamppb.New(e.AbortedAt),

			Reason: e.Reason,
		}

	case *domainEvent.WorkoutSessionAborted:

		if e == nil {

			return nil

		}

		return &workoutexecutionv1event.WorkoutSessionAborted{

			SessionId: e.SessionID,

			UserId: e.UserID,

			AbortedAt: timestamppb.New(e.AbortedAt),

			Reason: e.Reason,
		}

	case domainEvent.NewPersonalRecordAchieved:

		return &workoutexecutionv1event.NewPersonalRecordAchieved{

			UserId: e.UserID,

			ExerciseId: e.ExerciseID,

			OneRepMax: e.OneRepMax,

			Weight: e.Weight,

			Reps: int32(e.Reps),

			AchievedAt: timestamppb.New(e.AchievedAt),
		}

	case *domainEvent.NewPersonalRecordAchieved:

		if e == nil {

			return nil

		}

		return &workoutexecutionv1event.NewPersonalRecordAchieved{

			UserId: e.UserID,

			ExerciseId: e.ExerciseID,

			OneRepMax: e.OneRepMax,

			Weight: e.Weight,

			Reps: int32(e.Reps),

			AchievedAt: timestamppb.New(e.AchievedAt),
		}

	default:

		return nil

	}

}

func mapWorkoutSessionCompletedToProto(e *domainEvent.WorkoutSessionCompleted) *workoutexecutionv1event.WorkoutSessionCompleted {

	var avgScore *float32

	if e.Summary.AverageFormScore != nil {

		score := float32(*e.Summary.AverageFormScore)

		avgScore = &score

	}

	return &workoutexecutionv1event.WorkoutSessionCompleted{

		SessionId: e.SessionID,

		UserId: e.UserID,

		CompletedAt: timestamppb.New(e.CompletedAt),

		TotalSets: int32(e.Summary.TotalSets),

		TotalVolume: float32(e.Summary.TotalVolume),

		AverageFormScore: avgScore,

		AverageRpe: float32(e.Summary.AverageRPE),
	}

}
