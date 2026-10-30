package grpc_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/viethung213/gym-companion/internal/gamification/application/query"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
	gamificationGRPC "github.com/viethung213/gym-companion/internal/gamification/transport/grpc"
	gamificationv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/gamification/v1/message"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type mockUserXpRepo struct {
	findByIDFn func(ctx context.Context, userID string) (*aggregate.UserXp, error)
}

func (m *mockUserXpRepo) FindByID(ctx context.Context, userID string) (*aggregate.UserXp, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockUserXpRepo) FindByIDForUpdate(ctx context.Context, userID string) (*aggregate.UserXp, error) {
	return nil, nil
}

func (m *mockUserXpRepo) Save(ctx context.Context, userXp *aggregate.UserXp) error {
	return nil
}

func (m *mockUserXpRepo) SaveHistory(ctx context.Context, entry *entity.XpHistoryEntry) error {
	return nil
}

func (m *mockUserXpRepo) GetHistory(ctx context.Context, userID string, limit, offset int) ([]*entity.XpHistoryEntry, int64, error) {
	return nil, 0, nil
}

func (m *mockUserXpRepo) HasSession(ctx context.Context, userID, sessionID string) (bool, error) {
	return false, nil
}

type mockUserStreakRepo struct {
	findByIDFn func(ctx context.Context, userID string) (*aggregate.UserStreak, error)
}

func (m *mockUserStreakRepo) FindByID(ctx context.Context, userID string) (*aggregate.UserStreak, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockUserStreakRepo) FindByIDForUpdate(ctx context.Context, userID string) (*aggregate.UserStreak, error) {
	return nil, nil
}

func (m *mockUserStreakRepo) Save(ctx context.Context, userStreak *aggregate.UserStreak) error {
	return nil
}

func (m *mockUserStreakRepo) SaveHistory(ctx context.Context, entry *entity.StreakHistoryEntry) error {
	return nil
}

func (m *mockUserStreakRepo) GetHistory(ctx context.Context, userID string, limit, offset int) ([]*entity.StreakHistoryEntry, int64, error) {
	return nil, 0, nil
}

type mockLeaderboardRepo struct {
	getGlobalTopFn func(ctx context.Context, limit, offset int) ([]*entity.LeaderboardUser, int64, error)
	getUserRankFn  func(ctx context.Context, userID string) (int64, *entity.LeaderboardUser, error)
}

func (m *mockLeaderboardRepo) GetGlobalTop(ctx context.Context, limit, offset int) ([]*entity.LeaderboardUser, int64, error) {
	if m.getGlobalTopFn != nil {
		return m.getGlobalTopFn(ctx, limit, offset)
	}
	return nil, 0, nil
}

func (m *mockLeaderboardRepo) GetUserRank(ctx context.Context, userID string) (int64, *entity.LeaderboardUser, error) {
	if m.getUserRankFn != nil {
		return m.getUserRankFn(ctx, userID)
	}
	return 0, nil, nil
}

func (m *mockLeaderboardRepo) SaveSnapshot(ctx context.Context, snapshot *entity.UserSnapshot) error {
	return nil
}

func (m *mockLeaderboardRepo) GetSnapshot(ctx context.Context, userID string) (*entity.UserSnapshot, error) {
	return nil, nil
}

type mockBadgeRepo struct {
	findByUserIDFn func(ctx context.Context, userID string) ([]*aggregate.UserBadge, error)
}

