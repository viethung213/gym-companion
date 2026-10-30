package aggregate

import (
	"errors"
	"math"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/event"
	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
)

// Validation length constraints matching PostgreSQL schema column limits.
const (
	MaxIdempotencyKeyLength = 150
	MaxReferenceIDLength    = 100
	MaxNoteLength           = 500
)

var (
	// ErrInsufficientFunds indicates the wallet balance is lower than the requested spend/deduction amount.
	ErrInsufficientFunds = errors.New("insufficient fitcoins balance")

	// ErrInvalidCoinAmount indicates a zero or invalid FitCoins transaction amount.
	ErrInvalidCoinAmount = errors.New("coin amount must be valid and non-zero")

	// ErrInvalidTransactionReason indicates a transaction reason that does not match the operation type.
	ErrInvalidTransactionReason = errors.New("invalid coin transaction reason for operation")

	// ErrEmptyIdempotencyKey indicates a missing or overly long idempotency key for a ledger transaction.
	ErrEmptyIdempotencyKey = errors.New("idempotency key must be between 1 and 150 characters")

	// ErrInvalidReferenceID indicates a reference ID exceeding the column length limit.
	ErrInvalidReferenceID = errors.New("reference id cannot exceed 100 characters")

	// ErrEmptyAdminID indicates a missing administrator identifier for an admin adjustment.
	ErrEmptyAdminID = errors.New("admin id cannot be empty for admin adjustment")

	// ErrEmptyAdjustmentNote indicates a missing or overly long audit note for an admin adjustment.
	ErrEmptyAdjustmentNote = errors.New("adjustment note must be between 1 and 500 characters")

	// ErrBalanceOverflow indicates adding the requested amount would overflow int64.
	ErrBalanceOverflow = errors.New("wallet balance overflow")

	// ErrDailyCapReached indicates the user has already claimed the daily FitCoins reward for this reason on the given date.
	ErrDailyCapReached = errors.New("daily coin reward cap already reached for this date")

	// ErrInvalidCursor indicates a malformed pagination cursor for ledger queries.
	ErrInvalidCursor = errors.New("invalid pagination cursor")
)

// UserWallet is the aggregate root managing a user's FitCoins balance and daily reward caps.
type UserWallet struct {
	userID                  string
	balance                 int64
	lastWorkoutRewardDate   *string // YYYY-MM-DD
	lastPRRewardDate        *string // YYYY-MM-DD
	lastNutritionRewardDate *string // YYYY-MM-DD
	updatedAt               time.Time
}

// NewUserWallet constructs a new UserWallet aggregate with 0 balance.
func NewUserWallet(userID string) (*UserWallet, error) {
	if userID == "" {
		return nil, ErrEmptyUserID
	}
	return &UserWallet{
		userID:    userID,
		balance:   0,
		updatedAt: time.Now().UTC(),
	}, nil
}

// ReconstituteUserWallet reassembles a UserWallet aggregate from persistence data.
func ReconstituteUserWallet(
	userID string,
	balance int64,
	lastWorkoutRewardDate *string,
	lastPRRewardDate *string,
	lastNutritionRewardDate *string,
	updatedAt time.Time,
) *UserWallet {
	return &UserWallet{
		userID:                  userID,
		balance:                 balance,
		lastWorkoutRewardDate:   copyOptionalDate(lastWorkoutRewardDate),
		lastPRRewardDate:        copyOptionalDate(lastPRRewardDate),
		lastNutritionRewardDate: copyOptionalDate(lastNutritionRewardDate),
		updatedAt:               updatedAt,
	}
}

// UserID returns the owning user's identifier.
func (w *UserWallet) UserID() string {
	return w.userID
}

// Balance returns the current FitCoins balance.
func (w *UserWallet) Balance() int64 {
	return w.balance
}

// LastWorkoutRewardDate returns a copy of the YYYY-MM-DD date when workout FitCoins were last awarded.
func (w *UserWallet) LastWorkoutRewardDate() *string {
	return copyOptionalDate(w.lastWorkoutRewardDate)
}

