package test_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	appService "github.com/viethung213/gym-companion/internal/gamification/application/service"
	infraEvent "github.com/viethung213/gym-companion/internal/gamification/infrastructure/event"
	"github.com/viethung213/gym-companion/internal/gamification/infrastructure/persistence"
	lbredis "github.com/viethung213/gym-companion/internal/gamification/infrastructure/redis"
	"github.com/viethung213/gym-companion/internal/gamification/test/testutil"
)

// Earning XP through the real ingestion pipeline must move the user on the Redis leaderboard as
// soon as the transaction commits, to the exact score PostgreSQL now holds.
func TestLeaderboardFlow_EarnedXpReachesRedisAfterCommit(t *testing.T) {
	ctx := context.Background()
	db, repos := testutil.NewTestDB(t)
	seeds := parityDataset()
	seedParityData(ctx, t, db, seeds)
	client := goredis.NewClient(&goredis.Options{Addr: miniredis.RunT(t).Addr()})
	t.Cleanup(func() { _ = client.Close() })
	board := buildRedisLeaderboardWithClient(ctx, t, db, client)

	writer := infraEvent.NewOutboxWriter(repos.OutboxRepo)
	coordinator := appService.NewActivityIngestionCoordinator(
		command.NewProcessWorkoutXpHandler(repos.UserXpRepo, repos.TxManager, writer),
		command.NewRecordStreakActivityHandler(repos.UserStreakRepo, repos.TxManager, writer),
		repos.BadgeRepo,
		repos.TxManager,
		writer,
	).WithLeaderboardProjector(command.NewProjectLeaderboardScoreHandler(
		lbredis.NewLeaderboardProjection(client, "flow-test")))

	const userID = "0b6c1f3e-0000-4000-8000-000000000006" // seeded with 0 XP, last place
	completedAt := time.Date(2026, 11, 21, 7, 30, 0, 0, time.UTC)
	if err := coordinator.IngestWorkoutSession(ctx, &command.ProcessWorkoutXpCommand{
		UserID:        userID,
		SessionID:     "flow-session-1",
		ActivityDate:  completedAt.Format("2006-01-02"),
		TotalSets:     10,
		TotalVolumeKg: 3000,
		CompletedAt:   completedAt,
	}); err != nil {
		t.Fatalf("IngestWorkoutSession: %v", err)
	}

	gormRepo := persistence.NewGormLeaderboardRepository(db)
	_, want, err := gormRepo.GetUserRank(ctx, userID)
	if err != nil || want == nil || want.Xp == 0 {
		t.Fatalf("postgres standing = %+v, %v; want XP earned", want, err)
	}
	_, got, err := board.GetUserRank(ctx, userID)
	if err != nil || got == nil || got.Xp != want.Xp {
		t.Fatalf("redis standing = %+v, %v; want XP %d from the committed transaction", got, err, want.Xp)
	}
	assertLeaderboardParity(ctx, t, gormRepo, board, seedIDs(seeds))
}

// A cold Redis (no leaderboard key yet, e.g. right after a restart) and an unreachable Redis must
// both serve PostgreSQL's answer; the cold case must also ask for a rebuild.
func TestLeaderboardFlow_ColdOrDownRedisFallsBackToPostgres(t *testing.T) {
	ctx := context.Background()
	db, _ := testutil.NewTestDB(t)
	seeds := parityDataset()
	seedParityData(ctx, t, db, seeds)
	gormRepo := persistence.NewGormLeaderboardRepository(db)

	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	projection := lbredis.NewLeaderboardProjection(client, "flow-test")
	profiles := lbredis.NewCachedUserSnapshotRepository(
		client, persistence.NewGormUserSnapshotRepository(db), 24*time.Hour)
	redisRepo := lbredis.NewLeaderboardRepository(client, projection, profiles, gormRepo)

	var rebuildRequests atomic.Int32
	board := lbredis.NewFallbackLeaderboardRepository(
		redisRepo, gormRepo, func() { rebuildRequests.Add(1) }, time.Minute)

	assertLeaderboardParity(ctx, t, gormRepo, board, seedIDs(seeds))
	if rebuildRequests.Load() == 0 {
		t.Error("cold Redis served from PostgreSQL without requesting a rebuild")
	}

	mr.Close()
	assertLeaderboardParity(ctx, t, gormRepo, board, seedIDs(seeds))
}