func (m *mockBadgeRepo) FindByUserID(ctx context.Context, userID string) ([]*aggregate.UserBadge, error) {
	if m.findByUserIDFn != nil {
		return m.findByUserIDFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockBadgeRepo) HasBadge(ctx context.Context, userID, badgeCode string) (bool, error) {
	return false, nil
}

func (m *mockBadgeRepo) Save(ctx context.Context, badge *aggregate.UserBadge) error {
	return nil
}

func (m *mockBadgeRepo) SaveBatch(ctx context.Context, badges []*aggregate.UserBadge) error {
	return nil
}

func TestGRPCHandler_GetMyXp_Unauthenticated(t *testing.T) {
	handler := gamificationGRPC.NewGRPCHandler(
		query.NewGetMyXpHandler(&mockUserXpRepo{}),
		query.NewGetXpHistoryHandler(&mockUserXpRepo{}),
		query.NewGetStreakHandler(&mockUserStreakRepo{}),
		query.NewGetStreakHistoryHandler(&mockUserStreakRepo{}),
		query.NewGetGlobalLeaderboardHandler(&mockLeaderboardRepo{}),
		query.NewGetMyBadgesHandler(&mockBadgeRepo{}),
	)

	ctx := context.Background()
	_, err := handler.GetMyXp(ctx, connect.NewRequest(&gamificationv1message.GetMyXpRequest{}))
	assert.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	assert.True(t, ok)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestGRPCHandler_GetMyXp_Success(t *testing.T) {
	userXp := aggregate.ReconstituteUserXp("user-1", 100, 2, nil, time.Now())
	xpRepo := &mockUserXpRepo{
		findByIDFn: func(ctx context.Context, userID string) (*aggregate.UserXp, error) {
			return userXp, nil
		},
	}

	handler := gamificationGRPC.NewGRPCHandler(
		query.NewGetMyXpHandler(xpRepo),
		query.NewGetXpHistoryHandler(xpRepo),
		query.NewGetStreakHandler(&mockUserStreakRepo{}),
		query.NewGetStreakHistoryHandler(&mockUserStreakRepo{}),
		query.NewGetGlobalLeaderboardHandler(&mockLeaderboardRepo{}),
		query.NewGetMyBadgesHandler(&mockBadgeRepo{}),
	)

	ctx := context.WithValue(context.Background(), middleware.UserIDKey, "user-1")
	resp, err := handler.GetMyXp(ctx, connect.NewRequest(&gamificationv1message.GetMyXpRequest{}))
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "user-1", resp.Msg.XpSummary.UserId)
	assert.Equal(t, int64(100), resp.Msg.XpSummary.Xp)
	assert.Equal(t, int32(2), resp.Msg.XpSummary.Level)
}

func TestGRPCHandler_GetStreak_Success(t *testing.T) {
	today := time.Now().UTC().Format("2006-01-02")
	userStreak := aggregate.ReconstituteUserStreak("user-1", 5, 10, today, time.Now())
	streakRepo := &mockUserStreakRepo{
		findByIDFn: func(ctx context.Context, userID string) (*aggregate.UserStreak, error) {
			return userStreak, nil
		},
	}

	handler := gamificationGRPC.NewGRPCHandler(
		query.NewGetMyXpHandler(&mockUserXpRepo{}),
		query.NewGetXpHistoryHandler(&mockUserXpRepo{}),
		query.NewGetStreakHandler(streakRepo),
		query.NewGetStreakHistoryHandler(streakRepo),
		query.NewGetGlobalLeaderboardHandler(&mockLeaderboardRepo{}),
		query.NewGetMyBadgesHandler(&mockBadgeRepo{}),
	)

	ctx := context.WithValue(context.Background(), middleware.UserIDKey, "user-1")
	resp, err := handler.GetStreak(ctx, connect.NewRequest(&gamificationv1message.GetStreakRequest{}))
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int32(5), resp.Msg.StreakSummary.CurrentStreak)
	assert.True(t, resp.Msg.StreakSummary.IsActiveToday)
}

func TestGRPCHandler_GetGlobalLeaderboard_Success(t *testing.T) {
	topUsers := []*entity.LeaderboardUser{
		{
			Rank:        1,
			UserID:      "user-1",
			DisplayName: "Alice",
			AvatarURL:   "http://avatar",
			Xp:          500,
			Level:       3,
		},
	}
	leaderboardRepo := &mockLeaderboardRepo{
		getGlobalTopFn: func(ctx context.Context, limit, offset int) ([]*entity.LeaderboardUser, int64, error) {
			return topUsers, int64(1), nil
		},
		getUserRankFn: func(ctx context.Context, userID string) (int64, *entity.LeaderboardUser, error) {
			return int64(1), topUsers[0], nil
		},
	}

	handler := gamificationGRPC.NewGRPCHandler(
		query.NewGetMyXpHandler(&mockUserXpRepo{}),
		query.NewGetXpHistoryHandler(&mockUserXpRepo{}),
		query.NewGetStreakHandler(&mockUserStreakRepo{}),
		query.NewGetStreakHistoryHandler(&mockUserStreakRepo{}),
		query.NewGetGlobalLeaderboardHandler(leaderboardRepo),
		query.NewGetMyBadgesHandler(&mockBadgeRepo{}),
	)

	ctx := context.WithValue(context.Background(), middleware.UserIDKey, "user-1")
	resp, err := handler.GetGlobalLeaderboard(ctx, connect.NewRequest(&gamificationv1message.GetGlobalLeaderboardRequest{Limit: 50}))
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Msg.TopEntries, 1)
	assert.Equal(t, int32(1), resp.Msg.TopEntries[0].Rank)
	assert.Equal(t, int32(1), resp.Msg.MyStanding.Rank)
}

