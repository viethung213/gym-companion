package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/derror"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/repository"
)

type JoinMatchmakingQueueCommand struct {
	UserID string
}

type JoinMatchmakingQueueResult struct {
	TicketID  string
	EventChan <-chan *port.MatchFoundEvent
}

type JoinMatchmakingQueueHandler struct {
	queuePort port.MatchmakerQueuePort
	matchRepo repository.SoloMatchRepository
}

func NewJoinMatchmakingQueueHandler(
	queuePort port.MatchmakerQueuePort,
	matchRepo repository.SoloMatchRepository,
) *JoinMatchmakingQueueHandler {
	return &JoinMatchmakingQueueHandler{
		queuePort: queuePort,
		matchRepo: matchRepo,
	}
}

func (h *JoinMatchmakingQueueHandler) Handle(ctx context.Context, cmd JoinMatchmakingQueueCommand) (*JoinMatchmakingQueueResult, error) {
	if h.matchRepo != nil {
		activeMatch, err := h.matchRepo.FindActiveByUserID(ctx, cmd.UserID)
		if err != nil {
			return nil, fmt.Errorf("check active match: %w", err)
		}
		if activeMatch != nil {
			return nil, derror.ErrActiveMatchAlreadyExists
		}
	}

	if h.queuePort != nil {
		inQueue, err := h.queuePort.IsUserInQueue(ctx, cmd.UserID)
		if err != nil {
			return nil, fmt.Errorf("check user in queue: %w", err)
		}
		if inQueue {
			return nil, derror.ErrUserAlreadyInQueue
		}
	}

	ticketID := "ticket_" + uuid.New().String()
	ticket, err := aggregate.NewMatchmakingTicket(ticketID, cmd.UserID, time.Now().UTC())
	if err != nil {
		return nil, fmt.Errorf("create matchmaking ticket: %w", err)
	}

	eventChan, err := h.queuePort.ListenMatch(ctx, ticketID)
	if err != nil {
		return nil, fmt.Errorf("listen match queue: %w", err)
	}

	if err := h.queuePort.Enqueue(ctx, ticket); err != nil {
		if errors.Is(err, derror.ErrUserAlreadyInQueue) {
			return nil, derror.ErrUserAlreadyInQueue
		}
		return nil, fmt.Errorf("enqueue matchmaking ticket: %w", err)
	}

	return &JoinMatchmakingQueueResult{
		TicketID:  ticketID,
		EventChan: eventChan,
	}, nil
}
