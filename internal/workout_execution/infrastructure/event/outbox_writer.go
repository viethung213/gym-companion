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
	aggregateType, aggregateID, partitionKey, eventType := extractEventMetadata(ev)

	var payloadBytes []byte
	var err error
	if protoMsg := mapToProtoEvent(ev); protoMsg != nil {
		payloadBytes, err = protojson.Marshal(protoMsg)
		if err != nil {
			return fmt.Errorf("marshal proto event payload: %w", err)
		}
	} else {
		payloadBytes, err = json.Marshal(ev)
		if err != nil {
			return fmt.Errorf("marshal event payload: %w", err)
		}
	}

	outboxID := uuid.NewString()
	eventID := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339)

	cloudEvent := map[string]interface{}{
		"specversion":     "1.0",
		"id":              eventID,
		"source":          "services/workout-execution-service",
		"type":            eventType,
		"time":            now,
		"datacontenttype": "application/json",
		"data":            json.RawMessage(payloadBytes),
	}

	envelopeBytes, err := json.Marshal(cloudEvent)
	if err != nil {
		return fmt.Errorf("marshal cloud event envelope: %w", err)
	}

	if partitionKey == "" {
		partitionKey = aggregateID
	}

	record := &port.OutboxRecord{
		ID:            outboxID,
		EventID:       eventID,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       envelopeBytes,
		PartitionKey:  partitionKey,
		Published:     false,
		CreatedAt:     time.Now().UTC(),
	}

	if err := w.outboxRepo.Save(ctx, record); err != nil {
		return fmt.Errorf("save outbox record: %w", err)
	}

	return nil
}

func extractEventMetadata(ev any) (aggregateType, aggregateID, partitionKey, eventType string) {
	if ev == nil {
		return "Unknown", "", "", "contracts.core.workout_execution.v1.unknownEvent"
	}
	switch e := ev.(type) {
	case domainEvent.WorkoutSessionStarted:
		return "WorkoutSession", e.SessionID, e.UserID, e.EventName()
	case *domainEvent.WorkoutSessionStarted:
		if e != nil {
			return "WorkoutSession", e.SessionID, e.UserID, e.EventName()
		}
	case domainEvent.WorkoutSessionCompleted:
		return "WorkoutSession", e.SessionID, e.UserID, e.EventName()
	case *domainEvent.WorkoutSessionCompleted:
		if e != nil {
			return "WorkoutSession", e.SessionID, e.UserID, e.EventName()
		}
	case domainEvent.WorkoutSessionAborted:
		return "WorkoutSession", e.SessionID, e.UserID, e.EventName()
	case *domainEvent.WorkoutSessionAborted:
		if e != nil {
			return "WorkoutSession", e.SessionID, e.UserID, e.EventName()
		}
	case domainEvent.NewPersonalRecordAchieved:
		return "PersonalRecord", e.ExerciseID, e.UserID, e.EventName()
	case *domainEvent.NewPersonalRecordAchieved:
		if e != nil {
			return "PersonalRecord", e.ExerciseID, e.UserID, e.EventName()
		}
	case domainEvent.BodyMetricUpdated:
		return "BodyMetric", e.UserID, e.UserID, e.EventName()
	case *domainEvent.BodyMetricUpdated:
		if e != nil {
			return "BodyMetric", e.UserID, e.UserID, e.EventName()
		}
	case domainEvent.MotionSpecificationUpdated:
		return "MotionSpecification", e.ExerciseID, e.ExerciseID, e.EventName()
	case *domainEvent.MotionSpecificationUpdated:
		if e != nil {
			return "MotionSpecification", e.ExerciseID, e.ExerciseID, e.EventName()
		}
	case domainEvent.WorkoutSessionShared:
		return "WorkoutSession", e.SessionID, e.UserID, e.EventName()
	case *domainEvent.WorkoutSessionShared:
		if e != nil {
			return "WorkoutSession", e.SessionID, e.UserID, e.EventName()
		}
	case domainEvent.SoloTrackingCompleted:
		return "SoloMatch", e.MatchID, e.UserID, e.EventName()
	case *domainEvent.SoloTrackingCompleted:
		if e != nil {
			return "SoloMatch", e.MatchID, e.UserID, e.EventName()
		}
	case domainEvent.SoloTrackingAborted:
		return "SoloMatch", e.MatchID, e.UserID, e.EventName()
	case *domainEvent.SoloTrackingAborted:
		if e != nil {
			return "SoloMatch", e.MatchID, e.UserID, e.EventName()
		}
	case domainEvent.SoloMatchRoomReady:
		return "SoloMatch", e.MatchID, e.MatchID, e.EventName()
	case *domainEvent.SoloMatchRoomReady:
		if e != nil {
			return "SoloMatch", e.MatchID, e.MatchID, e.EventName()
		}
	}

	eventType = "contracts.core.workout_execution.v1.unknownEvent"
	if nameProvider, ok := ev.(NameProvider); ok {
		eventType = nameProvider.EventName()
	} else if ev != nil && reflect.TypeOf(ev).Kind() == reflect.Struct {
		vp := reflect.New(reflect.TypeOf(ev))
		vp.Elem().Set(reflect.ValueOf(ev))
		if nameProvider, ok := vp.Interface().(NameProvider); ok {
			eventType = nameProvider.EventName()
		}
	}
	partitionKey = extractUserID(ev)
	return "Generic", "", partitionKey, eventType
}

