package test_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/gamification/test/testutil"
)

func TestIntegration_UserXp_SaveAndHasSession(t *testing.T) {
	_, repos := testutil.NewTestDB(t)
	ctx := context.Background()

	// 1. Create and Save UserXp
	userXp, err := aggregate.NewUserXp("user-100")
	require.NoError(t, err)

	events, err := userXp.EarnXp(150, "WORKOUT", time.Now().UTC())
	require.NoError(t, err)
	assert.NotEmpty(t, events)

	err = repos.UserXpRepo.Save(ctx, userXp)
	require.NoError(t, err)

	// 2. Fetch UserXp
	fetched, err := repos.UserXpRepo.FindByID(ctx, "user-100")
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, int64(150), fetched.Xp())
	assert.Equal(t, int32(2), fetched.Level())

	// 3. Save XpHistory (verifying column amount mapping - Bug #1)
	sessionID := "sess-xyz"
	history := entity.NewXpHistoryEntry(
		"hist-1",
		"user-100",
		150,
		"WORKOUT",
		&sessionID,
		"2026-10-13",
		time.Now().UTC(),
	)
	err = repos.UserXpRepo.SaveHistory(ctx, history)
	require.NoError(t, err, "SaveHistory must succeed without missing column error")

	// 4. Test HasSession (Idempotency - Bug #2)
	has, err := repos.UserXpRepo.HasSession(ctx, "user-100", "sess-xyz")
	require.NoError(t, err)
	assert.True(t, has, "HasSession must return true for saved session")

	hasNot, err := repos.UserXpRepo.HasSession(ctx, "user-100", "sess-unrecorded")
	require.NoError(t, err)
	assert.False(t, hasNot)

	// 5. GetHistory
	entries, total, err := repos.UserXpRepo.GetHistory(ctx, "user-100", 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, entries, 1)
	assert.Equal(t, int32(150), entries[0].DeltaXp)
}

func TestIntegration_UserXp_FindByIDForUpdate(t *testing.T) {
	_, repos := testutil.NewTestDB(t)
	ctx := context.Background()

	// Branch 1: Brand new user (fallback baseline seeding)
	newAgg, err := repos.UserXpRepo.FindByIDForUpdate(ctx, "brand-new-user-xp")
	require.NoError(t, err)
	require.NotNil(t, newAgg)
	assert.Equal(t, "brand-new-user-xp", newAgg.UserID())
	assert.Equal(t, int64(0), newAgg.Xp())
	assert.Equal(t, int32(1), newAgg.Level())

	// Branch 2: Existing user (fast-path SELECT ... FOR UPDATE)
	_, err = newAgg.EarnXp(300, "WORKOUT", time.Now().UTC())
	require.NoError(t, err)
	err = repos.UserXpRepo.Save(ctx, newAgg)
	require.NoError(t, err)

	existingAgg, err := repos.UserXpRepo.FindByIDForUpdate(ctx, "brand-new-user-xp")
	require.NoError(t, err)
	require.NotNil(t, existingAgg)
	assert.Equal(t, int64(300), existingAgg.Xp())
}

func TestIntegration_UserStreak_AndHistory(t *testing.T) {
	_, repos := testutil.NewTestDB(t)
	ctx := context.Background()

	streak, err := aggregate.NewUserStreak("user-200")
	require.NoError(t, err)

	now := time.Now().UTC()
	events, err := streak.RecordActivity("2026-10-13", "WORKOUT", now)
	require.NoError(t, err)
	assert.NotEmpty(t, events)

	err = repos.UserStreakRepo.Save(ctx, streak)
	require.NoError(t, err)

	fetched, err := repos.UserStreakRepo.FindByID(ctx, "user-200")
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, int32(1), fetched.CurrentStreak())

	hist := entity.NewStreakHistoryEntry("sh-1", "user-200", "2026-10-13", "WORKOUT", now)
	err = repos.UserStreakRepo.SaveHistory(ctx, hist)
	require.NoError(t, err)

	entries, total, err := repos.UserStreakRepo.GetHistory(ctx, "user-200", 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, entries, 1)
}

func TestIntegration_UserStreak_FindByIDForUpdate(t *testing.T) {
	_, repos := testutil.NewTestDB(t)
	ctx := context.Background()

	// Branch 1: Brand new user (fallback baseline seeding)
	newStreak, err := repos.UserStreakRepo.FindByIDForUpdate(ctx, "brand-new-user-streak")
	require.NoError(t, err)
	require.NotNil(t, newStreak)
	assert.Equal(t, "brand-new-user-streak", newStreak.UserID())
	assert.Equal(t, int32(0), newStreak.CurrentStreak())

	// Branch 2: Existing user (fast-path SELECT ... FOR UPDATE)
	_, err = newStreak.RecordActivity("2026-10-14", "WORKOUT", time.Now().UTC())
	require.NoError(t, err)
	err = repos.UserStreakRepo.Save(ctx, newStreak)
	require.NoError(t, err)

	existingStreak, err := repos.UserStreakRepo.FindByIDForUpdate(ctx, "brand-new-user-streak")
	require.NoError(t, err)
	require.NotNil(t, existingStreak)
	assert.Equal(t, int32(1), existingStreak.CurrentStreak())
}

