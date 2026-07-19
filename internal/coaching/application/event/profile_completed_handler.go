package event

import (
	"context"
	"fmt"
	"log"

	"github.com/viethung213/gym-companion/internal/coaching/application/command"
)

// ProfileCompletedEvent represents the deserialized UserProfileCompleted event from Kafka.
type ProfileCompletedEvent struct {
	UserID             string
	Goals              []string
	RegisteredInjuries []string
	ExperienceLevel    string
}

// ProfileCompletedHandler receives ProfileCompleted events and dispatches
// the InitiateRoadmapCommand. It contains no business logic.
type ProfileCompletedHandler struct {
	initiateRoadmap *command.InitiateRoadmapHandler
}

// NewProfileCompletedHandler creates a new event handler.
func NewProfileCompletedHandler(initiateRoadmap *command.InitiateRoadmapHandler) *ProfileCompletedHandler {
	return &ProfileCompletedHandler{
		initiateRoadmap: initiateRoadmap,
	}
}

// Handle maps the event to a command and dispatches it.
func (h *ProfileCompletedHandler) Handle(ctx context.Context, evt ProfileCompletedEvent) error {
	log.Printf("INFO: received ProfileCompleted event for user %s, dispatching InitiateRoadmap command", evt.UserID)

	cmd := &command.InitiateRoadmapCommand{
		UserID:             evt.UserID,
		Goals:              evt.Goals,
		RegisteredInjuries: evt.RegisteredInjuries,
		ExperienceLevel:    evt.ExperienceLevel,
	}

	if err := h.initiateRoadmap.Handle(ctx, cmd); err != nil {
		return fmt.Errorf("handle initiate roadmap command: %w", err)
	}

	return nil
}
