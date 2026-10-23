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

// AdminAdjustCoinsCommand carries parameters for an administrative wallet balance adjustment.
type AdminAdjustCoinsCommand struct {
	AdminID        string
	TargetUserID   string
	Amount         int64
	Reason         vo.CoinTransactionReason
	IdempotencyKey string
	Note           string
	AdjustedAt     time.Time
}

// AdminAdjustCoinsResult contains the updated balance and immutable ledger transaction ID.
type AdminAdjustCoinsResult struct {
	BalanceAfter  int64
	TransactionID string
}

// AdminAdjustCoinsHandler coordinates administrative FitCoins adjustments under a pessimistic row lock.
type AdminAdjustCoinsHandler struct {
	userWalletRepo repository.UserWalletRepository
	txManager      port.TransactionManager
	outboxWriter   port.OutboxWriter
}

// NewAdminAdjustCoinsHandler constructs a new AdminAdjustCoinsHandler.
func NewAdminAdjustCoinsHandler(
	userWalletRepo repository.UserWalletRepository,
	txManager port.TransactionManager,
	outboxWriter port.OutboxWriter,
) *AdminAdjustCoinsHandler {
	return &AdminAdjustCoinsHandler{
		userWalletRepo: userWalletRepo,
		txManager:      txManager,
		outboxWriter:   outboxWriter,
	}
}

// Handle executes the administrative wallet adjustment atomically with idempotency replay protection.
//
//nolint:gocritic // hugeParam: command struct is an immutable value object
func (h *AdminAdjustCoinsHandler) Handle(
	ctx context.Context,
	cmd AdminAdjustCoinsCommand,
) (*AdminAdjustCoinsResult, error) {
	if cmd.TargetUserID == "" {
		return nil, aggregate.ErrEmptyUserID
	}
	if _, err := uuid.Parse(cmd.TargetUserID); err != nil {
		return nil, aggregate.ErrEmptyUserID
	}
	if cmd.AdminID == "" {
		return nil, aggregate.ErrEmptyAdminID
	}
	if cmd.IdempotencyKey == "" || len(cmd.IdempotencyKey) > aggregate.MaxIdempotencyKeyLength {
		return nil, aggregate.ErrEmptyIdempotencyKey
	}
	if cmd.Note == "" || len(cmd.Note) > aggregate.MaxNoteLength {
		return nil, aggregate.ErrEmptyAdjustmentNote
	}

	adjustedAt := cmd.AdjustedAt
	if adjustedAt.IsZero() {
		adjustedAt = time.Now().UTC()
	}

	var result AdminAdjustCoinsResult

	err := h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		wallet, err := h.userWalletRepo.FindByIDForUpdate(txCtx, cmd.TargetUserID)
		if err != nil {
			return fmt.Errorf("find user_wallet for update: %w", err)
		}
		if wallet == nil {
			wallet, err = aggregate.NewUserWallet(cmd.TargetUserID)
			if err != nil {
				return fmt.Errorf("initialize user_wallet: %w", err)
			}
		}

		// Check idempotency key inside row lock to prevent duplicate admin adjustments.
		existing, err := h.userWalletRepo.FindLedgerEntryByIdempotencyKey(txCtx, cmd.TargetUserID, cmd.IdempotencyKey)
		if err != nil {
			return fmt.Errorf("check idempotency key: %w", err)
		}
		if existing != nil {
			result = AdminAdjustCoinsResult{
				BalanceAfter:  existing.BalanceAfter,
				TransactionID: existing.ID,
			}
			return nil
		}

		entry, ev, err := wallet.AdminAdjust(
			uuid.NewString(),
			cmd.Amount,
			cmd.Reason,
			cmd.AdminID,
			cmd.IdempotencyKey,
			cmd.Note,
			adjustedAt,
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

		if err := h.outboxWriter.WriteEvents(txCtx, "user_wallet", cmd.TargetUserID, []any{ev}); err != nil {
			return fmt.Errorf("write user_wallet outbox event: %w", err)
		}

		result = AdminAdjustCoinsResult{
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
