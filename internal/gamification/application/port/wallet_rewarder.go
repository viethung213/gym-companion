package port

import (
	"context"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/event"
	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
)

type AwardCoinsInput struct {
	UserID         string
	Amount         int64
	Reason         vo.CoinTransactionReason
	ReferenceID    *string
	IdempotencyKey string
	ActivityDate   string
	OccurredAt     time.Time
}

type AwardCoinsOutput struct {
	TransactionID string
	BalanceAfter  int64
	Skipped       bool
}

type WalletRewarder interface {
	AwardCoins(ctx context.Context, in AwardCoinsInput) (*AwardCoinsOutput, error)
	AwardCoinsBatch(ctx context.Context, userID string, items []AwardCoinsInput) ([]*AwardCoinsOutput, error)
	HandleEvents(ctx context.Context, userID, activityDate string, occurredAt time.Time, events []event.Event) ([]event.Event, error)
}
