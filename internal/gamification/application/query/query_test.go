package query_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/application/query"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
)

type mockUserXpRepo struct {
	userXp  *aggregate.UserXp
	history []*entity.XpHistoryEntry
}

func (m *mockUserXpRepo) FindByID(ctx context.Context, userID string) (*aggregate.UserXp, error) {
	return m.userXp, nil
}

func (m *mockUserXpRepo) FindByIDForUpdate(ctx context.Context, userID string) (*aggregate.UserXp, error) {
	return m.userXp, nil
}

func (m *mockUserXpRepo) Save(ctx context.Context, userXp *aggregate.UserXp) error {
	return nil
}

func (m *mockUserXpRepo) SaveHistory(ctx context.Context, history *entity.XpHistoryEntry) error {
	return nil
}

func (m *mockUserXpRepo) GetHistory(ctx context.Context, userID string, limit, offset int) ([]*entity.XpHistoryEntry, int64, error) {
	return m.history, int64(len(m.history)), nil
}

func (m *mockUserXpRepo) HasSession(ctx context.Context, userID, sessionID string) (bool, error) {
	return false, nil
}

type mockUserStreakRepo struct {
	streak  *aggregate.UserStreak
	history []*entity.StreakHistoryEntry
}

func (m *mockUserStreakRepo) FindByID(ctx context.Context, userID string) (*aggregate.UserStreak, error) {
	return m.streak, nil
}

func (m *mockUserStreakRepo) FindByIDForUpdate(ctx context.Context, userID string) (*aggregate.UserStreak, error) {
	return m.streak, nil
}

func (m *mockUserStreakRepo) Save(ctx context.Context, userStreak *aggregate.UserStreak) error {
	return nil
}

func (m *mockUserStreakRepo) SaveHistory(ctx context.Context, history *entity.StreakHistoryEntry) error {
	return nil
}

func (m *mockUserStreakRepo) GetHistory(ctx context.Context, userID string, limit, offset int) ([]*entity.StreakHistoryEntry, int64, error) {
	return m.history, int64(len(m.history)), nil
}

type mockLeaderboardRepo struct {
	topUsers []*entity.LeaderboardUser
	rank     int64
	entry    *entity.LeaderboardUser
}

func (m *mockLeaderboardRepo) GetGlobalTop(ctx context.Context, limit, offset int) ([]*entity.LeaderboardUser, int64, error) {
	return m.topUsers, int64(len(m.topUsers)), nil
}

func (m *mockLeaderboardRepo) GetUserRank(ctx context.Context, userID string) (int64, *entity.LeaderboardUser, error) {
	return m.rank, m.entry, nil
}

func (m *mockLeaderboardRepo) SaveSnapshot(ctx context.Context, snapshot *entity.UserSnapshot) error {
	return nil
}

func (m *mockLeaderboardRepo) GetSnapshot(ctx context.Context, userID string) (*entity.UserSnapshot, error) {
	return nil, nil
}

type mockBadgeRepo struct {
	badges []*aggregate.UserBadge
}

func (m *mockBadgeRepo) FindByUserID(ctx context.Context, userID string) ([]*aggregate.UserBadge, error) {
	return m.badges, nil
}

func (m *mockBadgeRepo) HasBadge(ctx context.Context, userID, badgeCode string) (bool, error) {
	return false, nil
}

func (m *mockBadgeRepo) Save(ctx context.Context, badge *aggregate.UserBadge) error {
	m.badges = append(m.badges, badge)
	return nil
}

func (m *mockBadgeRepo) SaveBatch(ctx context.Context, badges []*aggregate.UserBadge) error {
	m.badges = append(m.badges, badges...)
	return nil
}

