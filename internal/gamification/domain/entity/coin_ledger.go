package entity

import (
	"time"
)

// CoinLedgerEntry represents an immutable audit log entry for coin transactions.
type CoinLedgerEntry struct {
	id           string
	userID       string
	adminID      *string
	amount       int64
	balanceAfter int64
	reason       string
	createdAt    time.Time
}

// NewCoinLedgerEntry constructs a new immutable CoinLedgerEntry entity.
func NewCoinLedgerEntry(
	id string,
	userID string,
	adminID *string,
	amount int64,
	balanceAfter int64,
	reason string,
	createdAt time.Time,
) *CoinLedgerEntry {
	return &CoinLedgerEntry{
		id:           id,
		userID:       userID,
		adminID:      adminID,
		amount:       amount,
		balanceAfter: balanceAfter,
		reason:       reason,
		createdAt:    createdAt,
	}
}

// ID returns the unique ledger entry identifier.
func (e *CoinLedgerEntry) ID() string {
	return e.id
}

// UserID returns the target user identifier.
func (e *CoinLedgerEntry) UserID() string {
	return e.userID
}

// AdminID returns the identifier of the administrator who performed the adjustment, if applicable.
func (e *CoinLedgerEntry) AdminID() *string {
	return e.adminID
}

// Amount returns the coin delta (+ for credit, - for debit).
func (e *CoinLedgerEntry) Amount() int64 {
	return e.amount
}

// BalanceAfter returns the user's total balance immediately following this entry.
func (e *CoinLedgerEntry) BalanceAfter() int64 {
	return e.balanceAfter
}

// Reason returns the reason code for this transaction.
func (e *CoinLedgerEntry) Reason() string {
	return e.reason
}

// CreatedAt returns the immutable creation timestamp.
func (e *CoinLedgerEntry) CreatedAt() time.Time {
	return e.createdAt
}
