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
	"google.golang.org/protobuf/types/known/timestamppb"
)

// NameProvider interface to extract event name string.
type NameProvider interface {
	EventName() string
}

// OutboxWriter implements port.EventRecorder.
type OutboxWriter struct {
	outboxRepo port.OutboxRepository
}

var _ port.EventRecorder = (*OutboxWriter)(nil)

// NewOutboxWriter constructs OutboxWriter.
func NewOutboxWriter(outboxRepo port.OutboxRepository) *OutboxWriter {
	return &OutboxWriter{outboxRepo: outboxRepo}
}

// RecordEvents serializes domain events to CloudEvents and writes to outbox.
func (w *OutboxWriter) RecordEvents(ctx context.Context, events []any) error {
	for _, ev := range events {
		if err := w.recordSingleEvent(ctx, ev); err != nil {
			return err
		}
	}
	return nil
}

// WriteEvents delegates to RecordEvents for backward compatibility.
func (w *OutboxWriter) WriteEvents(ctx context.Context, aggregateType, aggregateID string, events []interface{}) error {
	anyEvents := make([]any, len(events))
	for i, ev := range events {
		anyEvents[i] = ev
	}
	return w.RecordEvents(ctx, anyEvents)
}

func (w *OutboxWriter) recordSingleEvent(ctx context.Context, ev any) error {
	if ev == nil {
		return nil
	}

	var (
		aggregateType string
		aggregateID   string
		partitionKey  string
		eventType     string
		occurredAt    time.Time
		payloadBytes  []byte
		err           error
	)

	switch e := ev.(type) {
	case *domainEvent.WorkoutSessionStarted:
		aggregateType, aggregateID, partitionKey, eventType = "WorkoutSession", e.SessionID, e.UserID, e.EventName()
		occurredAt = e.StartedAt
		payloadBytes, err = protojson.Marshal(&workoutexecutionv1event.WorkoutSessionStarted{
			SessionId: e.SessionID,
			UserId:    e.UserID,
			PlanId:    e.PlanID,
			StartedAt: timestamppb.New(e.StartedAt),
		})
	case domainEvent.WorkoutSessionStarted:
		aggregateType, aggregateID, partitionKey, eventType = "WorkoutSession", e.SessionID, e.UserID, e.EventName()
		occurredAt = e.StartedAt
		payloadBytes, err = protojson.Marshal(&workoutexecutionv1event.WorkoutSessionStarted{
			SessionId: e.SessionID,
			UserId:    e.UserID,
			PlanId:    e.PlanID,
			StartedAt: timestamppb.New(e.StartedAt),
		})

	case *domainEvent.WorkoutSessionCompleted:
		aggregateType, aggregateID, partitionKey, eventType = "WorkoutSession", e.SessionID, e.UserID, e.EventName()
		occurredAt = e.CompletedAt
		payloadBytes, err = protojson.Marshal(mapWorkoutSessionCompletedToProto(e))
	case domainEvent.WorkoutSessionCompleted:
		aggregateType, aggregateID, partitionKey, eventType = "WorkoutSession", e.SessionID, e.UserID, e.EventName()
		occurredAt = e.CompletedAt
		payloadBytes, err = protojson.Marshal(mapWorkoutSessionCompletedToProto(&e))

	case *domainEvent.WorkoutSessionAborted:
		aggregateType, aggregateID, partitionKey, eventType = "WorkoutSession", e.SessionID, e.UserID, e.EventName()
		occurredAt = e.AbortedAt
		payloadBytes, err = protojson.Marshal(&workoutexecutionv1event.WorkoutSessionAborted{
			SessionId: e.SessionID,
			UserId:    e.UserID,
			PlanId:    e.PlanID,
			AbortedAt: timestamppb.New(e.AbortedAt),
			Reason:    e.Reason,
		})
	case domainEvent.WorkoutSessionAborted:
		aggregateType, aggregateID, partitionKey, eventType = "WorkoutSession", e.SessionID, e.UserID, e.EventName()
		occurredAt = e.AbortedAt
		payloadBytes, err = protojson.Marshal(&workoutexecutionv1event.WorkoutSessionAborted{
			SessionId: e.SessionID,
			UserId:    e.UserID,
			PlanId:    e.PlanID,
			AbortedAt: timestamppb.New(e.AbortedAt),
			Reason:    e.Reason,
		})

	case *domainEvent.NewPersonalRecordAchieved:
		aggregateType, aggregateID, partitionKey, eventType = "PersonalRecord", e.ExerciseID, e.UserID, e.EventName()
		occurredAt = e.AchievedAt
		payloadBytes, err = protojson.Marshal(&workoutexecutionv1event.NewPersonalRecordAchieved{
			UserId:     e.UserID,
			ExerciseId: e.ExerciseID,
			OneRepMax:  e.OneRepMax,
			Weight:     e.Weight,
			Reps:       int32(e.Reps),
			AchievedAt: timestamppb.New(e.AchievedAt),
		})
	case domainEvent.NewPersonalRecordAchieved:
		aggregateType, aggregateID, partitionKey, eventType = "PersonalRecord", e.ExerciseID, e.UserID, e.EventName()
		occurredAt = e.AchievedAt
		payloadBytes, err = protojson.Marshal(&workoutexecutionv1event.NewPersonalRecordAchieved{
			UserId:     e.UserID,
			ExerciseId: e.ExerciseID,
			OneRepMax:  e.OneRepMax,
			Weight:     e.Weight,
			Reps:       int32(e.Reps),
			AchievedAt: timestamppb.New(e.AchievedAt),
		})

	case *domainEvent.WorkoutSessionShared:
		aggregateType, aggregateID, partitionKey, eventType = "WorkoutSession", e.SessionID, e.UserID, e.EventName()
		occurredAt = e.SharedAt
		payloadBytes, err = protojson.Marshal(mapWorkoutSessionSharedToProto(e))
	case domainEvent.WorkoutSessionShared:
		aggregateType, aggregateID, partitionKey, eventType = "WorkoutSession", e.SessionID, e.UserID, e.EventName()
		occurredAt = e.SharedAt
		payloadBytes, err = protojson.Marshal(mapWorkoutSessionSharedToProto(&e))

	case *domainEvent.SoloTrackingCompleted:
		aggregateType, aggregateID, partitionKey, eventType = "SoloMatch", e.MatchID, e.UserID, e.EventName()
		occurredAt = e.CompletedAt
		payloadBytes, err = protojson.Marshal(mapSoloTrackingCompletedToProto(e))
	case domainEvent.SoloTrackingCompleted:
		aggregateType, aggregateID, partitionKey, eventType = "SoloMatch", e.MatchID, e.UserID, e.EventName()
		occurredAt = e.CompletedAt
		payloadBytes, err = protojson.Marshal(mapSoloTrackingCompletedToProto(&e))

	case *domainEvent.SoloTrackingAborted:
		aggregateType, aggregateID, partitionKey, eventType = "SoloMatch", e.MatchID, e.UserID, e.EventName()
		occurredAt = e.AbortedAt
		payloadBytes, err = protojson.Marshal(mapSoloTrackingAbortedToProto(e))
	case domainEvent.SoloTrackingAborted:
		aggregateType, aggregateID, partitionKey, eventType = "SoloMatch", e.MatchID, e.UserID, e.EventName()
		occurredAt = e.AbortedAt
		payloadBytes, err = protojson.Marshal(mapSoloTrackingAbortedToProto(&e))

	case *domainEvent.SoloMatchRoomReady:
		aggregateType, aggregateID, partitionKey, eventType = "SoloMatch", e.MatchID, e.MatchID, e.EventName()
		occurredAt = e.StartedAt
		payloadBytes, err = protojson.Marshal(&workoutexecutionv1event.SoloMatchRoomReady{
			MatchId:   e.MatchID,
			StartedAt: timestamppb.New(e.StartedAt),
		})
	case domainEvent.SoloMatchRoomReady:
		aggregateType, aggregateID, partitionKey, eventType = "SoloMatch", e.MatchID, e.MatchID, e.EventName()
		occurredAt = e.StartedAt
		payloadBytes, err = protojson.Marshal(&workoutexecutionv1event.SoloMatchRoomReady{
			MatchId:   e.MatchID,
			StartedAt: timestamppb.New(e.StartedAt),
		})

	case *domainEvent.BodyMetricUpdated:
		aggregateType, aggregateID, partitionKey, eventType = "BodyMetric", e.UserID, e.UserID, e.EventName()
		occurredAt = e.RecordedAt
		payloadBytes, err = json.Marshal(any(ev))
	case domainEvent.BodyMetricUpdated:
		aggregateType, aggregateID, partitionKey, eventType = "BodyMetric", e.UserID, e.UserID, e.EventName()
		occurredAt = e.RecordedAt
		payloadBytes, err = json.Marshal(any(ev))

	case *domainEvent.MotionSpecificationUpdated:
		aggregateType, aggregateID, partitionKey, eventType = "MotionSpecification", e.ExerciseID, e.ExerciseID, e.EventName()
		occurredAt = e.UpdatedAt
		payloadBytes, err = json.Marshal(any(ev))
	case domainEvent.MotionSpecificationUpdated:
		aggregateType, aggregateID, partitionKey, eventType = "MotionSpecification", e.ExerciseID, e.ExerciseID, e.EventName()
		occurredAt = e.UpdatedAt
		payloadBytes, err = json.Marshal(any(ev))

	default:
		eventType = "contracts.core.workout_execution.v1.unknownEvent"
		if np, ok := ev.(NameProvider); ok {
			eventType = np.EventName()
		}
		aggregateType = "Generic"
		partitionKey = extractUserID(ev)
		payloadBytes, err = json.Marshal(any(ev))
	}

	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}

	if partitionKey == "" {
		partitionKey = aggregateID
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	return w.writeCloudEvent(ctx, aggregateType, aggregateID, eventType, partitionKey, occurredAt, payloadBytes)
}

