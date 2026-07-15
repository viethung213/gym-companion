package command

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
)

type exerciseEventPayload struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	BodyPartID         string   `json:"bodyPartId"`
	EquipmentID        string   `json:"equipmentId"`
	TargetMuscleID     string   `json:"targetMuscleId"`
	Instructions       string   `json:"instructions"`
	SecondaryMuscleIDs []string `json:"secondaryMuscleIds"`
	ThumbnailURL       string   `json:"thumbnailUrl"`
	MediaURL           string   `json:"mediaUrl"`
	VideoURL           string   `json:"videoUrl"`
	Difficulty         string   `json:"difficulty"`
	DefaultRestSeconds int32    `json:"defaultRestSeconds"`
	TagIDs             []string `json:"tagIds"`
	Status             string   `json:"status"`
}

func newExerciseEventPayload(info *domain.Info) exerciseEventPayload {
	return exerciseEventPayload{
		ID:                 info.ID,
		Name:               info.Name,
		BodyPartID:         info.BodyPartID,
		EquipmentID:        info.EquipmentID,
		TargetMuscleID:     info.TargetMuscleID,
		Instructions:       info.Instructions,
		SecondaryMuscleIDs: info.SecondaryMuscleIDs,
		ThumbnailURL:       info.ThumbnailURL,
		MediaURL:           info.MediaURL,
		VideoURL:           info.VideoURL,
		Difficulty:         info.Difficulty,
		DefaultRestSeconds: info.DefaultRestSeconds,
		TagIDs:             info.TagIDs,
		Status:             string(info.Status),
	}
}

func newEvent(
	ids port.IDGenerator,
	eventType string,
	exercise *domain.Exercise,
	now time.Time,
) (*domain.Event, error) {
	info := exercise.Info()
	payload, err := json.Marshal(newExerciseEventPayload(&info))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domain.ErrInvalidOutboxPayload, err)
	}

	id, err := ids.NewID()
	if err != nil {
		return nil, fmt.Errorf("generate event id: %w", err)
	}

	return &domain.Event{
		ID:           id,
		Type:         eventType,
		PartitionKey: exercise.Info().ID,
		Payload:      payload,
		CreatedAt:    now,
	}, nil
}
