package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

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

func (p *MotionSpecEventPayload) UnmarshalJSON(data []byte) error {
	var raw struct {
		ExerciseIDPascal string `json:"ExerciseID"`
		ExerciseIDCamel  string `json:"exerciseId"`
		ExerciseIDSnake  string `json:"exercise_id"`
		IsReadyPascal    bool   `json:"IsReady"`
		IsReadyCamel     bool   `json:"isReady"`
		IsReadySnake     bool   `json:"is_ready"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	p.ExerciseID = raw.ExerciseIDSnake
	if raw.ExerciseIDCamel != "" {
		p.ExerciseID = raw.ExerciseIDCamel
	}
	if raw.ExerciseIDPascal != "" {
		p.ExerciseID = raw.ExerciseIDPascal
	}
	p.IsReady = raw.IsReadyPascal || raw.IsReadyCamel || raw.IsReadySnake
	return nil
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
	if json.Unmarshal(payload, &env) == nil && env.Type != "" {
		// Ignore unrelated events published to the shared topic 'workout_execution.events'
		if !strings.HasSuffix(env.Type, "motionSpecificationUpdated") && env.Type != "contracts.core.workout_execution.v1.motionSpecificationUpdated" {
			return "", false, "", "", nil
		}
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
	if exerciseID == "" {
		// Event was ignored (e.g. non-motionSpecificationUpdated event on shared topic)
		return nil
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
