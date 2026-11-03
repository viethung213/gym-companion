package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"
	workoutexecutionv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/event"
	domainEvent "github.com/viethung213/gym-companion/internal/workout_execution/domain/event"
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

// NameProvider interface to extract event name string.
type NameProvider interface {
	EventName() string
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

// PackCloudEvent packs a domain event into CloudEvents 1.0 JSON payload and extracts routing metadata.
func PackCloudEvent(ev any) (*OutboxRecordData, error) {
	if ev == nil {
		return nil, errors.New("nil domain event")
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
		return nil, fmt.Errorf("marshal event payload: %w", err)
	}

	if partitionKey == "" {
		partitionKey = aggregateID
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	eventID := uuid.NewString()
	cloudEvent := CloudEventEnvelope{
		SpecVersion:     "1.0",
		ID:              eventID,
		Source:          "services/workout-execution-service",
		Type:            eventType,
		Time:            occurredAt.Format(time.RFC3339),
		DataContentType: "application/json",
		Data:            json.RawMessage(payloadBytes),
	}

	envelopeBytes, err := json.Marshal(cloudEvent)
	if err != nil {
		return nil, fmt.Errorf("marshal cloud event envelope: %w", err)
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
