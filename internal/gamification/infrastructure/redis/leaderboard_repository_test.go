package redis_test

import (
	"context"
	"errors"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
	lbredis "github.com/viethung213/gym-companion/internal/gamification/infrastructure/redis"
)

type fakeSnapshotRepo struct {
	snapshots map[string]*entity.UserSnapshot
	err       error
}

func (f *fakeSnapshotRepo) SaveSnapshot(_ context.Context, s *entity.UserSnapshot) error {
	f.snapshots[s.UserID] = s
	return nil
}

func (f *fakeSnapshotRepo) GetSnapshot(_ context.Context, userID string) (*entity.UserSnapshot, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.snapshots[userID], nil
}

func (f *fakeSnapshotRepo) GetSnapshots(_ context.Context, userIDs []string) (map[string]*entity.UserSnapshot, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make(map[string]*entity.UserSnapshot, len(userIDs))
	for _, id := range userIDs {
		if s, ok := f.snapshots[id]; ok {
			out[id] = s
		}
	}
	return out, nil
}

type fakeScoreSource struct {
	scores map[string]entity.LeaderboardScore
	onGet  func()
}

func (f *fakeScoreSource) ListScores(context.Context, string, int) ([]entity.LeaderboardScore, error) {
	return nil, errors.New("not used")
}

func (f *fakeScoreSource) GetScore(_ context.Context, userID string) (*entity.LeaderboardScore, error) {
	if f.onGet != nil {
		f.onGet()
	}
	s, ok := f.scores[userID]
	if !ok {
		return nil, nil
	}
	return &s, nil
}

type redisLeaderboardFixture struct {
	client     *goredis.Client
	projection *lbredis.LeaderboardProjection
	snapshots  *fakeSnapshotRepo
	source     *fakeScoreSource
	repo       *lbredis.LeaderboardRepository
}

func newRedisLeaderboardFixture(t *testing.T) *redisLeaderboardFixture {
	t.Helper()
	client, _ := newTestRedis(t)
	f := &redisLeaderboardFixture{
		client:     client,
		projection: lbredis.NewLeaderboardProjection(client, "node-a"),
		snapshots:  &fakeSnapshotRepo{snapshots: map[string]*entity.UserSnapshot{}},
		source:     &fakeScoreSource{scores: map[string]entity.LeaderboardScore{}},
	}
	f.repo = lbredis.NewLeaderboardRepository(client, f.projection, f.snapshots, f.source)
	return f
}

func (f *redisLeaderboardFixture) seed(t *testing.T, ready bool, scores ...entity.LeaderboardScore) {
	t.Helper()
	ctx := context.Background()
	if err := f.projection.UpsertScores(ctx, scores); err != nil {
		t.Fatalf("seed UpsertScores: %v", err)
	}
	if !ready {
		return
	}
	if ok, err := f.projection.TryAcquireRebuildLock(ctx); err != nil || !ok {
		t.Fatalf("seed lock = %v, %v", ok, err)
	}
	defer func() { _ = f.projection.ReleaseRebuildLock(ctx) }()
	if err := f.projection.BeginRebuild(ctx); err != nil {
		t.Fatalf("seed BeginRebuild: %v", err)
	}
	if err := f.projection.MarkReady(ctx); err != nil {
		t.Fatalf("seed MarkReady: %v", err)
	}
}

func baseTime() time.Time { return time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC) }

func TestRedisLeaderboard_Top_OrderDecodeAndDefaults(t *testing.T) {
	f := newRedisLeaderboardFixture(t)
	ctx := context.Background()
	f.snapshots.snapshots["u-top"] = entity.NewUserSnapshot("u-top", "Top Gymer", "top.png", baseTime())
	f.seed(t, true,
		projectionScore("u-top", 5000, baseTime().Add(250*time.Millisecond)),
		projectionScore("u-mid", 2450, baseTime()),
		projectionScore("u-low", 10, baseTime()),
	)

	top, total, err := f.repo.GetGlobalTop(ctx, 50, 0)
	if err != nil {
		t.Fatalf("GetGlobalTop: %v", err)
	}
	if total != 3 || len(top) != 3 {
		t.Fatalf("got %d rows (total %d), want 3", len(top), total)
	}
	first := top[0]
	if first.Rank != 1 || first.UserID != "u-top" || first.Xp != 5000 || first.Level != vo.CalculateLevel(5000) ||
		first.DisplayName != "Top Gymer" || first.AvatarURL != "top.png" || !first.UpdatedAt.Equal(baseTime()) {
		t.Errorf("first row = %+v", first)
	}
	if top[1].UserID != "u-mid" || top[1].Rank != 2 || top[1].DisplayName != entity.DefaultDisplayName || top[1].AvatarURL != "" {
		t.Errorf("second row = %+v, want u-mid rank 2 with defaults", top[1])
	}

	page, _, err := f.repo.GetGlobalTop(ctx, 1, 1)
	if err != nil || len(page) != 1 || page[0].UserID != "u-mid" || page[0].Rank != 2 {
		t.Errorf("offset page = %+v, %v; want u-mid rank 2", page, err)
	}
}

