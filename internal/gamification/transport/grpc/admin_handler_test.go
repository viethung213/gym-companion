package grpc_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/application/query"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	gamificationGRPC "github.com/viethung213/gym-companion/internal/gamification/transport/grpc"
	gamificationv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/gamification/v1/message"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type mockAdminTxManager struct{}

func (m *mockAdminTxManager) WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return fn(ctx)
}

type mockAdminWalletRepo struct {
	wallet *aggregate.UserWallet
	ledger []*entity.CoinLedgerEntry
}

func (m *mockAdminWalletRepo) FindByID(ctx context.Context, userID string) (*aggregate.UserWallet, error) {
	return m.wallet, nil
}

func (m *mockAdminWalletRepo) FindByIDForUpdate(ctx context.Context, userID string) (*aggregate.UserWallet, error) {
	return m.wallet, nil
}

func (m *mockAdminWalletRepo) Save(ctx context.Context, wallet *aggregate.UserWallet) error {
	m.wallet = wallet
	return nil
}

func (m *mockAdminWalletRepo) SaveLedgerEntry(ctx context.Context, entry *entity.CoinLedgerEntry) error {
	m.ledger = append(m.ledger, entry)
	return nil
}

type mockAdminStreakRepo struct {
	streak *aggregate.UserStreak
}

func (m *mockAdminStreakRepo) FindByID(ctx context.Context, userID string) (*aggregate.UserStreak, error) {
	return m.streak, nil
}

func (m *mockAdminStreakRepo) FindByIDForUpdate(ctx context.Context, userID string) (*aggregate.UserStreak, error) {
	return m.streak, nil
}

func (m *mockAdminStreakRepo) Save(ctx context.Context, streak *aggregate.UserStreak) error {
	m.streak = streak
	return nil
}

func (m *mockAdminStreakRepo) SaveHistory(ctx context.Context, entry *entity.StreakHistoryEntry) error {
	return nil
}

func (m *mockAdminStreakRepo) GetHistory(ctx context.Context, userID string, limit, offset int) ([]*entity.StreakHistoryEntry, int64, error) {
	return nil, 0, nil
}

func setupAdminHandler(wallet *aggregate.UserWallet, streak *aggregate.UserStreak, xp *aggregate.UserXp) *gamificationGRPC.AdminHandler {
	txManager := &mockAdminTxManager{}
	walletRepo := &mockAdminWalletRepo{wallet: wallet}
	streakRepo := &mockAdminStreakRepo{streak: streak}
	xpRepo := &mockUserXpRepo{
		findByIDFn: func(ctx context.Context, userID string) (*aggregate.UserXp, error) {
			return xp, nil
		},
	}

	adjustCoinsHandler := command.NewAdminAdjustCoinsHandler(walletRepo, txManager)
	repairStreakHandler := command.NewAdminRepairStreakHandler(streakRepo, txManager)
	getAdminGamificationHandler := query.NewGetAdminUserGamificationHandler(xpRepo, streakRepo, walletRepo)

	return gamificationGRPC.NewAdminHandler(
		getAdminGamificationHandler,
		adjustCoinsHandler,
		repairStreakHandler,
	)
}

func contextWithAdminRole() context.Context {
	ctx := context.WithValue(context.Background(), middleware.UserIDKey, "admin-1")
	return context.WithValue(ctx, middleware.UserRoleKey, "Admin")
}

func contextWithRegularUserRole() context.Context {
	ctx := context.WithValue(context.Background(), middleware.UserIDKey, "user-regular")
	return context.WithValue(ctx, middleware.UserRoleKey, "user")
}

func TestAdminHandler_RBAC_PermissionDenied(t *testing.T) {
	handler := setupAdminHandler(nil, nil, nil)
	regularCtx := contextWithRegularUserRole()
	anonymousCtx := context.Background()

	t.Run("rejects regular user from GetAdminUserGamification", func(t *testing.T) {
		req := connect.NewRequest(&gamificationv1message.GetAdminUserGamificationRequest{UserId: "target-user"})
		_, err := handler.GetAdminUserGamification(regularCtx, req)
		require.Error(t, err)
		assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	})

	t.Run("rejects anonymous user from AdminAdjustCoins", func(t *testing.T) {
		req := connect.NewRequest(&gamificationv1message.AdminAdjustCoinsRequest{UserId: "target-user", Amount: 100})
		_, err := handler.AdminAdjustCoins(anonymousCtx, req)
		require.Error(t, err)
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})

	t.Run("rejects regular user from AdminRepairStreak", func(t *testing.T) {
		req := connect.NewRequest(&gamificationv1message.AdminRepairStreakRequest{UserId: "target-user", TargetStreak: 10})
		_, err := handler.AdminRepairStreak(regularCtx, req)
		require.Error(t, err)
		assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	})
}

