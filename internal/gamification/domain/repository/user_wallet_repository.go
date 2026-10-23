package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
)

// UserWalletRepository defines the storage operations for the UserWallet aggregate and immutable CoinLedger.
type UserWalletRepository interface {
	FindByID(ctx context.Context, userID string) (*aggregate.UserWallet, error)
	FindByIDForUpdate(ctx context.Context, userID string) (*aggregate.UserWallet, error)
	Save(ctx context.Context, wallet *aggregate.UserWallet) error
	SaveLedgerEntry(ctx context.Context, entry *entity.CoinLedgerEntry) error
	SaveLedgerEntries(ctx context.Context, entries []*entity.CoinLedgerEntry) error
	FindLedgerEntryByIdempotencyKey(ctx context.Context, userID, key string) (*entity.CoinLedgerEntry, error)
	ListLedgerEntries(ctx context.Context, userID string, limit int, cursor string) ([]*entity.CoinLedgerEntry, string, error)
}
