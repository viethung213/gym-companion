package command

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
)

// AdminAdjustCoinsCommand contains parameters to directly adjust a user's wallet coins.
type AdminAdjustCoinsCommand struct {
	UserID  string
	AdminID string
	Amount  int64
	Reason  string
}

// AdminAdjustCoinsResult returns the adjusted wallet balance.
type AdminAdjustCoinsResult struct {
	UserID       string
	BalanceAfter int64
}

// AdminAdjustCoinsHandler handles administrative coin adjustment.
type AdminAdjustCoinsHandler struct {
	walletRepo repository.UserWalletRepository
	txManager  port.TransactionManager
}

// NewAdminAdjustCoinsHandler constructs AdminAdjustCoinsHandler.
func NewAdminAdjustCoinsHandler(
	walletRepo repository.UserWalletRepository,
	txManager port.TransactionManager,
) *AdminAdjustCoinsHandler {
	return &AdminAdjustCoinsHandler{
		walletRepo: walletRepo,
		txManager:  txManager,
	}
}

// Handle executes the coin adjustment atomically.
func (h *AdminAdjustCoinsHandler) Handle(ctx context.Context, cmd AdminAdjustCoinsCommand) (*AdminAdjustCoinsResult, error) {
	if cmd.UserID == "" {
		return nil, aggregate.ErrEmptyUserID
	}
	if _, err := uuid.Parse(cmd.UserID); err != nil {
		return nil, aggregate.ErrInvalidUserID
	}
	if cmd.Amount == 0 {
		return nil, aggregate.ErrZeroCoinAmount
	}

	var result *AdminAdjustCoinsResult
	now := time.Now().UTC()

	reason := cmd.Reason
	if reason == "" {
		reason = "ADMIN_ADJUST"
	}
	var adminIDPtr *string
	if cmd.AdminID != "" {
		adminIDPtr = &cmd.AdminID
	}

	err := h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		wallet, err := h.walletRepo.FindByIDForUpdate(txCtx, cmd.UserID)
		if err != nil {
			return fmt.Errorf("lock user_wallet: %w", err)
		}
		if wallet == nil {
			wallet, err = aggregate.NewUserWallet(cmd.UserID)
			if err != nil {
				return err
			}
		}

		if err := wallet.AdjustBalance(cmd.Amount, now); err != nil {
			return err
		}

		if err := h.walletRepo.Save(txCtx, wallet); err != nil {
			return fmt.Errorf("save user_wallet: %w", err)
		}

		ledgerEntry := entity.NewCoinLedgerEntry(
			uuid.NewString(),
			wallet.UserID(),
			adminIDPtr,
			cmd.Amount,
			wallet.Balance(),
			reason,
			now,
		)
		if err := h.walletRepo.SaveLedgerEntry(txCtx, ledgerEntry); err != nil {
			return fmt.Errorf("save coin_ledger: %w", err)
		}

		result = &AdminAdjustCoinsResult{
			UserID:       wallet.UserID(),
			BalanceAfter: wallet.Balance(),
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}
