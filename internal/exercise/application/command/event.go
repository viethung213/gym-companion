package command

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	exercisev1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/event"
	exercisev1msg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func newEvent(
	ids port.IDGenerator,
	eventType string,
	exercise *domain.Exercise,
	now time.Time,
) (*domain.Event, error) {
	info := exercise.Info()
	exerciseInfo := mapToProtoExerciseInfo(&info)

	var protoMsg proto.Message
	switch eventType {
	case domain.EventTypeExerciseCreated:
		protoMsg = &exercisev1event.ExerciseCreatedEvent{
			ExerciseId: exerciseInfo.Id,
			Exercise:   exerciseInfo,
		}
	case domain.EventTypeExerciseApproved:
		protoMsg = &exercisev1event.ExerciseApprovedEvent{
			ExerciseId: exerciseInfo.Id,
			Exercise:   exerciseInfo,
		}
	case domain.EventTypeExerciseArchived:
		protoMsg = &exercisev1event.ExerciseArchivedEvent{
			ExerciseId: exerciseInfo.Id,
			Exercise:   exerciseInfo,
		}
	case domain.EventTypeExerciseRestored:
		protoMsg = &exercisev1event.ExerciseRestoredEvent{
			ExerciseId: exerciseInfo.Id,
			Exercise:   exerciseInfo,
		}
	default:
		return nil, fmt.Errorf("unknown event type: %s", eventType)
	}

	payloadBytes, err := protojson.Marshal(protoMsg)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domain.ErrInvalidOutboxPayload, err)
	}

	id, err := ids.NewID()
	if err != nil {
		return nil, fmt.Errorf("generate event id: %w", err)
	}

	// Wrap payload inside CloudEvents envelope to align with AGENTS.md conventions
	cloudEvent := map[string]any{
		"specversion":     "1.0",
		"id":              id,
		"source":          "services/exercise-service",
		"type":            eventType,
		"time":            now.Format(time.RFC3339),
		"datacontenttype": "application/json",
		"data":            json.RawMessage(payloadBytes),
	}

	envelopeBytes, err := json.Marshal(cloudEvent)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domain.ErrInvalidOutboxPayload, err)
	}

	return &domain.Event{
		ID:           id,
		Type:         eventType,
		PartitionKey: info.ID,
		Payload:      envelopeBytes,
		CreatedAt:    now,
	}, nil
}

// mapToProtoExerciseInfo builds the event payload. Images stay relative:
// consumers only read id and name.
func mapToProtoExerciseInfo(info *domain.Info) *exercisev1msg.ExerciseInfo {
	secondary := make([]exercisev1msg.MuscleGroup, len(info.SecondaryMuscles))
	for i, m := range info.SecondaryMuscles {
		secondary[i] = exercisev1msg.MuscleGroup(protoValue(exercisev1msg.MuscleGroup_value, "MUSCLE_GROUP_", m))
	}

	return &exercisev1msg.ExerciseInfo{
		Id:                 info.ID,
		Slug:               info.Slug,
		Name:               info.Name,
		PrimaryMuscle:      exercisev1msg.MuscleGroup(protoValue(exercisev1msg.MuscleGroup_value, "MUSCLE_GROUP_", info.PrimaryMuscle)),
		SecondaryMuscles:   secondary,
		Equipment:          exercisev1msg.Equipment(protoValue(exercisev1msg.Equipment_value, "EQUIPMENT_", info.Equipment)),
		Category:           exercisev1msg.ExerciseCategory(protoValue(exercisev1msg.ExerciseCategory_value, "EXERCISE_CATEGORY_", info.Category)),
		Level:              exercisev1msg.ExerciseLevel(protoValue(exercisev1msg.ExerciseLevel_value, "EXERCISE_LEVEL_", info.Level)),
		Force:              exercisev1msg.ExerciseForce(protoValue(exercisev1msg.ExerciseForce_value, "EXERCISE_FORCE_", info.Force)),
		Mechanic:           exercisev1msg.ExerciseMechanic(protoValue(exercisev1msg.ExerciseMechanic_value, "EXERCISE_MECHANIC_", info.Mechanic)),
		Instructions:       info.Instructions,
		Images:             info.Images,
		VideoUrl:           info.VideoURL,
		DefaultRestSeconds: info.DefaultRestSeconds,
		MotionSlug:         info.MotionSlug,
		Status:             mapToProtoStatus(info.Status),
		HasAiSupported:     info.HasAISupported,
		MeasurementType: exercisev1msg.ExerciseMeasurementType(
			protoValue(exercisev1msg.ExerciseMeasurementType_value, "EXERCISE_MEASUREMENT_TYPE_", info.MeasurementType)),
	}
}

// protoValue resolves a domain code to its enum number by the contract's
// naming convention (prefix + code); unknown or empty codes give UNSPECIFIED.
func protoValue[T ~string](values map[string]int32, prefix string, code T) int32 {
	return values[prefix+string(code)]
}

func mapToProtoStatus(status domain.Status) exercisev1msg.ExerciseStatus {
	switch status {
	case domain.StatusDraft:
		return exercisev1msg.ExerciseStatus_EXERCISE_STATUS_DRAFT
	case domain.StatusActive:
		return exercisev1msg.ExerciseStatus_EXERCISE_STATUS_ACTIVE
	case domain.StatusArchived:
		return exercisev1msg.ExerciseStatus_EXERCISE_STATUS_ARCHIVED
	default:
		return exercisev1msg.ExerciseStatus_EXERCISE_STATUS_UNSPECIFIED
	}
}