func extractUserID(ev interface{}) string {
	if ev == nil {
		return ""
	}
	switch e := ev.(type) {
	case domainEvent.WorkoutSessionStarted:
		return e.UserID
	case *domainEvent.WorkoutSessionStarted:
		if e != nil {
			return e.UserID
		}
	case domainEvent.WorkoutSessionCompleted:
		return e.UserID
	case *domainEvent.WorkoutSessionCompleted:
		if e != nil {
			return e.UserID
		}
	case domainEvent.WorkoutSessionAborted:
		return e.UserID
	case *domainEvent.WorkoutSessionAborted:
		if e != nil {
			return e.UserID
		}
	case domainEvent.NewPersonalRecordAchieved:
		return e.UserID
	case *domainEvent.NewPersonalRecordAchieved:
		if e != nil {
			return e.UserID
		}
	case domainEvent.BodyMetricUpdated:
		return e.UserID
	case *domainEvent.BodyMetricUpdated:
		if e != nil {
			return e.UserID
		}
	case domainEvent.WorkoutSessionShared:
		return e.UserID
	case *domainEvent.WorkoutSessionShared:
		if e != nil {
			return e.UserID
		}
	case domainEvent.SoloTrackingCompleted:
		return e.UserID
	case *domainEvent.SoloTrackingCompleted:
		if e != nil {
			return e.UserID
		}
	case domainEvent.SoloTrackingAborted:
		return e.UserID
	case *domainEvent.SoloTrackingAborted:
		if e != nil {
			return e.UserID
		}
	case domainEvent.SoloMatchRoomReady:
		return e.MatchID
	case *domainEvent.SoloMatchRoomReady:
		if e != nil {
			return e.MatchID
		}
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

func mapToProtoEvent(ev interface{}) proto.Message {
	switch e := ev.(type) {
	case domainEvent.WorkoutSessionStarted:
		return &workoutexecutionv1event.WorkoutSessionStarted{
			SessionId: e.SessionID,
			UserId:    e.UserID,
			PlanId:    e.PlanID,
			StartedAt: timestamppb.New(e.StartedAt),
		}
	case *domainEvent.WorkoutSessionStarted:
		if e == nil {
			return nil
		}
		return &workoutexecutionv1event.WorkoutSessionStarted{
			SessionId: e.SessionID,
			UserId:    e.UserID,
			PlanId:    e.PlanID,
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
			UserId:    e.UserID,
			PlanId:    e.PlanID,
			AbortedAt: timestamppb.New(e.AbortedAt),
			Reason:    e.Reason,
		}
	case *domainEvent.WorkoutSessionAborted:
		if e == nil {
			return nil
		}
		return &workoutexecutionv1event.WorkoutSessionAborted{
			SessionId: e.SessionID,
			UserId:    e.UserID,
			PlanId:    e.PlanID,
			AbortedAt: timestamppb.New(e.AbortedAt),
			Reason:    e.Reason,
		}
	case domainEvent.NewPersonalRecordAchieved:
		return &workoutexecutionv1event.NewPersonalRecordAchieved{
			UserId:     e.UserID,
			ExerciseId: e.ExerciseID,
			OneRepMax:  e.OneRepMax,
			Weight:     e.Weight,
			Reps:       int32(e.Reps),
			AchievedAt: timestamppb.New(e.AchievedAt),
		}
	case *domainEvent.NewPersonalRecordAchieved:
		if e == nil {
			return nil
		}
		return &workoutexecutionv1event.NewPersonalRecordAchieved{
			UserId:     e.UserID,
			ExerciseId: e.ExerciseID,
			OneRepMax:  e.OneRepMax,
			Weight:     e.Weight,
			Reps:       int32(e.Reps),
			AchievedAt: timestamppb.New(e.AchievedAt),
		}
	case domainEvent.WorkoutSessionShared:
		return mapWorkoutSessionSharedToProto(&e)
	case *domainEvent.WorkoutSessionShared:
		if e == nil {
			return nil
		}
		return mapWorkoutSessionSharedToProto(e)
	case domainEvent.SoloTrackingCompleted:
		return mapSoloTrackingCompletedToProto(&e)
	case *domainEvent.SoloTrackingCompleted:
		if e == nil {
			return nil
		}
		return mapSoloTrackingCompletedToProto(e)
	case domainEvent.SoloTrackingAborted:
		return mapSoloTrackingAbortedToProto(&e)
	case *domainEvent.SoloTrackingAborted:
		if e == nil {
			return nil
		}
		return mapSoloTrackingAbortedToProto(e)
	case domainEvent.SoloMatchRoomReady:
		return &workoutexecutionv1event.SoloMatchRoomReady{
			MatchId:   e.MatchID,
			StartedAt: timestamppb.New(e.StartedAt),
		}
	case *domainEvent.SoloMatchRoomReady:
		if e == nil {
			return nil
		}
		return &workoutexecutionv1event.SoloMatchRoomReady{
			MatchId:   e.MatchID,
			StartedAt: timestamppb.New(e.StartedAt),
		}
	default:
		return nil
	}
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
