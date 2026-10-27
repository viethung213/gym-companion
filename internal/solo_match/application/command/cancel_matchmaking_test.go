package command_test

import (
	"context"
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/solo_match/application/command"
	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
)

type cancelQueuePortMock struct {
	canceled  []string
	cancelErr error
}

func (q *cancelQueuePortMock) Enqueue(ctx context.Context, ticket *aggregate.MatchmakingTicket) error {
	return nil
}

func (q *cancelQueuePortMock) Cancel(ctx context.Context, ticketID string, userID string) error {
	if q.cancelErr != nil {
		return q.cancelErr
	}
	q.canceled = append(q.canceled, ticketID)
	return nil
}

func (q *cancelQueuePortMock) ListenMatch(ctx context.Context, ticketID string) (<-chan *port.MatchFoundEvent, error) {
	return nil, nil
}

func (q *cancelQueuePortMock) IsUserInQueue(ctx context.Context, userID string) (bool, error) {
	return false, nil
}

func (q *cancelQueuePortMock) Close() error {
	return nil
}

func TestCancelMatchmakingHandler(t *testing.T) {
	ctx := context.Background()

	t.Run("successfully cancels matchmaking ticket", func(t *testing.T) {
		queuePort := &cancelQueuePortMock{canceled: make([]string, 0)}
		handler := command.NewCancelMatchmakingHandler(queuePort)

		cmd := command.CancelMatchmakingCommand{
			TicketID: "ticket_123",
			UserID:   "user_123",
		}

		err := handler.Handle(ctx, cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(queuePort.canceled) != 1 || queuePort.canceled[0] != "ticket_123" {
			t.Errorf("expected ticket_123 in canceled list, got %v", queuePort.canceled)
		}
	})

	t.Run("fails when queue port returns error", func(t *testing.T) {
		queuePort := &cancelQueuePortMock{
			canceled:  make([]string, 0),
			cancelErr: errors.New("redis connection refused"),
		}
		handler := command.NewCancelMatchmakingHandler(queuePort)

		cmd := command.CancelMatchmakingCommand{
			TicketID: "ticket_123",
			UserID:   "user_123",
		}

		err := handler.Handle(ctx, cmd)
		if err == nil {
			t.Errorf("expected error when queue port fails, got nil")
		}
	})
}
