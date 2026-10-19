package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GormUserWalletRepository implements repository.UserWalletRepository.
type GormUserWalletRepository struct {
	db *gorm.DB
}

// Compile-time interface check.
var _ repository.UserWalletRepository = (*GormUserWalletRepository)(nil)

// NewGormUserWalletRepository constructs a new GormUserWalletRepository.
func NewGormUserWalletRepository(db *gorm.DB) *GormUserWalletRepository {
	return &GormUserWalletRepository{db: db}
}

// FindByID retrieves a UserWallet aggregate without locks.
func (r *GormUserWalletRepository) FindByID(ctx context.Context, userID string) (*aggregate.UserWallet, error) {
	db := GetDB(ctx, r.db)
	var model UserWalletModel
	err := db.Where("user_id = ?", userID).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find user_wallet by id: %w", err)
	}
	return walletModelToDomain(&model), nil
}

// FindByIDForUpdate retrieves a UserWallet aggregate with pessimistic row lock (SELECT ... FOR UPDATE).
// If the user wallet does not exist yet, it seeds a baseline row (0 balance) so FOR UPDATE always locks a physical row.
func (r *GormUserWalletRepository) FindByIDForUpdate(ctx context.Context, userID string) (*aggregate.UserWallet, error) {
	db := GetDB(ctx, r.db)

	now := time.Now().UTC()
	initModel := &UserWalletModel{
		UserID:    userID,
		Balance:   0,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoNothing: true,
	}).Create(initModel).Error; err != nil {
		return nil, fmt.Errorf("seed user_wallet baseline for update: %w", err)
	}

	var model UserWalletModel
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID).
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find user_wallet for update: %w", err)
	}
	return walletModelToDomain(&model), nil
}

// Save persists the UserWallet aggregate state using UPSERT.
func (r *GormUserWalletRepository) Save(ctx context.Context, wallet *aggregate.UserWallet) error {
	if wallet == nil {
		return errors.New("nil user_wallet aggregate")
	}

	model := walletDomainToModel(wallet)
	db := GetDB(ctx, r.db)
	err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		UpdateAll: true,
	}).Create(model).Error
	if err != nil {
		return fmt.Errorf("save user_wallet: %w", err)
	}
	return nil
}

// SaveLedgerEntry inserts an immutable transaction record into gamification.coin_ledger.
func (r *GormUserWalletRepository) SaveLedgerEntry(ctx context.Context, entry *entity.CoinLedgerEntry) error {
	if entry == nil {
		return errors.New("nil coin_ledger entry")
	}

	model := &CoinLedgerModel{
		ID:           entry.ID(),
		UserID:       entry.UserID(),
		AdminID:      entry.AdminID(),
		Amount:       entry.Amount(),
		BalanceAfter: entry.BalanceAfter(),
		Reason:       entry.Reason(),
		CreatedAt:    entry.CreatedAt(),
	}

	db := GetDB(ctx, r.db)
	if err := db.Create(model).Error; err != nil {
		return fmt.Errorf("insert coin_ledger entry: %w", err)
	}
	return nil
}

func walletModelToDomain(m *UserWalletModel) *aggregate.UserWallet {
	return aggregate.ReconstituteUserWallet(
		m.UserID,
		m.Balance,
		m.CreatedAt,
		m.UpdatedAt,
	)
}

func walletDomainToModel(w *aggregate.UserWallet) *UserWalletModel {
	return &UserWalletModel{
		UserID:    w.UserID(),
		Balance:   w.Balance(),
		CreatedAt: w.CreatedAt(),
		UpdatedAt: w.UpdatedAt(),
	}
}