func (w *OutboxWriter) writeCloudEvent(ctx context.Context, aggregateType, aggregateID, eventType, partitionKey string, occurredAt time.Time, payloadBytes []byte) error {
	eventID := uuid.NewString()

	cloudEvent := map[string]interface{}{
		"specversion":     "1.0",
		"id":              eventID,
		"source":          "services/workout-execution-service",
		"type":            eventType,
		"time":            occurredAt.Format(time.RFC3339),
		"datacontenttype": "application/json",
		"data":            json.RawMessage(payloadBytes),
	}

	envelopeBytes, err := json.Marshal(cloudEvent)
	if err != nil {
		return fmt.Errorf("marshal cloud event envelope: %w", err)
	}

	record := &port.OutboxRecord{
		ID:            uuid.NewString(),
		EventID:       eventID,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       envelopeBytes,
		PartitionKey:  partitionKey,
		Published:     false,
		CreatedAt:     occurredAt,
	}

	if err := w.outboxRepo.Save(ctx, record); err != nil {
		return fmt.Errorf("save outbox record: %w", err)
	}

	return nil
}

func extractUserID(ev interface{}) string {
	if ev == nil {
		return ""
	}
	v := reflect.ValueOf(ev)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		field := v.FieldByName("UserID")
		if field.IsValid() && field.Kind() == reflect.String {
			return field.String()
		}
	}
	return ""
}

