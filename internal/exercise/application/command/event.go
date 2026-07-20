package command

import (
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
	case domain.EventTypeExerciseSubmittedForApproval:
		protoMsg = &exercisev1event.ExerciseSubmittedForApprovalEvent{
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

func mapToProtoExerciseInfo(info *domain.Info) *exercisev1msg.ExerciseInfo {
	return &exercisev1msg.ExerciseInfo{
		Id:                 info.ID,
		Name:               info.Name,
		BodyPartId:         info.BodyPartID,
		EquipmentId:        info.EquipmentID,
		TargetMuscleId:     info.TargetMuscleID,
		Instructions:       info.Instructions,
		SecondaryMuscleIds: info.SecondaryMuscleIDs,
		ThumbnailUrl:       info.ThumbnailURL,
		MediaUrl:           info.MediaURL,
		VideoUrl:           info.VideoURL,
		Difficulty:         info.Difficulty,
		DefaultRestSeconds: info.DefaultRestSeconds,
		TagIds:             info.TagIDs,
		Status:             mapToProtoStatus(info.Status),
	}
}

func mapToProtoStatus(status domain.Status) exercisev1msg.ExerciseStatus {
	switch status {
	case domain.StatusDraft:
		return exercisev1msg.ExerciseStatus_EXERCISE_STATUS_DRAFT
	case domain.StatusPendingApproval:
		return exercisev1msg.ExerciseStatus_EXERCISE_STATUS_PENDING_APPROVAL
	case domain.StatusActive:
		return exercisev1msg.ExerciseStatus_EXERCISE_STATUS_ACTIVE
	case domain.StatusArchived:
		return exercisev1msg.ExerciseStatus_EXERCISE_STATUS_ARCHIVED
	default:
		return exercisev1msg.ExerciseStatus_EXERCISE_STATUS_UNSPECIFIED
	}
}
