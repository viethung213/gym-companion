package command

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
)

// SpendCoinsCommand carries parameters for deducting FitCoins from a user's wallet.
type SpendCoinsCommand struct {
	UserID         string
	Amount         int64
	Reason         vo.CoinTransactionReason
	ReferenceID    *string
	IdempotencyKey string
	Note           *string
	SpentAt        time.Time
}

// SpendCoinsResult contains the updated balance and immutable ledger transaction ID.
type SpendCoinsResult struct {
	BalanceAfter  int64
	TransactionID string
}

// SpendCoinsHandler coordinates deducting FitCoins under a pessimistic row lock.
type SpendCoinsHandler struct {
	userWalletRepo repository.UserWalletRepository
	txManager      port.TransactionManager
	outboxWriter   port.OutboxWriter
}

// NewSpendCoinsHandler constructs a new SpendCoinsHandler.
func NewSpendCoinsHandler(
	userWalletRepo repository.UserWalletRepository,
	txManager port.TransactionManager,
	outboxWriter port.OutboxWriter,
) *SpendCoinsHandler {
	return &SpendCoinsHandler{
		userWalletRepo: userWalletRepo,
		txManager:      txManager,
		outboxWriter:   outboxWriter,
	}
}

// Handle executes the spend operation atomically with idempotency replay protection.
//
//nolint:gocritic // hugeParam: command struct is an immutable value object
func (h *SpendCoinsHandler) Handle(
	ctx context.Context,
	cmd SpendCoinsCommand,
) (*SpendCoinsResult, error) {
	if cmd.UserID == "" {
		return nil, aggregate.ErrEmptyUserID
	}
	if cmd.IdempotencyKey == "" {
		return nil, aggregate.ErrEmptyIdempotencyKey
	}
	if cmd.Amount <= 0 {
		return nil, aggregate.ErrInvalidCoinAmount
	}
	if !cmd.Reason.IsSpend() {
		return nil, aggregate.ErrInvalidTransactionReason
	}

	spentAt := cmd.SpentAt
	if spentAt.IsZero() {
		spentAt = time.Now().UTC()
	}

	var result SpendCoinsResult

	err := h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		wallet, err := h.userWalletRepo.FindByIDForUpdate(txCtx, cmd.UserID)
		if err != nil {
			return fmt.Errorf("find user_wallet for update: %w", err)
		}
		if wallet == nil {
			wallet, err = aggregate.NewUserWallet(cmd.UserID)
			if err != nil {
				return fmt.Errorf("initialize user_wallet: %w", err)
			}
		}

		// Check idempotency key inside row lock to prevent TOCTOU double-spending.
		existing, err := h.userWalletRepo.FindLedgerEntryByIdempotencyKey(txCtx, cmd.UserID, cmd.IdempotencyKey)
		if err != nil {
			return fmt.Errorf("check idempotency key: %w", err)
		}
		if existing != nil {
			result = SpendCoinsResult{
				BalanceAfter:  existing.BalanceAfter,
				TransactionID: existing.ID,
			}
			return nil
		}

		entry, ev, err := wallet.Spend(
			uuid.NewString(),
			cmd.Amount,
			cmd.Reason,
			cmd.ReferenceID,
			cmd.IdempotencyKey,
			cmd.Note,
			spentAt,
		)
		if err != nil {
			return err
		}

		if err := h.userWalletRepo.Save(txCtx, wallet); err != nil {
			return fmt.Errorf("save user_wallet: %w", err)
		}

		if err := h.userWalletRepo.SaveLedgerEntry(txCtx, entry); err != nil {
			return fmt.Errorf("save coin_ledger entry: %w", err)
		}

		if err := h.outboxWriter.WriteEvents(txCtx, "user_wallet", cmd.UserID, []any{ev}); err != nil {
			return fmt.Errorf("write user_wallet outbox event: %w", err)
		}

		result = SpendCoinsResult{
			BalanceAfter:  wallet.Balance(),
			TransactionID: entry.ID,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &result, nil
}
