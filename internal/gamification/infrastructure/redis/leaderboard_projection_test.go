package redis_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	lbredis "github.com/viethung213/gym-companion/internal/gamification/infrastructure/redis"
)

func projectionScore(userID string, xp int64, at time.Time) entity.LeaderboardScore {
	return entity.LeaderboardScore{UserID: userID, Xp: xp, UpdatedAt: at}
}

func TestProjection_UpsertLT_NeverRegresses(t *testing.T) {
	client, _ := newTestRedis(t)
	ctx := context.Background()
	projection := lbredis.NewLeaderboardProjection(client, "node-a")
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

	if err := projection.UpsertScores(ctx, []entity.LeaderboardScore{projectionScore("u-1", 200, at.Add(time.Hour))}); err != nil {
		t.Fatalf("UpsertScores: %v", err)
	}
	best, _ := client.ZScore(ctx, leaderboardKey, "u-1").Result()

	// Replay of an older event (lower XP, earlier time) must not move the user down.
	if err := projection.UpsertScores(ctx, []entity.LeaderboardScore{projectionScore("u-1", 150, at)}); err != nil {
		t.Fatalf("UpsertScores replay: %v", err)
	}
	after, _ := client.ZScore(ctx, leaderboardKey, "u-1").Result()
	if after != best {
		t.Fatalf("score regressed from %v to %v", best, after)
	}

	if err := projection.UpsertScores(ctx, []entity.LeaderboardScore{projectionScore("u-1", 300, at.Add(2*time.Hour))}); err != nil {
		t.Fatalf("UpsertScores newer: %v", err)
	}
	newer, _ := client.ZScore(ctx, leaderboardKey, "u-1").Result()
	if newer >= best {
		t.Fatalf("higher xp did not improve score: %v -> %v", best, newer)
	}
}

func TestProjection_UpsertAddsNewMembersAndSkipsInvalid(t *testing.T) {
	client, _ := newTestRedis(t)
	ctx := context.Background()
	projection := lbredis.NewLeaderboardProjection(client, "node-a")
	at := time.Now().UTC()

	err := projection.UpsertScores(ctx, []entity.LeaderboardScore{
		projectionScore("u-1", 10, at),
		projectionScore("u-2", 0, at),
		projectionScore("", 99, at),
		projectionScore(lbredis.ReadySentinel, 99, at),
	})
	if err != nil {
		t.Fatalf("UpsertScores: %v", err)
	}
	members, _ := client.ZRange(ctx, leaderboardKey, 0, -1).Result()
	if len(members) != 2 || members[0] != "u-1" || members[1] != "u-2" {
		t.Fatalf("members = %v, want [u-1 u-2]", members)
	}

	if err := projection.UpsertScores(ctx, nil); err != nil {
		t.Fatalf("UpsertScores(nil): %v", err)
	}
}

func TestProjection_MarkReady_SentinelLast(t *testing.T) {
	client, _ := newTestRedis(t)
	ctx := context.Background()
	projection := lbredis.NewLeaderboardProjection(client, "node-a")
	at := time.Now().UTC()

	mustAcquire(t, projection)
	_ = projection.BeginRebuild(ctx)
	_ = projection.UpsertScores(ctx, []entity.LeaderboardScore{projectionScore("u-1", 0, at), projectionScore("u-2", 5000, at)})
	if err := projection.MarkReady(ctx); err != nil {
		t.Fatalf("MarkReady: %v", err)
	}

	rank, err := client.ZRank(ctx, leaderboardKey, lbredis.ReadySentinel).Result()
	if err != nil || rank != 2 {
		t.Fatalf("sentinel rank = %d, %v; want 2 (last)", rank, err)
	}
}

func TestProjection_Reset_RemovesEverything(t *testing.T) {
	client, _ := newTestRedis(t)
	ctx := context.Background()
	projection := lbredis.NewLeaderboardProjection(client, "node-a")

	mustAcquire(t, projection)
	_ = projection.BeginRebuild(ctx)
	_ = projection.UpsertScores(ctx, []entity.LeaderboardScore{projectionScore("u-1", 10, time.Now())})
	_ = projection.MarkReady(ctx)
	if err := projection.Reset(ctx); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if n, _ := client.Exists(ctx, leaderboardKey).Result(); n != 0 {
		t.Fatal("leaderboard key still exists after Reset")
	}
}

