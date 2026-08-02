package command

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
)

type SetAISupportedCommand struct {
	ID        string
	Supported bool
}

type SetAISupportedHandler struct {
	repo  port.Repository
	clock port.Clock
}

func NewSetAISupportedHandler(
	repo port.Repository,
	clock port.Clock,
) *SetAISupportedHandler {
	return &SetAISupportedHandler{
		repo:  repo,
		clock: clock,
	}
}

func (h *SetAISupportedHandler) Handle(
	ctx context.Context,
	cmd SetAISupportedCommand,
) (*domain.Exercise, error) {
	exercise, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, fmt.Errorf("find exercise: %w", err)
	}

	now := h.clock.Now()
	if setErr := exercise.SetAISupported(cmd.Supported, now); setErr != nil {
		return nil, setErr
	}

	if err = h.repo.Save(ctx, exercise, nil); err != nil {
		return nil, fmt.Errorf("save exercise: %w", err)
	}

	return exercise, nil
}