func mapSoloTrackingCompletedToProto(e *domainEvent.SoloTrackingCompleted) *workoutexecutionv1event.SoloTrackingCompleted {
	return &workoutexecutionv1event.SoloTrackingCompleted{
		MatchId:              e.MatchID,
		UserId:               e.UserID,
		ExerciseId:           e.ExerciseID,
		ChallengeType:        e.ChallengeType.String(),
		ConfirmedValue:       e.ConfirmedValue,
		AverageFormScore:     e.AverageFormScore,
		TotalDurationSeconds: e.TotalDurationSeconds,
		CompletedAt:          timestamppb.New(e.CompletedAt),
		PlanId:               e.PlanID,
	}
}

func mapSoloTrackingAbortedToProto(e *domainEvent.SoloTrackingAborted) *workoutexecutionv1event.SoloTrackingAborted {
	return &workoutexecutionv1event.SoloTrackingAborted{
		MatchId:              e.MatchID,
		UserId:               e.UserID,
		ExerciseId:           e.ExerciseID,
		Reason:               e.Reason,
		AbortedAt:            timestamppb.New(e.AbortedAt),
		PlanId:               e.PlanID,
		ConfirmedValue:       e.ConfirmedValue,
		TotalDurationSeconds: e.TotalDurationSeconds,
	}
}

func mapWorkoutSessionCompletedToProto(e *domainEvent.WorkoutSessionCompleted) *workoutexecutionv1event.WorkoutSessionCompleted {
	var avgScore *float32
	if e.Summary.AverageFormScore != nil {
		score := float32(*e.Summary.AverageFormScore)
		avgScore = &score
	}

	return &workoutexecutionv1event.WorkoutSessionCompleted{
		SessionId:        e.SessionID,
		UserId:           e.UserID,
		PlanId:           e.PlanID,
		CompletedAt:      timestamppb.New(e.CompletedAt),
		TotalSets:        int32(e.Summary.TotalSets),
		TotalVolume:      float32(e.Summary.TotalVolume),
		AverageFormScore: avgScore,
		AverageRpe:       float32(e.Summary.AverageRPE),
	}
}

func mapWorkoutSessionSharedToProto(e *domainEvent.WorkoutSessionShared) *workoutexecutionv1event.WorkoutSessionShared {
	return &workoutexecutionv1event.WorkoutSessionShared{
		SessionId:       e.SessionID,
		UserId:          e.UserID,
		Caption:         e.Caption,
		MediaUrls:       e.MediaURLs,
		Visibility:      e.Visibility,
		DurationSeconds: e.DurationSeconds,
		TotalSets:       e.TotalSets,
		TotalVolumeKg:   e.TotalVolumeKg,
		SharedAt:        timestamppb.New(e.SharedAt),
		WorkoutTitle:    e.WorkoutTitle,
		ExerciseCount:   e.ExerciseCount,
	}
}