func TestGRPCHandler_GetMyBadges_Success(t *testing.T) {
	b := aggregate.ReconstituteUserBadge("badge-1", "user-1", "STREAK_7", "7-Day Streak", "STREAK", time.Now(), nil)
	badgeRepo := &mockBadgeRepo{
		findByUserIDFn: func(ctx context.Context, userID string) ([]*aggregate.UserBadge, error) {
			return []*aggregate.UserBadge{b}, nil
		},
	}

	handler := gamificationGRPC.NewGRPCHandler(
		query.NewGetMyXpHandler(&mockUserXpRepo{}),
		query.NewGetXpHistoryHandler(&mockUserXpRepo{}),
		query.NewGetStreakHandler(&mockUserStreakRepo{}),
		query.NewGetStreakHistoryHandler(&mockUserStreakRepo{}),
		query.NewGetGlobalLeaderboardHandler(&mockLeaderboardRepo{}),
		query.NewGetMyBadgesHandler(badgeRepo),
	)

	ctx := context.WithValue(context.Background(), middleware.UserIDKey, "user-1")
	resp, err := handler.GetMyBadges(ctx, connect.NewRequest(&gamificationv1message.GetMyBadgesRequest{}))
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int32(1), resp.Msg.TotalUnlocked)
	assert.Greater(t, resp.Msg.TotalAvailable, int32(1))
	var foundStreak7 bool
	for _, b := range resp.Msg.Badges {
		if b.BadgeCode == "STREAK_7" {
			foundStreak7 = true
			assert.True(t, b.IsUnlocked)
			assert.NotEmpty(t, b.Name)
		}
	}
	assert.True(t, foundStreak7)
}

type mockUserWalletRepo struct {
	findByIDFn   func(ctx context.Context, userID string) (*aggregate.UserWallet, error)
	listLedgerFn func(ctx context.Context, userID string, limit int, cursor string) ([]*entity.CoinLedgerEntry, string, error)
}

func (m *mockUserWalletRepo) FindByID(ctx context.Context, userID string) (*aggregate.UserWallet, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, userID)
	}
	w, _ := aggregate.NewUserWallet(userID)
	return w, nil
}

func (m *mockUserWalletRepo) FindByIDForUpdate(ctx context.Context, userID string) (*aggregate.UserWallet, error) {
	return m.FindByID(ctx, userID)
}

func (m *mockUserWalletRepo) Save(ctx context.Context, wallet *aggregate.UserWallet) error {
	return nil
}

func (m *mockUserWalletRepo) SaveLedgerEntry(ctx context.Context, entry *entity.CoinLedgerEntry) error {
	return nil
}

func (m *mockUserWalletRepo) SaveLedgerEntries(ctx context.Context, entries []*entity.CoinLedgerEntry) error {
	return nil
}

func (m *mockUserWalletRepo) FindLedgerEntryByIdempotencyKey(ctx context.Context, userID, key string) (*entity.CoinLedgerEntry, error) {
	return nil, nil
}

func (m *mockUserWalletRepo) ListLedgerEntries(ctx context.Context, userID string, limit int, cursor string) ([]*entity.CoinLedgerEntry, string, error) {
	if m.listLedgerFn != nil {
		return m.listLedgerFn(ctx, userID, limit, cursor)
	}
	return nil, "", nil
}

