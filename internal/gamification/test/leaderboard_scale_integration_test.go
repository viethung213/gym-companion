//go:build integration

package test_test

import (
	"context"
	"errors"
	"math/rand/v2"
	"os"
	"slices"
	"strconv"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"github.com/viethung213/gym-companion/internal/gamification/infrastructure/persistence"
	lbredis "github.com/viethung213/gym-companion/internal/gamification/infrastructure/redis"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	scaleTopSamples  = 200
	scaleRankSamples = 500
	scaleParityRanks = 1000
)

var errRollbackScale = errors.New("rollback scale test transaction")

// TestLeaderboardScale_Postgres seeds LEADERBOARD_SCALE_USERS users (e.g. 100000) inside a
// rolled-back transaction, rebuilds the Redis read model (Redis DB 15) and reports rebuild time,
// ZSET memory and Redis vs PostgreSQL latency, while checking both return identical results.
//
//	LEADERBOARD_SCALE_USERS=100000 TEST_DATABASE_URL=postgres://... TEST_REDIS_ADDR=localhost:6379 \
//	  go test -tags integration -run Scale -v -timeout 20m ./internal/gamification/test/
func TestLeaderboardScale_Postgres(t *testing.T) {
	users, _ := strconv.Atoi(os.Getenv("LEADERBOARD_SCALE_USERS"))
	databaseURL, redisAddr := os.Getenv("TEST_DATABASE_URL"), os.Getenv("TEST_REDIS_ADDR")
	if users <= 0 || databaseURL == "" || redisAddr == "" {
		t.Skip("set LEADERBOARD_SCALE_USERS, TEST_DATABASE_URL and TEST_REDIS_ADDR")
	}

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	client := goredis.NewClient(&goredis.Options{Addr: redisAddr, DB: parityTestRedisDB})
	t.Cleanup(func() {
		_ = client.FlushDB(context.Background()).Err()
		_ = client.Close()
	})
	if err := client.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("flush redis test db: %v", err)
	}

	err = persistence.NewSQLTransactionManager(db).WithTransaction(context.Background(),
		func(txCtx context.Context) error {
			runLeaderboardScale(txCtx, t, db, client, users)
			return errRollbackScale
		})
	if !errors.Is(err, errRollbackScale) {
		t.Fatalf("scale transaction: %v", err)
	}
}

func runLeaderboardScale(ctx context.Context, t *testing.T, db *gorm.DB, client *goredis.Client, users int) {
	t.Helper()
	tx := persistence.GetDB(ctx, db)

	start := time.Now()
	seedScaleUsers(t, tx, users)
	var total int64
	tx.Raw("SELECT COUNT(*) FROM gamification.user_xp").Scan(&total)
	t.Logf("seed: %d users inserted in %v (table now %d rows)", users, time.Since(start).Round(time.Millisecond), total)

	gormRepo := persistence.NewGormLeaderboardRepository(db)
	snapshots := persistence.NewGormUserSnapshotRepository(db)
	projection := lbredis.NewLeaderboardProjection(client, "scale-test")

	start = time.Now()
	res, err := command.NewRebuildLeaderboardHandler(gormRepo, projection, 1000).
		Handle(ctx, command.RebuildLeaderboardCommand{Reset: true})
	if err != nil || res.Skipped || int64(res.Projected) != total {
		t.Fatalf("rebuild = %+v, %v; want %d projected", res, err, total)
	}
	t.Logf("rebuild: %d users projected in %v", res.Projected, time.Since(start).Round(time.Millisecond))

	memory, _ := client.MemoryUsage(ctx, "gamification:leaderboard:xp").Result()
	t.Logf("redis zset memory: %.1f MiB (%.0f bytes/user)", float64(memory)/(1<<20), float64(memory)/float64(total))

	profiles := lbredis.NewCachedUserSnapshotRepository(client, snapshots, 24*time.Hour)
	redisRepo := lbredis.NewLeaderboardRepository(client, projection, profiles, gormRepo)
	sampleIDs := sampleUserIDs(t, tx, scaleRankSamples)
	topIDs := topUserIDs(ctx, t, client)

	// The bulk insert is uncommitted, so user_snapshots has stale planner statistics here; this is
	// the worst case for PostgreSQL snapshot lookups that the profile cache must absorb.
	measure(t, "top50 postgres         ", scaleTopSamples, func(int) { mustTop(ctx, t, gormRepo) })
	measure(t, "snapshots IN (postgres)", scaleTopSamples, func(int) {
		if _, err := snapshots.GetSnapshots(ctx, topIDs); err != nil {
			t.Fatalf("GetSnapshots: %v", err)
		}
	})
	measure(t, "top50 redis+cache      ", scaleTopSamples, func(int) { mustTop(ctx, t, redisRepo) })
	logProfileCacheMemory(ctx, t, client, topIDs)

	if err := tx.Exec("ANALYZE gamification.user_snapshots").Error; err != nil {
		t.Fatalf("analyze user_snapshots: %v", err)
	}
	measure(t, "rank postgres          ", len(sampleIDs), func(i int) { mustRank(ctx, t, gormRepo, sampleIDs[i]) })
	measure(t, "rank redis (cold cache)", len(sampleIDs), func(i int) { mustRank(ctx, t, redisRepo, sampleIDs[i]) })
	measure(t, "rank redis (warm cache)", len(sampleIDs), func(i int) { mustRank(ctx, t, redisRepo, sampleIDs[i]) })

	assertScaleParity(ctx, t, gormRepo, redisRepo, sampleUserIDs(t, tx, scaleParityRanks))
}