func TestIntegration_Leaderboard_AndUserSnapshots(t *testing.T) {
	_, repos := testutil.NewTestDB(t)
	ctx := context.Background()

	now := time.Now().UTC()
	snap := entity.NewUserSnapshot("user-300", "Son Goku", "https://img/avatar.png", now)
	err := repos.LeaderboardRepo.SaveSnapshot(ctx, snap)
	require.NoError(t, err)

	fetchedSnap, err := repos.LeaderboardRepo.GetSnapshot(ctx, "user-300")
	require.NoError(t, err)
	require.NotNil(t, fetchedSnap)
	assert.Equal(t, "Son Goku", fetchedSnap.DisplayName)

	// Save XP
	userXp := aggregate.ReconstituteUserXp("user-300", 5000, 11, nil, now)
	err = repos.UserXpRepo.Save(ctx, userXp)
	require.NoError(t, err)

	// Top global query
	top, total, err := repos.LeaderboardRepo.GetGlobalTop(ctx, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, top, 1)
	assert.Equal(t, "user-300", top[0].UserID)
	assert.Equal(t, "Son Goku", top[0].DisplayName)
	assert.Equal(t, int64(5000), top[0].Xp)

	// User rank query
	rank, userStanding, err := repos.LeaderboardRepo.GetUserRank(ctx, "user-300")
	require.NoError(t, err)
	assert.Equal(t, int64(1), rank)
	assert.Equal(t, "Son Goku", userStanding.DisplayName)
}

func TestIntegration_BadgeRepository(t *testing.T) {
	_, repos := testutil.NewTestDB(t)
	ctx := context.Background()

	b, _, err := aggregate.UnlockBadge("b-1", "user-400", "STREAK_7", "Chiến Binh 7 Ngày", "STREAK", nil, time.Now().UTC())
	require.NoError(t, err)

	err = repos.BadgeRepo.Save(ctx, b)
	require.NoError(t, err)

	has, err := repos.BadgeRepo.HasBadge(ctx, "user-400", "STREAK_7")
	require.NoError(t, err)
	assert.True(t, has)

	badges, err := repos.BadgeRepo.FindByUserID(ctx, "user-400")
	require.NoError(t, err)
	require.Len(t, badges, 1)
	assert.Equal(t, "STREAK_7", badges[0].BadgeCode())
	assert.Equal(t, "Chiến Binh 7 Ngày", badges[0].BadgeName())
	assert.Equal(t, "STREAK", badges[0].Category())

	// Test SaveBatch
	b2, _, err := aggregate.UnlockBadge("b-2", "user-400", "PR_BREAKER", "Phá Vỡ Giới Hạn", "PR", nil, time.Now().UTC())
	require.NoError(t, err)
	b3, _, err := aggregate.UnlockBadge("b-3", "user-400", "LEVEL_10", "Tân Binh Đột Phá", "LEVEL", nil, time.Now().UTC())
	require.NoError(t, err)

	err = repos.BadgeRepo.SaveBatch(ctx, []*aggregate.UserBadge{b2, b3})
	require.NoError(t, err)

	allBadges, err := repos.BadgeRepo.FindByUserID(ctx, "user-400")
	require.NoError(t, err)
	assert.Len(t, allBadges, 3)
}

func TestIntegration_Outbox_ReleaseClaims(t *testing.T) {
	_, repos := testutil.NewTestDB(t)
	ctx := context.Background()

	record := &port.OutboxRecord{
		ID:            "outbox-test-1",
		EventID:       "evt-test-1",
		AggregateType: "UserXp",
		AggregateID:   "user-999",
		EventType:     "contracts.gamification.v1.XpEarned",
		Payload:       []byte(`{}`),
		PartitionKey:  "user-999",
	}
	err := repos.OutboxRepo.Save(ctx, record)
	require.NoError(t, err)

	// Claim
	claimed, err := repos.OutboxRepo.ClaimBatch(ctx, 10, 1*time.Minute)
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	// Release claims
	gormOutboxRepo, ok := repos.OutboxRepo.(*persistence.GormOutboxRepository)
	require.True(t, ok)

	err = gormOutboxRepo.ReleaseClaims(ctx, []string{"outbox-test-1"})
	require.NoError(t, err)

	// Verify it can be claimed again immediately
	reclaimed, err := repos.OutboxRepo.ClaimBatch(ctx, 10, 1*time.Minute)
	require.NoError(t, err)
	require.Len(t, reclaimed, 1)
	assert.Equal(t, "outbox-test-1", reclaimed[0].ID)
}