func TestGetMyXpHandler(t *testing.T) {
	repo := &mockUserXpRepo{}
	handler := query.NewGetMyXpHandler(repo)

	// User not found -> returns defaults (0 XP, Level 1)
	res, err := handler.Handle(context.Background(), query.GetMyXpQuery{UserID: "u-new"})
	if err != nil || res.Xp != 0 || res.Level != 1 {
		t.Fatalf("unexpected defaults: %+v, err: %v", res, err)
	}

	// User exists
	now := time.Now().UTC()
	date := "2026-10-13"
	repo.userXp = aggregate.ReconstituteUserXp("u-1", 1250, 6, &date, now)

	res2, err := handler.Handle(context.Background(), query.GetMyXpQuery{UserID: "u-1"})
	if err != nil || res2.Xp != 1250 || res2.Level != 6 {
		t.Fatalf("unexpected user state: %+v, err: %v", res2, err)
	}
}

func TestGetStreakHandler(t *testing.T) {
	repo := &mockUserStreakRepo{}
	handler := query.NewGetStreakHandler(repo)

	// User not found -> defaults
	res, err := handler.Handle(context.Background(), query.GetStreakQuery{UserID: "u-new"})
	if err != nil || res.CurrentStreak != 0 || res.IsActiveToday {
		t.Fatalf("unexpected defaults: %+v, err: %v", res, err)
	}

	// User exists and active today
	now := time.Now().UTC()
	repo.streak = aggregate.ReconstituteUserStreak("u-1", 5, 10, "2026-10-13", now)

	res2, err := handler.Handle(context.Background(), query.GetStreakQuery{
		UserID:           "u-1",
		CurrentLocalDate: "2026-10-13",
	})
	if err != nil || res2.CurrentStreak != 5 || !res2.IsActiveToday {
		t.Fatalf("unexpected streak state: %+v, err: %v", res2, err)
	}

	// Active yesterday (delta = 1) -> streak still active, not active today yet
	res3, err := handler.Handle(context.Background(), query.GetStreakQuery{
		UserID:           "u-1",
		CurrentLocalDate: "2026-10-14",
	})
	if err != nil || res3.CurrentStreak != 5 || res3.IsActiveToday {
		t.Fatalf("unexpected streak state for delta=1: %+v, err: %v", res3, err)
	}

	// Inactive for 7 days (delta >= 2) -> virtual streak shows 0, preserves LongestStreak
	res4, err := handler.Handle(context.Background(), query.GetStreakQuery{
		UserID:           "u-1",
		CurrentLocalDate: "2026-10-20",
	})
	if err != nil || res4.CurrentStreak != 0 || res4.IsActiveToday || res4.LongestStreak != 10 {
		t.Fatalf("unexpected broken streak state for delta>=2: %+v, err: %v", res4, err)
	}
}

func TestGetGlobalLeaderboardHandler(t *testing.T) {
	repo := &mockLeaderboardRepo{
		topUsers: []*entity.LeaderboardUser{
			{Rank: 1, UserID: "u-top", DisplayName: "Top Gymer", Xp: 9999, Level: 15},
		},
		rank: 1,
		entry: &entity.LeaderboardUser{Rank: 1, UserID: "u-top", DisplayName: "Top Gymer", Xp: 9999, Level: 15},
	}
	handler := query.NewGetGlobalLeaderboardHandler(repo)

	res, err := handler.Handle(context.Background(), query.GetGlobalLeaderboardQuery{UserID: "u-top"})
	if err != nil || len(res.TopUsers) != 1 || res.MyRank != 1 {
		t.Fatalf("unexpected leaderboard result: %+v, err: %v", res, err)
	}
}

func TestGetMyBadgesHandler(t *testing.T) {
	badge := aggregate.ReconstituteUserBadge("b-1", "u-1", "FIRST_WORKOUT", "First Workout", "MILESTONE", time.Now().UTC(), nil)
	repo := &mockBadgeRepo{badges: []*aggregate.UserBadge{badge}}
	handler := query.NewGetMyBadgesHandler(repo)

	res, err := handler.Handle(context.Background(), query.GetMyBadgesQuery{UserID: "u-1"})
	if err != nil || len(res.Badges) != 1 {
		t.Fatalf("unexpected badges result: %+v, err: %v", res, err)
	}
}
