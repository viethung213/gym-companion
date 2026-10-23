package entity

import (
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
)

// CoinLedgerEntry represents an immutable, append-only FitCoins transaction record.
type CoinLedgerEntry struct {
	ID             string
	UserID         string
	Amount         int64
	BalanceAfter   int64
	Reason         vo.CoinTransactionReason
	ReferenceID    *string
	IdempotencyKey string
	ActivityDate   string // YYYY-MM-DD
	AdminID        *string
	Note           *string
	CreatedAt      time.Time
}

// NewCoinLedgerEntry creates a new immutable FitCoins ledger entry.
func NewCoinLedgerEntry(
	id string,
	userID string,
	amount int64,
	balanceAfter int64,
	reason vo.CoinTransactionReason,
	referenceID *string,
	idempotencyKey string,
	activityDate string,
	adminID *string,
	note *string,
	createdAt time.Time,
) *CoinLedgerEntry {
	return &CoinLedgerEntry{
		ID:             id,
		UserID:         userID,
		Amount:         amount,
		BalanceAfter:   balanceAfter,
		Reason:         reason,
		ReferenceID:    referenceID,
		IdempotencyKey: idempotencyKey,
		ActivityDate:   activityDate,
		AdminID:        adminID,
		Note:           note,
		CreatedAt:      createdAt,
	}
}