func TestRedisLeaderboard_Top_FiltersSentinel(t *testing.T) {
	f := newRedisLeaderboardFixture(t)
	f.seed(t, true, projectionScore("u-1", 100, baseTime()), projectionScore("u-2", 50, baseTime()))

	top, total, err := f.repo.GetGlobalTop(context.Background(), 50, 0)
	if err != nil {
		t.Fatalf("GetGlobalTop: %v", err)
	}
	if total != 2 || len(top) != 2 || top[0].Rank != 1 || top[1].Rank != 2 {
		t.Fatalf("top = %+v (total %d), want 2 ranked users without sentinel", top, total)
	}
	for _, u := range top {
		if u.UserID == lbredis.ReadySentinel {
			t.Fatal("sentinel leaked into the leaderboard")
		}
	}
}

func TestRedisLeaderboard_Top_UserIDAscOnExactTie(t *testing.T) {
	f := newRedisLeaderboardFixture(t)
	f.seed(t, true,
		projectionScore("bbbb", 100, baseTime()),
		projectionScore("aaaa", 100, baseTime()),
		projectionScore("cccc", 100, baseTime().Add(-time.Second)),
	)

	top, _, err := f.repo.GetGlobalTop(context.Background(), 50, 0)
	if err != nil {
		t.Fatalf("GetGlobalTop: %v", err)
	}
	got := []string{top[0].UserID, top[1].UserID, top[2].UserID}
	want := []string{"cccc", "aaaa", "bbbb"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestRedisLeaderboard_NotReady(t *testing.T) {
	f := newRedisLeaderboardFixture(t)
	ctx := context.Background()
	f.seed(t, false, projectionScore("u-1", 100, baseTime()))

	if _, _, err := f.repo.GetGlobalTop(ctx, 50, 0); !errors.Is(err, lbredis.ErrLeaderboardNotReady) {
		t.Errorf("GetGlobalTop err = %v, want ErrLeaderboardNotReady", err)
	}
	if _, _, err := f.repo.GetUserRank(ctx, "u-1"); !errors.Is(err, lbredis.ErrLeaderboardNotReady) {
		t.Errorf("GetUserRank err = %v, want ErrLeaderboardNotReady", err)
	}
}

func TestRedisLeaderboard_KeyDeleted_NotReady(t *testing.T) {
	f := newRedisLeaderboardFixture(t)
	ctx := context.Background()
	f.seed(t, true, projectionScore("u-1", 100, baseTime()))

	// Simulate eviction, then a consumer re-creating the key with a single member.
	if err := f.client.Del(ctx, leaderboardKey).Err(); err != nil {
		t.Fatalf("Del: %v", err)
	}
	f.seed(t, false, projectionScore("u-2", 300, baseTime()))

	if _, _, err := f.repo.GetGlobalTop(ctx, 50, 0); !errors.Is(err, lbredis.ErrLeaderboardNotReady) {
		t.Fatalf("GetGlobalTop err = %v, want ErrLeaderboardNotReady after key loss", err)
	}
}

func TestRedisLeaderboard_Rank_Found(t *testing.T) {
	f := newRedisLeaderboardFixture(t)
	f.snapshots.snapshots["u-2"] = entity.NewUserSnapshot("u-2", "Second", "2.png", baseTime())
	f.seed(t, true, projectionScore("u-1", 900, baseTime()), projectionScore("u-2", 400, baseTime()))

	rank, entry, err := f.repo.GetUserRank(context.Background(), "u-2")
	if err != nil {
		t.Fatalf("GetUserRank: %v", err)
	}
	if rank != 2 || entry == nil || entry.Rank != 2 || entry.Xp != 400 || entry.DisplayName != "Second" ||
		entry.Level != vo.CalculateLevel(400) || !entry.UpdatedAt.Equal(baseTime()) {
		t.Errorf("GetUserRank = %d, %+v", rank, entry)
	}
}

func TestRedisLeaderboard_Rank_ReadRepair(t *testing.T) {
	f := newRedisLeaderboardFixture(t)
	ctx := context.Background()
	f.seed(t, true, projectionScore("u-1", 900, baseTime()), projectionScore("u-3", 0, baseTime()))
	f.source.scores["u-new"] = projectionScore("u-new", 0, baseTime().Add(-time.Hour))

	rank, entry, err := f.repo.GetUserRank(ctx, "u-new")
	if err != nil {
		t.Fatalf("GetUserRank: %v", err)
	}
	if rank != 2 || entry == nil || entry.UserID != "u-new" || entry.Xp != 0 || entry.DisplayName != entity.DefaultDisplayName {
		t.Fatalf("read-repaired rank = %d, %+v; want rank 2", rank, entry)
	}
	if _, err := f.client.ZScore(ctx, leaderboardKey, "u-new").Result(); err != nil {
		t.Fatalf("read-repair did not write u-new into the ZSET: %v", err)
	}
}

func TestRedisLeaderboard_Rank_UnknownUser(t *testing.T) {
	f := newRedisLeaderboardFixture(t)
	f.seed(t, true, projectionScore("u-1", 900, baseTime()))

	rank, entry, err := f.repo.GetUserRank(context.Background(), "ghost")
	if err != nil || rank != 0 || entry != nil {
		t.Fatalf("GetUserRank(ghost) = %d, %+v, %v; want 0, nil, nil", rank, entry, err)
	}
}

func TestRedisLeaderboard_Rank_SentinelID(t *testing.T) {
	f := newRedisLeaderboardFixture(t)
	f.seed(t, true, projectionScore("u-1", 900, baseTime()))

	rank, entry, err := f.repo.GetUserRank(context.Background(), lbredis.ReadySentinel)
	if err != nil || rank != 0 || entry != nil {
		t.Fatalf("GetUserRank(sentinel) = %d, %+v, %v; want 0, nil, nil", rank, entry, err)
	}
}

func TestRedisLeaderboard_SnapshotError_Propagates(t *testing.T) {
	f := newRedisLeaderboardFixture(t)
	f.seed(t, true, projectionScore("u-1", 900, baseTime()))
	f.snapshots.err = errors.New("db down")

	_, _, err := f.repo.GetGlobalTop(context.Background(), 50, 0)
	if err == nil {
		t.Fatal("GetGlobalTop err = nil, want snapshot error")
	}
	if errors.Is(err, lbredis.ErrRedisUnavailable) {
		t.Fatal("a PostgreSQL snapshot failure must not be reported as Redis unavailable")
	}
}

func TestRedisLeaderboard_RedisDown_ErrRedisUnavailable(t *testing.T) {
	_, mr := newTestRedis(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	repo := lbredis.NewLeaderboardRepository(
		client,
		lbredis.NewLeaderboardProjection(client, "node-a"),
		&fakeSnapshotRepo{snapshots: map[string]*entity.UserSnapshot{}},
		&fakeScoreSource{},
	)
	mr.Close()
	ctx := context.Background()

	if _, _, err := repo.GetGlobalTop(ctx, 50, 0); !errors.Is(err, lbredis.ErrRedisUnavailable) {
		t.Errorf("GetGlobalTop err = %v, want ErrRedisUnavailable", err)
	}
	if _, _, err := repo.GetUserRank(ctx, "u-1"); !errors.Is(err, lbredis.ErrRedisUnavailable) {
		t.Errorf("GetUserRank err = %v, want ErrRedisUnavailable", err)
	}
}

func TestRedisLeaderboard_ReadRepair_KeyLostMeanwhile_NotReady(t *testing.T) {
	f := newRedisLeaderboardFixture(t)
	ctx := context.Background()
	f.seed(t, true, projectionScore("u-1", 900, baseTime()))
	f.source.scores["u-new"] = projectionScore("u-new", 0, baseTime())
	// A concurrent Reset (or eviction) lands while read-repair loads the score.
	f.source.onGet = func() { _ = f.client.Del(ctx, leaderboardKey).Err() }

	rank, entry, err := f.repo.GetUserRank(ctx, "u-new")
	if !errors.Is(err, lbredis.ErrLeaderboardNotReady) {
		t.Fatalf("GetUserRank = %d, %+v, %v; want ErrLeaderboardNotReady", rank, entry, err)
	}
}

func TestRedisLeaderboard_UserIDCaseInsensitive(t *testing.T) {
	f := newRedisLeaderboardFixture(t)
	ctx := context.Background()
	f.seed(t, true, projectionScore("AAAA-0001", 500, baseTime()))
	f.seed(t, false, projectionScore("aaaa-0001", 400, baseTime()))

	members, _ := f.client.ZRange(ctx, leaderboardKey, 0, -1).Result()
	if len(members) != 2 || members[0] != "aaaa-0001" {
		t.Fatalf("members = %v, want one lowercase user plus the sentinel", members)
	}
	rank, entry, err := f.repo.GetUserRank(ctx, "AAAA-0001")
	if err != nil || rank != 1 || entry.UserID != "aaaa-0001" || entry.Xp != 500 {
		t.Fatalf("GetUserRank(upper) = %d, %+v, %v; want rank 1 for aaaa-0001", rank, entry, err)
	}
}

func TestRedisLeaderboard_Top_FiltersBuildMarker(t *testing.T) {
	f := newRedisLeaderboardFixture(t)
	ctx := context.Background()
	f.seed(t, true, projectionScore("u-1", 100, baseTime()))
	if err := f.projection.BeginRebuild(ctx); err != nil {
		t.Fatalf("BeginRebuild: %v", err)
	}

	top, _, err := f.repo.GetGlobalTop(ctx, 50, 0)
	if err != nil || len(top) != 1 || top[0].UserID != "u-1" {
		t.Fatalf("top = %+v, %v; want only u-1 (no internal markers)", top, err)
	}
}