// seedScaleUsers inserts users with a skewed XP distribution (many ties at low XP) and
// second-precision timestamps spread over ~2 years, plus a snapshot per user.
func seedScaleUsers(t *testing.T, tx *gorm.DB, users int) {
	t.Helper()
	err := tx.Exec(`
		INSERT INTO gamification.user_xp (user_id, xp, level, created_at, updated_at)
		SELECT gen_random_uuid(), x, LEAST(FLOOR(SQRT(x / 50.0))::int + 1, 100), ts, ts
		FROM (
			SELECT FLOOR(POWER(random(), 3) * 60000)::int AS x,
			       TIMESTAMPTZ '2025-01-01 00:00:00+00' + FLOOR(random() * 60000000) * INTERVAL '1 second' AS ts
			FROM generate_series(1, ?)
		) seed`, users).Error
	if err != nil {
		t.Fatalf("seed user_xp: %v", err)
	}
	err = tx.Exec(`
		INSERT INTO gamification.user_snapshots (user_id, display_name, avatar_url, updated_at)
		SELECT u.user_id, 'Bench ' || LEFT(u.user_id::text, 8), '', NOW()
		FROM gamification.user_xp u
		LEFT JOIN gamification.user_snapshots s ON s.user_id = u.user_id
		WHERE s.user_id IS NULL`).Error
	if err != nil {
		t.Fatalf("seed user_snapshots: %v", err)
	}
	if err := tx.Exec("ANALYZE gamification.user_xp").Error; err != nil {
		t.Fatalf("analyze: %v", err)
	}
}

func topUserIDs(ctx context.Context, t *testing.T, client *goredis.Client) []string {
	t.Helper()
	ids, err := client.ZRange(ctx, "gamification:leaderboard:xp", 0, 49).Result()
	if err != nil {
		t.Fatalf("zrange: %v", err)
	}
	return ids
}

func logProfileCacheMemory(ctx context.Context, t *testing.T, client *goredis.Client, ids []string) {
	t.Helper()
	var bytes int64
	for _, id := range ids {
		n, err := client.MemoryUsage(ctx, "gamification:profile:"+id).Result()
		if err != nil {
			t.Fatalf("memory usage %s: %v", id, err)
		}
		bytes += n
	}
	t.Logf("profile cache: %d profiles cached, %.0f bytes/profile", len(ids), float64(bytes)/float64(len(ids)))
}

func sampleUserIDs(t *testing.T, tx *gorm.DB, n int) []string {
	t.Helper()
	var ids []string
	if err := tx.Raw("SELECT user_id::text FROM gamification.user_xp ORDER BY random() LIMIT ?", n).
		Scan(&ids).Error; err != nil {
		t.Fatalf("sample users: %v", err)
	}
	return ids
}

func mustTop(ctx context.Context, t *testing.T, repo repository.LeaderboardRepository) {
	t.Helper()
	if top, _, err := repo.GetGlobalTop(ctx, 50, 0); err != nil || len(top) != 50 {
		t.Fatalf("GetGlobalTop = %d rows, %v", len(top), err)
	}
}

func mustRank(ctx context.Context, t *testing.T, repo repository.LeaderboardRepository, userID string) {
	t.Helper()
	if rank, _, err := repo.GetUserRank(ctx, userID); err != nil || rank == 0 {
		t.Fatalf("GetUserRank(%s) = %d, %v", userID, rank, err)
	}
}

func measure(t *testing.T, label string, n int, fn func(i int)) {
	t.Helper()
	durations := make([]time.Duration, n)
	for i := range n {
		start := time.Now()
		fn(i)
		durations[i] = time.Since(start)
	}
	slices.Sort(durations)
	pct := func(p float64) time.Duration { return durations[int(p*float64(n-1))].Round(10 * time.Microsecond) }
	t.Logf("%s n=%d  p50=%v  p95=%v  p99=%v  max=%v", label, n, pct(0.50), pct(0.95), pct(0.99), durations[n-1].Round(10*time.Microsecond))
}

func assertScaleParity(ctx context.Context, t *testing.T, pg, rd repository.LeaderboardRepository, ids []string) {
	t.Helper()
	for _, page := range []struct{ limit, offset int }{{100, 0}, {100, 5000}} {
		want, _, err := pg.GetGlobalTop(ctx, page.limit, page.offset)
		if err != nil {
			t.Fatalf("postgres top %v: %v", page, err)
		}
		got, _, err := rd.GetGlobalTop(ctx, page.limit, page.offset)
		if err != nil {
			t.Fatalf("redis top %v: %v", page, err)
		}
		if len(want) != len(got) {
			t.Fatalf("top %v sizes: postgres %d, redis %d", page, len(want), len(got))
		}
		for i := range want {
			assertSameLeaderboardUser(t, "top row", want[i], got[i])
		}
	}

	mismatches := 0
	for _, id := range shuffled(ids) {
		wantRank, _, err := pg.GetUserRank(ctx, id)
		if err != nil {
			t.Fatalf("postgres rank %s: %v", id, err)
		}
		gotRank, _, err := rd.GetUserRank(ctx, id)
		if err != nil {
			t.Fatalf("redis rank %s: %v", id, err)
		}
		if wantRank != gotRank {
			mismatches++
			t.Errorf("rank %s: postgres %d, redis %d", id, wantRank, gotRank)
		}
	}
	t.Logf("parity: top 1-100 and 5001-5100 identical; %d/%d sampled ranks identical", len(ids)-mismatches, len(ids))
}

func shuffled(ids []string) []string {
	out := slices.Clone(ids)
	rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}
