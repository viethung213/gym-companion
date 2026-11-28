package test_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
	"github.com/viethung213/gym-companion/internal/gamification/infrastructure/persistence"
	lbredis "github.com/viethung213/gym-companion/internal/gamification/infrastructure/redis"
	"gorm.io/gorm"
)

type paritySeed struct {
	userID      string
	xp          int64
	updatedAt   time.Time
	displayName string // "" = no snapshot
}

// parityDataset covers every tie-break rule: equal XP with different seconds, equal XP and second
// with different user IDs, a user without snapshot and a zero-XP user.
func parityDataset() []paritySeed {
	base := time.Date(2026, 11, 20, 6, 0, 0, 0, time.UTC)
	return []paritySeed{
		{"0b6c1f3e-0000-4000-8000-000000000001", 4200, base, "Alpha"},
		{"0b6c1f3e-0000-4000-8000-000000000002", 4200, base.Add(-time.Second), "Bravo"},
		{"0b6c1f3e-0000-4000-8000-000000000003", 4200, base, "Charlie"},
		{"0b6c1f3e-0000-4000-8000-000000000004", 9800, base.Add(time.Hour), "Delta"},
		{"0b6c1f3e-0000-4000-8000-000000000005", 150, base, ""},
		{"0b6c1f3e-0000-4000-8000-000000000006", 0, base.Add(-48 * time.Hour), "Echo"},
		{"0b6c1f3e-0000-4000-8000-000000000007", 2450, base.Add(2 * time.Minute), "Foxtrot"},
		{"0b6c1f3e-0000-4000-8000-000000000008", 2450, base.Add(time.Minute), "Golf"},
		{"0b6c1f3e-0000-4000-8000-000000000009", 2450, base.Add(time.Minute), "Hotel"},
		{"0b6c1f3e-0000-4000-8000-00000000000a", 51, base, "India"},
		{"0b6c1f3e-0000-4000-8000-00000000000b", 51, base, "Juliet"},
		{"0b6c1f3e-0000-4000-8000-00000000000c", 490050, base.Add(-24 * time.Hour), "Kilo"},
	}
}

func seedParityData(ctx context.Context, t *testing.T, db *gorm.DB, seeds []paritySeed) {
	t.Helper()
	xpRepo := persistence.NewGormUserXpRepository(db)
	snapshots := persistence.NewGormUserSnapshotRepository(db)
	for _, s := range seeds {
		if err := xpRepo.Save(ctx, aggregate.ReconstituteUserXp(s.userID, s.xp, vo.CalculateLevel(s.xp), nil, s.updatedAt)); err != nil {
			t.Fatalf("seed user_xp %s: %v", s.userID, err)
		}
		if s.displayName == "" {
			continue
		}
		if err := snapshots.SaveSnapshot(ctx, entity.NewUserSnapshot(s.userID, s.displayName, "https://cdn/"+s.displayName, s.updatedAt)); err != nil {
			t.Fatalf("seed snapshot %s: %v", s.userID, err)
		}
	}
}

// buildRedisLeaderboard rebuilds a Redis read model from db and returns its read repository.
func buildRedisLeaderboard(ctx context.Context, t *testing.T, db *gorm.DB) repository.LeaderboardRepository {
	t.Helper()
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return buildRedisLeaderboardWithClient(ctx, t, db, client)
}

func buildRedisLeaderboardWithClient(ctx context.Context, t *testing.T, db *gorm.DB, client *goredis.Client) repository.LeaderboardRepository {
	t.Helper()
	source := persistence.NewGormLeaderboardRepository(db)
	projection := lbredis.NewLeaderboardProjection(client, "parity-test")

	res, err := command.NewRebuildLeaderboardHandler(source, projection, 3).Handle(ctx, command.RebuildLeaderboardCommand{Reset: true})
	if err != nil || res.Skipped {
		t.Fatalf("rebuild = %+v, %v", res, err)
	}
	profiles := lbredis.NewCachedUserSnapshotRepository(
		client, persistence.NewGormUserSnapshotRepository(db), 24*time.Hour)
	return lbredis.NewLeaderboardRepository(client, projection, profiles, source)
}

func assertSameLeaderboardUser(t *testing.T, label string, want, got *entity.LeaderboardUser) {
	t.Helper()
	if want == nil || got == nil {
		if want != got {
			t.Errorf("%s: postgres %+v, redis %+v", label, want, got)
		}
		return
	}
	if want.Rank != got.Rank || want.UserID != got.UserID || want.DisplayName != got.DisplayName ||
		want.AvatarURL != got.AvatarURL || want.Xp != got.Xp || want.Level != got.Level ||
		!want.UpdatedAt.Truncate(time.Second).Equal(got.UpdatedAt) {
		t.Errorf("%s:\n postgres %+v\n redis    %+v", label, *want, *got)
	}
}

func assertLeaderboardParity(ctx context.Context, t *testing.T, pg, rd repository.LeaderboardRepository, userIDs []string) {
	t.Helper()
	for _, page := range []struct{ limit, offset int }{{50, 0}, {5, 3}, {100, 0}} {
		want, wantTotal, err := pg.GetGlobalTop(ctx, page.limit, page.offset)
		if err != nil {
			t.Fatalf("postgres GetGlobalTop%v: %v", page, err)
		}
		got, gotTotal, err := rd.GetGlobalTop(ctx, page.limit, page.offset)
		if err != nil {
			t.Fatalf("redis GetGlobalTop%v: %v", page, err)
		}
		if wantTotal != gotTotal || len(want) != len(got) {
			t.Fatalf("GetGlobalTop%v sizes: postgres %d/%d, redis %d/%d", page, len(want), wantTotal, len(got), gotTotal)
		}
		for i := range want {
			assertSameLeaderboardUser(t, "top row", want[i], got[i])
		}
	}

	for _, id := range append(userIDs, "00000000-0000-4000-8000-0000000000ff") {
		wantRank, want, err := pg.GetUserRank(ctx, id)
		if err != nil {
			t.Fatalf("postgres GetUserRank(%s): %v", id, err)
		}
		gotRank, got, err := rd.GetUserRank(ctx, id)
		if err != nil {
			t.Fatalf("redis GetUserRank(%s): %v", id, err)
		}
		if wantRank != gotRank {
			t.Errorf("GetUserRank(%s): postgres %d, redis %d", id, wantRank, gotRank)
		}
		assertSameLeaderboardUser(t, "rank "+id, want, got)
	}
}

func seedIDs(seeds []paritySeed) []string {
	ids := make([]string, len(seeds))
	for i, s := range seeds {
		ids[i] = s.userID
	}
	return ids
}
