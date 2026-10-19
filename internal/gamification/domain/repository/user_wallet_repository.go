package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
)

// UserWalletRepository defines storage operations for UserWallet aggregate and coin ledger entries.
type UserWalletRepository interface {
	FindByID(ctx context.Context, userID string) (*aggregate.UserWallet, error)
	FindByIDForUpdate(ctx context.Context, userID string) (*aggregate.UserWallet, error)
	Save(ctx context.Context, wallet *aggregate.UserWallet) error
	SaveLedgerEntry(ctx context.Context, entry *entity.CoinLedgerEntry) error
}