// LastPRRewardDate returns a copy of the YYYY-MM-DD date when PR FitCoins were last awarded.
func (w *UserWallet) LastPRRewardDate() *string {
	return copyOptionalDate(w.lastPRRewardDate)
}

// LastNutritionRewardDate returns a copy of the YYYY-MM-DD date when nutrition FitCoins were last awarded.
func (w *UserWallet) LastNutritionRewardDate() *string {
	return copyOptionalDate(w.lastNutritionRewardDate)
}

// UpdatedAt returns the timestamp of the last wallet modification.
func (w *UserWallet) UpdatedAt() time.Time {
	return w.updatedAt
}

// IsDailyCapReached checks in O(1) memory whether the daily cap for the given reason and YYYY-MM-DD date is already reached.
func (w *UserWallet) IsDailyCapReached(reason vo.CoinTransactionReason, activityDate string) bool {
	switch reason {
	case vo.ReasonEarnWorkout:
		return w.lastWorkoutRewardDate != nil && *w.lastWorkoutRewardDate == activityDate
	case vo.ReasonEarnPR:
		return w.lastPRRewardDate != nil && *w.lastPRRewardDate == activityDate
	case vo.ReasonEarnNutrition:
		return w.lastNutritionRewardDate != nil && *w.lastNutritionRewardDate == activityDate
	default:
		return false
	}
}

// Award credits positive FitCoins to the wallet, enforcing O(1) daily caps and int64 overflow protection.
func (w *UserWallet) Award(
	entryID string,
	amount int64,
	reason vo.CoinTransactionReason,
	refID *string,
	idempotencyKey string,
	activityDate string,
	now time.Time,
) (*entity.CoinLedgerEntry, event.Event, error) {
	if amount <= 0 {
		return nil, nil, ErrInvalidCoinAmount
	}
	if !reason.IsEarn() {
		return nil, nil, ErrInvalidTransactionReason
	}
	if idempotencyKey == "" || len(idempotencyKey) > MaxIdempotencyKeyLength {
		return nil, nil, ErrEmptyIdempotencyKey
	}
	if refID != nil && len(*refID) > MaxReferenceIDLength {
		return nil, nil, ErrInvalidReferenceID
	}
	if _, err := time.Parse("2006-01-02", activityDate); err != nil {
		return nil, nil, ErrInvalidDateFormat
	}
	if w.balance > math.MaxInt64-amount {
		return nil, nil, ErrBalanceOverflow
	}

	if w.IsDailyCapReached(reason, activityDate) {
		return nil, nil, ErrDailyCapReached
	}

	w.balance += amount
	w.updatedAt = now

	dateCopy := activityDate
	switch reason {
	case vo.ReasonEarnWorkout:
		w.lastWorkoutRewardDate = &dateCopy
	case vo.ReasonEarnPR:
		w.lastPRRewardDate = &dateCopy
	case vo.ReasonEarnNutrition:
		w.lastNutritionRewardDate = &dateCopy
	}

	entry := entity.NewCoinLedgerEntry(
		entryID,
		w.userID,
		amount,
		w.balance,
		reason,
		copyOptionalString(refID),
		idempotencyKey,
		activityDate,
		nil,
		nil,
		now,
	)

	var refStr string
	if refID != nil {
		refStr = *refID
	}
	ev := &event.CoinsEarned{
		UserID:       w.userID,
		Amount:       amount,
		BalanceAfter: w.balance,
		Reason:       string(reason),
		ReferenceID:  refStr,
		EarnedAt:     now,
	}

	return entry, ev, nil
}

