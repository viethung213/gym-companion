package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/viethung213/gym-companion/internal/exercise/application/command"
)

type MotionSpecEventPayload struct {
	ExerciseID string `json:"exerciseId"`
	IsReady    bool   `json:"isReady"`
}

type MotionSpecConsumer struct {
	handler *command.SetAISupportedHandler
}

func NewMotionSpecConsumer(handler *command.SetAISupportedHandler) *MotionSpecConsumer {
	return &MotionSpecConsumer{handler: handler}
}

func (c *MotionSpecConsumer) ConsumeMotionSpecReady(ctx context.Context, payload []byte) error {
	var event MotionSpecEventPayload
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("unmarshal MotionSpecificationReady event: %w", err)
	}

	if event.ExerciseID == "" {
		return errors.New("exerciseId is required in event payload")
	}

	supported := true
	if event.IsReady {
		supported = true
	}

	log.Printf("MotionSpecConsumer: updating exercise %s has_ai_supported = %v", event.ExerciseID, supported)
	_, err := c.handler.Handle(ctx, command.SetAISupportedCommand{
		ID:        event.ExerciseID,
		Supported: supported,
	})
	if err != nil {
		return fmt.Errorf("handle SetAISupportedCommand: %w", err)
	}

	return nil
}
