package command_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
)

type mockUserWalletRepo struct {
	wallets map[string]*aggregate.UserWallet
	ledger  []*entity.CoinLedgerEntry
}

func newMockUserWalletRepo() *mockUserWalletRepo {
	return &mockUserWalletRepo{
		wallets: make(map[string]*aggregate.UserWallet),
		ledger:  make([]*entity.CoinLedgerEntry, 0),
	}
}

func (m *mockUserWalletRepo) FindByID(ctx context.Context, userID string) (*aggregate.UserWallet, error) {
	return m.wallets[userID], nil
}

func (m *mockUserWalletRepo) FindByIDForUpdate(ctx context.Context, userID string) (*aggregate.UserWallet, error) {
	return m.wallets[userID], nil
}

func (m *mockUserWalletRepo) Save(ctx context.Context, wallet *aggregate.UserWallet) error {
	m.wallets[wallet.UserID()] = wallet
	return nil
}

func (m *mockUserWalletRepo) SaveLedgerEntry(ctx context.Context, entry *entity.CoinLedgerEntry) error {
	m.ledger = append(m.ledger, entry)
	return nil
}

func TestAdminAdjustCoinsHandler_Handle(t *testing.T) {
	ctx := context.Background()
	walletRepo := newMockUserWalletRepo()
	txManager := &mockTxManager{}
	handler := command.NewAdminAdjustCoinsHandler(walletRepo, txManager)
	validUID := "11111111-1111-1111-1111-111111111111"

	t.Run("creates wallet and adds coins when wallet did not exist", func(t *testing.T) {
		res, err := handler.Handle(ctx, command.AdminAdjustCoinsCommand{
			UserID:  validUID,
			AdminID: "admin-uuid-1",
			Amount:  50,
			Reason:  "CS Ticket 99",
		})
		require.NoError(t, err)
		assert.Equal(t, validUID, res.UserID)
		assert.Equal(t, int64(50), res.BalanceAfter)

		savedWallet, err := walletRepo.FindByID(ctx, validUID)
		require.NoError(t, err)
		assert.Equal(t, int64(50), savedWallet.Balance())
		assert.Len(t, walletRepo.ledger, 1)
		assert.Equal(t, int64(50), walletRepo.ledger[0].Amount())
		assert.Equal(t, "CS Ticket 99", walletRepo.ledger[0].Reason())
		require.NotNil(t, walletRepo.ledger[0].AdminID())
		assert.Equal(t, "admin-uuid-1", *walletRepo.ledger[0].AdminID())
	})

	t.Run("deducts coins from existing wallet", func(t *testing.T) {
		res, err := handler.Handle(ctx, command.AdminAdjustCoinsCommand{
			UserID: validUID,
			Amount: -20,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(30), res.BalanceAfter)

		savedWallet, _ := walletRepo.FindByID(ctx, validUID)
		assert.Equal(t, int64(30), savedWallet.Balance())
	})

	t.Run("fails when deduction exceeds balance", func(t *testing.T) {
		_, err := handler.Handle(ctx, command.AdminAdjustCoinsCommand{
			UserID: validUID,
			Amount: -100,
		})
		assert.ErrorIs(t, err, aggregate.ErrInsufficientBalance)
	})

	t.Run("fails when amount is zero", func(t *testing.T) {
		_, err := handler.Handle(ctx, command.AdminAdjustCoinsCommand{
			UserID: validUID,
			Amount: 0,
		})
		assert.ErrorIs(t, err, aggregate.ErrZeroCoinAmount)
	})

	t.Run("fails when user ID is empty", func(t *testing.T) {
		_, err := handler.Handle(ctx, command.AdminAdjustCoinsCommand{
			UserID: "",
			Amount: 50,
		})
		assert.ErrorIs(t, err, aggregate.ErrEmptyUserID)
	})

	t.Run("fails when user ID is not valid UUID", func(t *testing.T) {
		_, err := handler.Handle(ctx, command.AdminAdjustCoinsCommand{
			UserID: "invalid-uuid",
			Amount: 50,
		})
		assert.ErrorIs(t, err, aggregate.ErrInvalidUserID)
	})
}

func TestAdminRepairStreakHandler_Handle(t *testing.T) {
	ctx := context.Background()
	streakRepo := newMockUserStreakRepo()
	txManager := &mockTxManager{}
	handler := command.NewAdminRepairStreakHandler(streakRepo, txManager)
	validUID := "22222222-2222-2222-2222-222222222222"

	t.Run("creates streak and sets value when streak did not exist", func(t *testing.T) {
		res, err := handler.Handle(ctx, command.AdminRepairStreakCommand{
			UserID:       validUID,
			TargetStreak: 12,
		})
		require.NoError(t, err)
		assert.Equal(t, validUID, res.UserID)
		assert.Equal(t, int32(12), res.CurrentStreak)
		assert.Equal(t, int32(12), res.LongestStreak)
	})

	t.Run("updates existing streak correctly", func(t *testing.T) {
		res, err := handler.Handle(ctx, command.AdminRepairStreakCommand{
			UserID:       validUID,
			TargetStreak: 25,
		})
		require.NoError(t, err)
		assert.Equal(t, int32(25), res.CurrentStreak)
		assert.Equal(t, int32(25), res.LongestStreak)
	})

	t.Run("rejects negative streak", func(t *testing.T) {
		_, err := handler.Handle(ctx, command.AdminRepairStreakCommand{
			UserID:       validUID,
			TargetStreak: -5,
		})
		assert.ErrorIs(t, err, aggregate.ErrNegativeStreak)
	})

	t.Run("rejects empty user ID", func(t *testing.T) {
		_, err := handler.Handle(ctx, command.AdminRepairStreakCommand{
			UserID:       "",
			TargetStreak: 5,
		})
		assert.ErrorIs(t, err, aggregate.ErrEmptyUserID)
	})

	t.Run("rejects invalid UUID", func(t *testing.T) {
		_, err := handler.Handle(ctx, command.AdminRepairStreakCommand{
			UserID:       "not-a-uuid",
			TargetStreak: 5,
		})
		assert.ErrorIs(t, err, aggregate.ErrInvalidUserID)
	})
}