// Spend deducts positive FitCoins from the wallet, rejecting any operation that would result in a negative balance.
func (w *UserWallet) Spend(
	entryID string,
	amount int64,
	reason vo.CoinTransactionReason,
	refID *string,
	idempotencyKey string,
	note *string,
	now time.Time,
) (*entity.CoinLedgerEntry, event.Event, error) {
	if amount <= 0 {
		return nil, nil, ErrInvalidCoinAmount
	}
	if !reason.IsSpend() {
		return nil, nil, ErrInvalidTransactionReason
	}
	if idempotencyKey == "" || len(idempotencyKey) > MaxIdempotencyKeyLength {
		return nil, nil, ErrEmptyIdempotencyKey
	}
	if refID != nil && len(*refID) > MaxReferenceIDLength {
		return nil, nil, ErrInvalidReferenceID
	}
	if note != nil && len(*note) > MaxNoteLength {
		return nil, nil, ErrEmptyAdjustmentNote
	}
	if w.balance < amount {
		return nil, nil, ErrInsufficientFunds
	}

	w.balance -= amount
	w.updatedAt = now
	activityDate := now.UTC().Format("2006-01-02")

	entry := entity.NewCoinLedgerEntry(
		entryID,
		w.userID,
		-amount,
		w.balance,
		reason,
		copyOptionalString(refID),
		idempotencyKey,
		activityDate,
		nil,
		copyOptionalString(note),
		now,
	)

	var refStr string
	if refID != nil {
		refStr = *refID
	}
	ev := &event.CoinsSpent{
		UserID:       w.userID,
		Amount:       amount,
		BalanceAfter: w.balance,
		Reason:       string(reason),
		ReferenceID:  refStr,
		SpentAt:      now,
	}

	return entry, ev, nil
}

// AdminAdjust applies a signed FitCoins delta (positive or negative) with mandatory admin audit fields.
func (w *UserWallet) AdminAdjust(
	entryID string,
	delta int64,
	reason vo.CoinTransactionReason,
	adminID string,
	idempotencyKey string,
	note string,
	now time.Time,
) (*entity.CoinLedgerEntry, event.Event, error) {
	if delta == 0 || delta == math.MinInt64 {
		return nil, nil, ErrInvalidCoinAmount
	}
	if !reason.IsAdmin() {
		return nil, nil, ErrInvalidTransactionReason
	}
	if adminID == "" {
		return nil, nil, ErrEmptyAdminID
	}
	if idempotencyKey == "" || len(idempotencyKey) > MaxIdempotencyKeyLength {
		return nil, nil, ErrEmptyIdempotencyKey
	}
	if note == "" || len(note) > MaxNoteLength {
		return nil, nil, ErrEmptyAdjustmentNote
	}

	if delta > 0 {
		if w.balance > math.MaxInt64-delta {
			return nil, nil, ErrBalanceOverflow
		}
		w.balance += delta
	} else {
		deduction := -delta
		if w.balance < deduction {
			return nil, nil, ErrInsufficientFunds
		}
		w.balance -= deduction
	}

	w.updatedAt = now
	activityDate := now.UTC().Format("2006-01-02")
	adminCopy := adminID
	noteCopy := note

	entry := entity.NewCoinLedgerEntry(
		entryID,
		w.userID,
		delta,
		w.balance,
		reason,
		nil,
		idempotencyKey,
		activityDate,
		&adminCopy,
		&noteCopy,
		now,
	)

	var ev event.Event
	if delta > 0 {
		ev = &event.CoinsEarned{
			UserID:       w.userID,
			Amount:       delta,
			BalanceAfter: w.balance,
			Reason:       string(reason),
			ReferenceID:  adminID,
			EarnedAt:     now,
		}
	} else {
		ev = &event.CoinsSpent{
			UserID:       w.userID,
			Amount:       -delta,
			BalanceAfter: w.balance,
			Reason:       string(reason),
			ReferenceID:  adminID,
			SpentAt:      now,
		}
	}

	return entry, ev, nil
}

func copyOptionalDate(src *string) *string {
	if src == nil {
		return nil
	}
	v := *src
	return &v
}

func copyOptionalString(src *string) *string {
	if src == nil || *src == "" {
		return nil
	}
	v := *src
	return &v
}
