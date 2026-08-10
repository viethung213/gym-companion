package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/viethung213/gym-companion/internal/exercise/application/command"
)

type cloudEventEnvelope struct {
	ID   string          `json:"id"`
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type MotionSpecEventPayload struct {
	ExerciseID string `json:"ExerciseID"`
	IsReady    bool   `json:"IsReady"`
}

type MotionSpecConsumer struct {
	handler *command.SetAISupportedHandler
}

func NewMotionSpecConsumer(handler *command.SetAISupportedHandler) *MotionSpecConsumer {
	return &MotionSpecConsumer{handler: handler}
}

func parseMotionSpecPayload(payload []byte) (exerciseID string, isReady bool, eventID, eventType string, err error) {
	dataBytes := payload
	eventType = "contracts.core.workout_execution.v1.motionSpecificationUpdated"

	// If payload is wrapped in CloudEvent envelope ({"specversion":"1.0", "data":{...}})
	var env cloudEventEnvelope
	if json.Unmarshal(payload, &env) == nil {
		if env.ID != "" {
			eventID = env.ID
		}
		if env.Type != "" {
			eventType = env.Type
		}
		if len(env.Data) > 0 {
			dataBytes = env.Data
		}
	}

	var data MotionSpecEventPayload
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return "", false, "", "", fmt.Errorf("unmarshal MotionSpecification event payload: %w", err)
	}

	if data.ExerciseID == "" {
		return "", false, "", "", errors.New("ExerciseID is required in event payload")
	}

	return data.ExerciseID, data.IsReady, eventID, eventType, nil
}

func (c *MotionSpecConsumer) ConsumeMotionSpecReady(ctx context.Context, payload []byte) error {
	exerciseID, isReady, eventID, eventType, err := parseMotionSpecPayload(payload)
	if err != nil {
		return err
	}

	log.Printf("MotionSpecConsumer: updating exercise %s has_ai_supported = %v (event_id: %s)", exerciseID, isReady, eventID)
	_, err = c.handler.Handle(ctx, &command.SetAISupportedCommand{
		ID:        exerciseID,
		Supported: isReady,
		EventID:   eventID,
		EventType: eventType,
		Payload:   payload,
	})
	if err != nil {
		return fmt.Errorf("handle SetAISupportedCommand: %w", err)
	}

	return nil
}
