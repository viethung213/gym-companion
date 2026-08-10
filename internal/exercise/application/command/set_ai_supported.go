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
	EventID   string
	EventType string
	Payload   []byte
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
	cmd *SetAISupportedCommand,
) (*domain.Exercise, error) {
	if cmd.EventID != "" {
		logRec := &port.OutboxLogRecord{
			ID:        cmd.EventID,
			EventID:   cmd.EventID,
			EventType: cmd.EventType,
			Payload:   cmd.Payload,
			Status:    "SUCCESS",
		}
		if err := h.repo.SetAISupportedWithOutboxLog(ctx, cmd.ID, cmd.Supported, logRec); err != nil {
			return nil, fmt.Errorf("set ai supported with outbox log: %w", err)
		}
		return h.repo.FindByID(ctx, cmd.ID)
	}

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