func TestProjection_Lock_ExclusiveAndCompareAndDelete(t *testing.T) {
	client, mr := newTestRedis(t)
	ctx := context.Background()
	nodeA := lbredis.NewLeaderboardProjection(client, "node-a")
	nodeB := lbredis.NewLeaderboardProjection(client, "node-b")

	if ok, err := nodeA.TryAcquireRebuildLock(ctx); err != nil || !ok {
		t.Fatalf("node A acquire = %v, %v; want true", ok, err)
	}
	if ttl := mr.TTL("gamification:leaderboard:xp:rebuild_lock"); ttl != 120*time.Second {
		t.Errorf("lock ttl = %v, want 120s", ttl)
	}
	if ok, err := nodeB.TryAcquireRebuildLock(ctx); err != nil || ok {
		t.Fatalf("node B acquire = %v, %v; want false", ok, err)
	}

	if err := nodeB.ReleaseRebuildLock(ctx); err != nil {
		t.Fatalf("node B release: %v", err)
	}
	if !mr.Exists("gamification:leaderboard:xp:rebuild_lock") {
		t.Fatal("node B released a lock it does not own")
	}

	if err := nodeA.ReleaseRebuildLock(ctx); err != nil {
		t.Fatalf("node A release: %v", err)
	}
	if ok, _ := nodeB.TryAcquireRebuildLock(ctx); !ok {
		t.Fatal("lock not free after owner released it")
	}
}

func mustAcquire(t *testing.T, p *lbredis.LeaderboardProjection) {
	t.Helper()
	if ok, err := p.TryAcquireRebuildLock(context.Background()); err != nil || !ok {
		t.Fatalf("TryAcquireRebuildLock = %v, %v; want true", ok, err)
	}
}

func TestProjection_MarkReady_RemovesBuildMarker(t *testing.T) {
	client, _ := newTestRedis(t)
	ctx := context.Background()
	p := lbredis.NewLeaderboardProjection(client, "node-a")

	mustAcquire(t, p)
	if err := p.BeginRebuild(ctx); err != nil {
		t.Fatalf("BeginRebuild: %v", err)
	}
	if err := p.MarkReady(ctx); err != nil {
		t.Fatalf("MarkReady: %v", err)
	}
	members, _ := client.ZRange(ctx, leaderboardKey, 0, -1).Result()
	if len(members) != 1 || members[0] != lbredis.ReadySentinel {
		t.Fatalf("members = %v, want only the ready sentinel", members)
	}
}

func TestProjection_MarkReady_RefusedAfterKeyLoss(t *testing.T) {
	client, _ := newTestRedis(t)
	ctx := context.Background()
	p := lbredis.NewLeaderboardProjection(client, "node-a")

	mustAcquire(t, p)
	_ = p.BeginRebuild(ctx)
	_ = client.Del(ctx, leaderboardKey).Err()
	_ = p.UpsertScores(ctx, []entity.LeaderboardScore{projectionScore("u-late", 10, time.Now())})

	if err := p.MarkReady(ctx); !errors.Is(err, lbredis.ErrRebuildInterrupted) {
		t.Fatalf("MarkReady err = %v, want ErrRebuildInterrupted", err)
	}
	if _, err := client.ZScore(ctx, leaderboardKey, lbredis.ReadySentinel).Result(); err == nil {
		t.Fatal("partial set was marked ready")
	}
}

func TestProjection_LockLost_MarkReadyAndExtendRefused(t *testing.T) {
	client, mr := newTestRedis(t)
	ctx := context.Background()
	nodeA := lbredis.NewLeaderboardProjection(client, "node-a")
	nodeB := lbredis.NewLeaderboardProjection(client, "node-b")

	mustAcquire(t, nodeA)
	_ = nodeA.BeginRebuild(ctx)
	mr.FastForward(121 * time.Second)
	mustAcquire(t, nodeB)

	if err := nodeA.ExtendRebuildLock(ctx); !errors.Is(err, lbredis.ErrRebuildInterrupted) {
		t.Fatalf("ExtendRebuildLock err = %v, want ErrRebuildInterrupted", err)
	}
	if err := nodeA.MarkReady(ctx); !errors.Is(err, lbredis.ErrRebuildInterrupted) {
		t.Fatalf("MarkReady err = %v, want ErrRebuildInterrupted after losing the lock", err)
	}
}

func TestProjection_ExtendRebuildLock_ResetsTTL(t *testing.T) {
	client, mr := newTestRedis(t)
	p := lbredis.NewLeaderboardProjection(client, "node-a")

	mustAcquire(t, p)
	mr.FastForward(90 * time.Second)
	if err := p.ExtendRebuildLock(context.Background()); err != nil {
		t.Fatalf("ExtendRebuildLock: %v", err)
	}
	if ttl := mr.TTL("gamification:leaderboard:xp:rebuild_lock"); ttl != 120*time.Second {
		t.Fatalf("lock ttl = %v, want 120s after extension", ttl)
	}
}