func TestGRPCHandler_GetWallet_SelfAndAdmin(t *testing.T) {
	walletRepo := &mockUserWalletRepo{
		findByIDFn: func(ctx context.Context, userID string) (*aggregate.UserWallet, error) {
			return aggregate.ReconstituteUserWallet(userID, 5, nil, nil, nil, time.Now().UTC()), nil
		},
	}

	handler := gamificationGRPC.NewHandler(
		query.NewGetMyXpHandler(&mockUserXpRepo{}),
		query.NewGetXpHistoryHandler(&mockUserXpRepo{}),
		query.NewGetStreakHandler(&mockUserStreakRepo{}),
		query.NewGetStreakHistoryHandler(&mockUserStreakRepo{}),
		query.NewGetGlobalLeaderboardHandler(&mockLeaderboardRepo{}),
		query.NewGetMyBadgesHandler(&mockBadgeRepo{}),
	).WithWalletHandlers(
		query.NewGetWalletHandler(walletRepo),
		query.NewListTransactionsHandler(walletRepo),
		nil,
		nil,
	)

	// Case 1: Normal user queries own wallet
	userCtx := context.WithValue(context.Background(), middleware.UserIDKey, "user-1")
	userCtx = context.WithValue(userCtx, middleware.UserRoleKey, "user")
	resp, err := handler.GetWallet(userCtx, connect.NewRequest(&gamificationv1message.GetWalletRequest{}))
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "user-1", resp.Msg.UserId)
	assert.Equal(t, int64(5), resp.Msg.Balance)

	// Case 2: Normal user queries someone else's wallet -> 403 Forbidden
	_, err = handler.GetWallet(userCtx, connect.NewRequest(&gamificationv1message.GetWalletRequest{UserId: "victim-user"}))
	assert.Error(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))

	// Case 3: Admin queries target user's wallet -> 200 OK
	adminCtx := context.WithValue(context.Background(), middleware.UserIDKey, "admin-1")
	adminCtx = context.WithValue(adminCtx, middleware.UserRoleKey, "admin")
	adminResp, err := handler.GetWallet(adminCtx, connect.NewRequest(&gamificationv1message.GetWalletRequest{UserId: "target-user"}))
	assert.NoError(t, err)
	assert.NotNil(t, adminResp)
	assert.Equal(t, "target-user", adminResp.Msg.UserId)
	assert.Equal(t, int64(5), adminResp.Msg.Balance)
}

func TestGRPCHandler_ListTransactions_SelfAndAdmin(t *testing.T) {
	now := time.Now().UTC()
	sampleEntries := []*entity.CoinLedgerEntry{
		{
			ID:           "entry-1",
			UserID:       "target-user",
			Amount:       50,
			BalanceAfter: 50,
			Reason:       vo.ReasonAdminAdjust,
			ActivityDate: "2026-10-25",
			CreatedAt:    now,
		},
	}
	walletRepo := &mockUserWalletRepo{
		listLedgerFn: func(ctx context.Context, userID string, limit int, cursor string) ([]*entity.CoinLedgerEntry, string, error) {
			if userID == "target-user" {
				return sampleEntries, "next-cur", nil
			}
			return nil, "", nil
		},
	}

	handler := gamificationGRPC.NewHandler(
		query.NewGetMyXpHandler(&mockUserXpRepo{}),
		query.NewGetXpHistoryHandler(&mockUserXpRepo{}),
		query.NewGetStreakHandler(&mockUserStreakRepo{}),
		query.NewGetStreakHistoryHandler(&mockUserStreakRepo{}),
		query.NewGetGlobalLeaderboardHandler(&mockLeaderboardRepo{}),
		query.NewGetMyBadgesHandler(&mockBadgeRepo{}),
	).WithWalletHandlers(
		query.NewGetWalletHandler(walletRepo),
		query.NewListTransactionsHandler(walletRepo),
		nil,
		nil,
	)

	// Case 1: Normal user queries someone else's transactions -> 403 Forbidden
	userCtx := context.WithValue(context.Background(), middleware.UserIDKey, "user-1")
	userCtx = context.WithValue(userCtx, middleware.UserRoleKey, "user")
	_, err := handler.ListTransactions(userCtx, connect.NewRequest(&gamificationv1message.ListTransactionsRequest{UserId: "target-user"}))
	assert.Error(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))

	// Case 2: Admin queries target user's transactions -> 200 OK
	adminCtx := context.WithValue(context.Background(), middleware.UserIDKey, "admin-1")
	adminCtx = context.WithValue(adminCtx, middleware.UserRoleKey, "admin")
	resp, err := handler.ListTransactions(adminCtx, connect.NewRequest(&gamificationv1message.ListTransactionsRequest{UserId: "target-user", Limit: 10}))
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Msg.Entries, 1)
	assert.Equal(t, "entry-1", resp.Msg.Entries[0].Id)
	assert.Equal(t, int64(50), resp.Msg.Entries[0].Amount)
	assert.Equal(t, "next-cur", resp.Msg.NextCursor)
}
