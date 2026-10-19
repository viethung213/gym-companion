package query_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/gamification/application/query"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
)

type mockUserWalletRepo struct {
	wallet *aggregate.UserWallet
	ledger []*entity.CoinLedgerEntry
	err    error
}

func (m *mockUserWalletRepo) FindByID(ctx context.Context, userID string) (*aggregate.UserWallet, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.wallet, nil
}

func (m *mockUserWalletRepo) FindByIDForUpdate(ctx context.Context, userID string) (*aggregate.UserWallet, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.wallet, nil
}

func (m *mockUserWalletRepo) Save(ctx context.Context, wallet *aggregate.UserWallet) error {
	m.wallet = wallet
	return nil
}

func (m *mockUserWalletRepo) SaveLedgerEntry(ctx context.Context, entry *entity.CoinLedgerEntry) error {
	m.ledger = append(m.ledger, entry)
	return nil
}

func TestGetAdminUserGamificationHandler_Handle(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("rejects empty user ID", func(t *testing.T) {
		handler := query.NewGetAdminUserGamificationHandler(
			&mockUserXpRepo{},
			&mockUserStreakRepo{},
			&mockUserWalletRepo{},
		)

		_, err := handler.Handle(ctx, query.GetAdminUserGamificationQuery{UserID: ""})
		assert.ErrorIs(t, err, aggregate.ErrEmptyUserID)
	})

	t.Run("rejects invalid UUID format", func(t *testing.T) {
		handler := query.NewGetAdminUserGamificationHandler(
			&mockUserXpRepo{},
			&mockUserStreakRepo{},
			&mockUserWalletRepo{},
		)

		_, err := handler.Handle(ctx, query.GetAdminUserGamificationQuery{UserID: "not-a-uuid"})
		assert.ErrorIs(t, err, aggregate.ErrInvalidUserID)
	})

	t.Run("returns default empty profile when user has no records", func(t *testing.T) {
		handler := query.NewGetAdminUserGamificationHandler(
			&mockUserXpRepo{},
			&mockUserStreakRepo{},
			&mockUserWalletRepo{},
		)

		uid := "33333333-3333-3333-3333-333333333333"
		res, err := handler.Handle(ctx, query.GetAdminUserGamificationQuery{UserID: uid})
		require.NoError(t, err)
		assert.Equal(t, uid, res.UserID)
		assert.Equal(t, int64(0), res.Xp)
		assert.Equal(t, int32(1), res.Level)
		assert.Equal(t, int32(0), res.CurrentStreak)
		assert.Equal(t, int64(0), res.CoinBalance)
	})

	t.Run("aggregates XP, Streak, and Wallet correctly", func(t *testing.T) {
		uid := "44444444-4444-4444-4444-444444444444"
		xp := aggregate.ReconstituteUserXp(uid, 2500, 7, nil, now)
		streak := aggregate.ReconstituteUserStreak(uid, 14, 20, "2026-10-17", now)
		wallet := aggregate.ReconstituteUserWallet(uid, 450, now, now)

		handler := query.NewGetAdminUserGamificationHandler(
			&mockUserXpRepo{userXp: xp},
			&mockUserStreakRepo{streak: streak},
			&mockUserWalletRepo{wallet: wallet},
		)

		res, err := handler.Handle(ctx, query.GetAdminUserGamificationQuery{UserID: uid})
		require.NoError(t, err)
		assert.Equal(t, uid, res.UserID)
		assert.Equal(t, int64(2500), res.Xp)
		assert.Equal(t, int32(7), res.Level)
		assert.Equal(t, int32(14), res.CurrentStreak)
		assert.Equal(t, int32(20), res.LongestStreak)
		assert.Equal(t, "2026-10-17", res.LastActiveDate)
		assert.Equal(t, int64(450), res.CoinBalance)
	})

	t.Run("propagates repository error when database fails", func(t *testing.T) {
		uid := "55555555-5555-5555-5555-555555555555"
		dbErr := errors.New("connection reset by peer")
		handler := query.NewGetAdminUserGamificationHandler(
			&mockUserXpRepo{},
			&mockUserStreakRepo{},
			&mockUserWalletRepo{err: dbErr},
		)

		_, err := handler.Handle(ctx, query.GetAdminUserGamificationQuery{UserID: uid})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "find user wallet")
	})
}
