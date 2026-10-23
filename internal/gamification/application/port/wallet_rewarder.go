package port

import (
	"context"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
)

// AwardCoinsInput encapsulates the parameters for crediting FitCoins to a user's wallet.
type AwardCoinsInput struct {
	UserID         string
	Amount         int64
	Reason         vo.CoinTransactionReason
	ReferenceID    *string
	IdempotencyKey string
	ActivityDate   string // YYYY-MM-DD
	OccurredAt     time.Time
}

// AwardCoinsOutput represents the result of an AwardCoins execution.
type AwardCoinsOutput struct {
	TransactionID string
	BalanceAfter  int64
	Skipped       bool
}

// WalletRewarder defines the in-process port for awarding FitCoins within an active transaction.
type WalletRewarder interface {
	AwardCoins(ctx context.Context, in AwardCoinsInput) (*AwardCoinsOutput, error)
}