func TestAdminHandler_Success_WithAdminRole(t *testing.T) {
	uid := "11111111-1111-1111-1111-111111111111"
	wallet, _ := aggregate.NewUserWallet(uid)
	streak, _ := aggregate.NewUserStreak(uid)
	xp, _ := aggregate.NewUserXp(uid)

	handler := setupAdminHandler(wallet, streak, xp)
	adminCtx := contextWithAdminRole()

	t.Run("GetAdminUserGamification returns summary", func(t *testing.T) {
		req := connect.NewRequest(&gamificationv1message.GetAdminUserGamificationRequest{UserId: uid})
		res, err := handler.GetAdminUserGamification(adminCtx, req)
		require.NoError(t, err)
		assert.Equal(t, uid, res.Msg.GetSummary().GetUserId())
		assert.Equal(t, int64(0), res.Msg.GetSummary().GetCoinBalance())
	})

	t.Run("AdminAdjustCoins modifies wallet balance with reason", func(t *testing.T) {
		req := connect.NewRequest(&gamificationv1message.AdminAdjustCoinsRequest{
			UserId: uid,
			Amount: 75,
			Reason: "CS Refund",
		})
		res, err := handler.AdminAdjustCoins(adminCtx, req)
		require.NoError(t, err)
		assert.Equal(t, uid, res.Msg.GetUserId())
		assert.Equal(t, int64(75), res.Msg.GetBalanceAfter())
	})

	t.Run("AdminRepairStreak updates user streak", func(t *testing.T) {
		req := connect.NewRequest(&gamificationv1message.AdminRepairStreakRequest{
			UserId:        uid,
			TargetStreak:  20,
			Reason:        "Dispute approved",
			EffectiveDate: "2026-10-10",
		})
		res, err := handler.AdminRepairStreak(adminCtx, req)
		require.NoError(t, err)
		assert.Equal(t, uid, res.Msg.GetUserId())
		assert.Equal(t, int32(20), res.Msg.GetCurrentStreak())
		assert.Equal(t, int32(20), res.Msg.GetLongestStreak())
	})

	t.Run("GetAdminUserGamification with empty userId returns CodeInvalidArgument", func(t *testing.T) {
		req := connect.NewRequest(&gamificationv1message.GetAdminUserGamificationRequest{UserId: ""})
		_, err := handler.GetAdminUserGamification(adminCtx, req)
		require.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})

	t.Run("GetAdminUserGamification with invalid UUID returns CodeInvalidArgument", func(t *testing.T) {
		req := connect.NewRequest(&gamificationv1message.GetAdminUserGamificationRequest{UserId: "not-a-uuid"})
		_, err := handler.GetAdminUserGamification(adminCtx, req)
		require.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})

	t.Run("AdminAdjustCoins with amount 0 returns CodeInvalidArgument", func(t *testing.T) {
		req := connect.NewRequest(&gamificationv1message.AdminAdjustCoinsRequest{UserId: uid, Amount: 0})
		_, err := handler.AdminAdjustCoins(adminCtx, req)
		require.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})

	t.Run("AdminAdjustCoins exceeding upper bound returns CodeInvalidArgument", func(t *testing.T) {
		req := connect.NewRequest(&gamificationv1message.AdminAdjustCoinsRequest{UserId: uid, Amount: 99_000_000})
		_, err := handler.AdminAdjustCoins(adminCtx, req)
		require.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})

	t.Run("AdminRepairStreak with negative streak returns CodeInvalidArgument", func(t *testing.T) {
		req := connect.NewRequest(&gamificationv1message.AdminRepairStreakRequest{UserId: uid, TargetStreak: -5})
		_, err := handler.AdminRepairStreak(adminCtx, req)
		require.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})

	t.Run("AdminRepairStreak with streak exceeding 3650 days returns CodeInvalidArgument", func(t *testing.T) {
		req := connect.NewRequest(&gamificationv1message.AdminRepairStreakRequest{UserId: uid, TargetStreak: 10000})
		_, err := handler.AdminRepairStreak(adminCtx, req)
		require.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})
}
