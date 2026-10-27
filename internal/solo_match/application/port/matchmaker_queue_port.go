package port

import (
	"context"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
)

type MatchFoundEvent struct {
	MatchID string
	Match   *aggregate.SoloMatch
	Err     error
}

type MatchmakerQueuePort interface {
	Enqueue(ctx context.Context, ticket *aggregate.MatchmakingTicket) error
	Cancel(ctx context.Context, ticketID string, userID string) error
	ListenMatch(ctx context.Context, ticketID string) (<-chan *MatchFoundEvent, error)
	IsUserInQueue(ctx context.Context, userID string) (bool, error)
	Close() error
}
