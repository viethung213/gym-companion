package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
)

// AwardCoinsHandler implements port.WalletRewarder to credit FitCoins atomically.
type AwardCoinsHandler struct {
	userWalletRepo repository.UserWalletRepository
	txManager      port.TransactionManager
	outboxWriter   port.OutboxWriter
}

// Compile-time interface check.
var _ port.WalletRewarder = (*AwardCoinsHandler)(nil)

// NewAwardCoinsHandler constructs a new AwardCoinsHandler.
func NewAwardCoinsHandler(
	userWalletRepo repository.UserWalletRepository,
	txManager port.TransactionManager,
	outboxWriter port.OutboxWriter,
) *AwardCoinsHandler {
	return &AwardCoinsHandler{
		userWalletRepo: userWalletRepo,
		txManager:      txManager,
		outboxWriter:   outboxWriter,
	}
}

// AwardCoins credits FitCoins to a user's wallet inside a pessimistic row lock.
// If the idempotency key was already processed or the daily cap for the reason is reached,
// it returns Skipped=true without failing the enclosing transaction.
//
//nolint:gocritic // hugeParam: input struct is an immutable value object
func (h *AwardCoinsHandler) AwardCoins(
	ctx context.Context,
	in port.AwardCoinsInput,
) (*port.AwardCoinsOutput, error) {
	if in.UserID == "" {
		return nil, aggregate.ErrEmptyUserID
	}
	if in.IdempotencyKey == "" {
		return nil, aggregate.ErrEmptyIdempotencyKey
	}

	occurredAt := in.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	var output port.AwardCoinsOutput

	err := h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		wallet, err := h.userWalletRepo.FindByIDForUpdate(txCtx, in.UserID)
		if err != nil {
			return fmt.Errorf("find user_wallet for update: %w", err)
		}
		if wallet == nil {
			wallet, err = aggregate.NewUserWallet(in.UserID)
			if err != nil {
				return fmt.Errorf("initialize user_wallet: %w", err)
			}
		}

		// Check idempotency key inside the row lock to eliminate TOCTOU race conditions.
		existing, err := h.userWalletRepo.FindLedgerEntryByIdempotencyKey(txCtx, in.UserID, in.IdempotencyKey)
		if err != nil {
			return fmt.Errorf("check idempotency key: %w", err)
		}
		if existing != nil {
			output = port.AwardCoinsOutput{
				TransactionID: existing.ID,
				BalanceAfter:  existing.BalanceAfter,
				Skipped:       true,
			}
			return nil
		}

		entry, ev, err := wallet.Award(
			uuid.NewString(),
			in.Amount,
			in.Reason,
			in.ReferenceID,
			in.IdempotencyKey,
			in.ActivityDate,
			occurredAt,
		)
		if err != nil {
			if errors.Is(err, aggregate.ErrDailyCapReached) {
				output = port.AwardCoinsOutput{
					BalanceAfter: wallet.Balance(),
					Skipped:      true,
				}
				return nil
			}
			return fmt.Errorf("award coins on wallet aggregate: %w", err)
		}

		if err := h.userWalletRepo.Save(txCtx, wallet); err != nil {
			return fmt.Errorf("save user_wallet: %w", err)
		}

		if err := h.userWalletRepo.SaveLedgerEntry(txCtx, entry); err != nil {
			return fmt.Errorf("save coin_ledger entry: %w", err)
		}

		if err := h.outboxWriter.WriteEvents(txCtx, "user_wallet", in.UserID, []any{ev}); err != nil {
			return fmt.Errorf("write user_wallet outbox event: %w", err)
		}

		output = port.AwardCoinsOutput{
			TransactionID: entry.ID,
			BalanceAfter:  wallet.Balance(),
			Skipped:       false,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &output, nil
}
