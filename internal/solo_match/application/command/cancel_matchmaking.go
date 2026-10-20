package command

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
)

type CancelMatchmakingCommand struct {
	TicketID string
	UserID   string
}

type CancelMatchmakingHandler struct {
	queuePort port.MatchmakerQueuePort
}

func NewCancelMatchmakingHandler(queuePort port.MatchmakerQueuePort) *CancelMatchmakingHandler {
	return &CancelMatchmakingHandler{queuePort: queuePort}
}

func (h *CancelMatchmakingHandler) Handle(ctx context.Context, cmd CancelMatchmakingCommand) error {
	if err := h.queuePort.Cancel(ctx, cmd.TicketID, cmd.UserID); err != nil {
		return fmt.Errorf("cancel matchmaking queue: %w", err)
	}
	return nil
}
