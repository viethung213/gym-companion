package aggregate

import (
	"errors"
	"time"
)

var (
	// ErrInvalidUserID indicates an ID string that cannot be parsed as a valid UUID.
	ErrInvalidUserID = errors.New("user id must be a valid UUID")

	// ErrZeroCoinAmount indicates a coin transaction with amount 0.
	ErrZeroCoinAmount = errors.New("coin amount cannot be zero")

	// ErrInsufficientBalance indicates deducting more coins than the user currently has.
	ErrInsufficientBalance = errors.New("insufficient wallet balance")

	// ErrCoinAmountOutOfRange indicates an adjustment amount exceeding safety bounds.
	ErrCoinAmountOutOfRange = errors.New("coin amount exceeds maximum single adjustment range (-1,000,000 to +1,000,000)")
)

// UserWallet is the aggregate root managing the user's FitCoins virtual currency balance.
type UserWallet struct {
	userID    string
	balance   int64
	createdAt time.Time
	updatedAt time.Time
}

// NewUserWallet constructs a new UserWallet aggregate initialized with 0 balance.
func NewUserWallet(userID string) (*UserWallet, error) {
	if userID == "" {
		return nil, ErrEmptyUserID
	}
	now := time.Now().UTC()
	return &UserWallet{
		userID:    userID,
		balance:   0,
		createdAt: now,
		updatedAt: now,
	}, nil
}

// ReconstituteUserWallet restores an existing UserWallet from persistence.
func ReconstituteUserWallet(
	userID string,
	balance int64,
	createdAt time.Time,
	updatedAt time.Time,
) *UserWallet {
	return &UserWallet{
		userID:    userID,
		balance:   balance,
		createdAt: createdAt,
		updatedAt: updatedAt,
	}
}

// UserID returns the aggregate identifier.
func (w *UserWallet) UserID() string {
	return w.userID
}

// Balance returns the user's current FitCoins balance.
func (w *UserWallet) Balance() int64 {
	return w.balance
}

// CreatedAt returns the wallet creation timestamp.
func (w *UserWallet) CreatedAt() time.Time {
	return w.createdAt
}

// UpdatedAt returns the timestamp when the wallet was last modified.
func (w *UserWallet) UpdatedAt() time.Time {
	return w.updatedAt
}

// AdjustBalance directly adjusts coins (+ to add, - to subtract) preserving invariants.
func (w *UserWallet) AdjustBalance(amount int64, now time.Time) error {
	if amount == 0 {
		return ErrZeroCoinAmount
	}
	if amount > 1_000_000 || amount < -1_000_000 {
		return ErrCoinAmountOutOfRange
	}

	newBalance := w.balance + amount
	if newBalance < 0 {
		return ErrInsufficientBalance
	}

	w.balance = newBalance
	w.updatedAt = now
	return nil
}
