package redis_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	lbredis "github.com/viethung213/gym-companion/internal/gamification/infrastructure/redis"
)

const profileTTL = 24 * time.Hour

// countingSnapshotRepo is a PostgreSQL stand-in that records which IDs were loaded.
type countingSnapshotRepo struct {
	snapshots map[string]*entity.UserSnapshot
	loaded    [][]string
	saved     []*entity.UserSnapshot
	getErr    error
	saveErr   error
	onLoad    func()
}

func newCountingSnapshotRepo(snaps ...*entity.UserSnapshot) *countingSnapshotRepo {
	r := &countingSnapshotRepo{snapshots: map[string]*entity.UserSnapshot{}}
	for _, s := range snaps {
		r.snapshots[s.UserID] = s
	}
	return r
}

func (r *countingSnapshotRepo) SaveSnapshot(_ context.Context, s *entity.UserSnapshot) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.saved = append(r.saved, s)
	r.snapshots[s.UserID] = s
	return nil
}

func (r *countingSnapshotRepo) GetSnapshot(ctx context.Context, userID string) (*entity.UserSnapshot, error) {
	found, err := r.GetSnapshots(ctx, []string{userID})
	if err != nil {
		return nil, err
	}
	return found[userID], nil
}

func (r *countingSnapshotRepo) GetSnapshots(
	_ context.Context,
	userIDs []string,
) (map[string]*entity.UserSnapshot, error) {
	r.loaded = append(r.loaded, slices.Clone(userIDs))
	if r.onLoad != nil {
		r.onLoad()
	}
	if r.getErr != nil {
		return nil, r.getErr
	}
	out := make(map[string]*entity.UserSnapshot, len(userIDs))
	for _, id := range userIDs {
		if s, ok := r.snapshots[id]; ok {
			out[id] = s
		}
	}
	return out, nil
}

func snapshotAt(userID, name string) *entity.UserSnapshot {
	return entity.NewUserSnapshot(userID, name, "https://cdn/"+userID+".png",
		time.Date(2026, 11, 20, 6, 30, 15, 123456000, time.UTC))
}

func profileKey(userID string) string { return "gamification:profile:" + userID }

func TestCachedSnapshots_MissLoadsOnceAndCaches(t *testing.T) {
	client, mr := newTestRedis(t)
	ctx := context.Background()
	pg := newCountingSnapshotRepo(snapshotAt("u-1", "Alpha"), snapshotAt("u-2", "Bravo"))
	repo := lbredis.NewCachedUserSnapshotRepository(client, pg, profileTTL)

	got, err := repo.GetSnapshots(ctx, []string{"u-1", "u-2"})
	if err != nil {
		t.Fatalf("GetSnapshots: %v", err)
	}
	if len(got) != 2 || *got["u-1"] != *pg.snapshots["u-1"] || *got["u-2"] != *pg.snapshots["u-2"] {
		t.Fatalf("GetSnapshots = %+v, want both snapshots from PostgreSQL", got)
	}
	if len(pg.loaded) != 1 || len(pg.loaded[0]) != 2 {
		t.Fatalf("postgres loads = %v, want one load of both ids", pg.loaded)
	}
	if ttl := mr.TTL(profileKey("u-1")); ttl != profileTTL {
		t.Errorf("cache ttl = %v, want %v", ttl, profileTTL)
	}

	again, err := repo.GetSnapshots(ctx, []string{"u-1", "u-2"})
	if err != nil || len(again) != 2 || *again["u-1"] != *pg.snapshots["u-1"] {
		t.Fatalf("cached GetSnapshots = %+v, %v", again, err)
	}
	if len(pg.loaded) != 1 {
		t.Fatalf("postgres loads = %d, want 1 (second read served from Redis)", len(pg.loaded))
	}
}

func TestCachedSnapshots_PartialHitLoadsOnlyMisses(t *testing.T) {
	client, _ := newTestRedis(t)
	ctx := context.Background()
	pg := newCountingSnapshotRepo(snapshotAt("u-1", "Alpha"), snapshotAt("u-2", "Bravo"))
	repo := lbredis.NewCachedUserSnapshotRepository(client, pg, profileTTL)

	_, _ = repo.GetSnapshots(ctx, []string{"u-1"})
	got, err := repo.GetSnapshots(ctx, []string{"u-1", "u-2", "ghost"})
	if err != nil || len(got) != 2 {
		t.Fatalf("GetSnapshots = %+v, %v; want u-1 and u-2", got, err)
	}
	if last := pg.loaded[len(pg.loaded)-1]; !slices.Equal(last, []string{"u-2", "ghost"}) {
		t.Fatalf("second postgres load = %v, want only the misses [u-2 ghost]", last)
	}
}

func TestCachedSnapshots_MissingUserNotCached(t *testing.T) {
	client, mr := newTestRedis(t)
	repo := lbredis.NewCachedUserSnapshotRepository(client, newCountingSnapshotRepo(), profileTTL)

	got, err := repo.GetSnapshots(context.Background(), []string{"ghost"})
	if err != nil || len(got) != 0 {
		t.Fatalf("GetSnapshots(ghost) = %+v, %v; want empty", got, err)
	}
	if mr.Exists(profileKey("ghost")) {
		t.Fatal("a user without snapshot was cached")
	}
}

func TestCachedSnapshots_EmptyAvatarRoundTrips(t *testing.T) {
	client, _ := newTestRedis(t)
	ctx := context.Background()
	snap := entity.NewUserSnapshot("u-1", "No Avatar", "", time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	pg := newCountingSnapshotRepo(snap)
	repo := lbredis.NewCachedUserSnapshotRepository(client, pg, profileTTL)

	_, _ = repo.GetSnapshots(ctx, []string{"u-1"})
	got, err := repo.GetSnapshot(ctx, "u-1")
	if err != nil || got == nil || *got != *snap || len(pg.loaded) != 1 {
		t.Fatalf("GetSnapshot = %+v, %v (loads %d); want cached %+v", got, err, len(pg.loaded), snap)
	}
}

func TestCachedSnapshots_KeysAreCaseInsensitive(t *testing.T) {
	client, mr := newTestRedis(t)
	ctx := context.Background()
	pg := newCountingSnapshotRepo(snapshotAt("aaaa-0001", "Alpha"))
	repo := lbredis.NewCachedUserSnapshotRepository(client, pg, profileTTL)

	_, _ = repo.GetSnapshots(ctx, []string{"aaaa-0001"})
	if !mr.Exists(profileKey("aaaa-0001")) {
		t.Fatal("profile not cached under the lowercase key")
	}
	got, err := repo.GetSnapshot(ctx, "AAAA-0001")
	if err != nil || got == nil || got.DisplayName != "Alpha" || len(pg.loaded) != 1 {
		t.Fatalf("GetSnapshot(upper) = %+v, %v (loads %d); want cached Alpha", got, err, len(pg.loaded))
	}
}

func TestCachedSnapshots_RedisDownFallsBackToPostgres(t *testing.T) {
	_, mr := newTestRedis(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	pg := newCountingSnapshotRepo(snapshotAt("u-1", "Alpha"))
	repo := lbredis.NewCachedUserSnapshotRepository(client, pg, profileTTL)
	mr.Close()

	got, err := repo.GetSnapshots(context.Background(), []string{"u-1"})
	if err != nil || got["u-1"] == nil || got["u-1"].DisplayName != "Alpha" {
		t.Fatalf("GetSnapshots with Redis down = %+v, %v; want PostgreSQL result", got, err)
	}
}

func TestCachedSnapshots_PostgresErrorPropagates(t *testing.T) {
	client, _ := newTestRedis(t)
	pg := newCountingSnapshotRepo()
	pg.getErr = errors.New("db down")
	repo := lbredis.NewCachedUserSnapshotRepository(client, pg, profileTTL)

	if _, err := repo.GetSnapshots(context.Background(), []string{"u-1"}); err == nil {
		t.Fatal("GetSnapshots err = nil, want PostgreSQL error")
	}
}

func TestCachedSnapshots_EmptyInput(t *testing.T) {
	client, _ := newTestRedis(t)
	pg := newCountingSnapshotRepo()
	repo := lbredis.NewCachedUserSnapshotRepository(client, pg, profileTTL)

	got, err := repo.GetSnapshots(context.Background(), nil)
	if err != nil || got == nil || len(got) != 0 || len(pg.loaded) != 0 {
		t.Fatalf("GetSnapshots(nil) = %v, %v (loads %d); want empty map, no load", got, err, len(pg.loaded))
	}
}

func TestCachedSnapshots_SaveWritesThrough(t *testing.T) {
	client, mr := newTestRedis(t)
	ctx := context.Background()
	pg := newCountingSnapshotRepo(snapshotAt("u-1", "Old Name"))
	repo := lbredis.NewCachedUserSnapshotRepository(client, pg, profileTTL)
	_, _ = repo.GetSnapshot(ctx, "u-1")

	if err := repo.SaveSnapshot(ctx, snapshotAt("u-1", "New Name")); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	if len(pg.saved) != 1 {
		t.Fatalf("postgres saves = %d, want 1", len(pg.saved))
	}
	if ttl := mr.TTL(profileKey("u-1")); ttl != profileTTL {
		t.Errorf("cache ttl after save = %v, want %v", ttl, profileTTL)
	}
	loads := len(pg.loaded)
	got, err := repo.GetSnapshot(ctx, "u-1")
	if err != nil || got.DisplayName != "New Name" || len(pg.loaded) != loads {
		t.Fatalf("GetSnapshot after rename = %+v, %v; want New Name served from Redis", got, err)
	}
}

func TestCachedSnapshots_StaleReadCannotOverwriteConcurrentSave(t *testing.T) {
	client, _ := newTestRedis(t)
	ctx := context.Background()
	pg := newCountingSnapshotRepo(snapshotAt("u-1", "Old Name"))
	repo := lbredis.NewCachedUserSnapshotRepository(client, pg, profileTTL)

	// The reader loads "Old Name" from PostgreSQL; a rename commits before it writes the cache.
	stale := *pg.snapshots["u-1"]
	pg.onLoad = func() {
		pg.onLoad = nil
		if err := repo.SaveSnapshot(ctx, snapshotAt("u-1", "New Name")); err != nil {
			t.Fatalf("SaveSnapshot: %v", err)
		}
		pg.snapshots["u-1"] = &stale
	}
	_, _ = repo.GetSnapshot(ctx, "u-1")

	pg.snapshots["u-1"] = snapshotAt("u-1", "New Name")
	got, err := repo.GetSnapshot(ctx, "u-1")
	if err != nil || got.DisplayName != "New Name" {
		t.Fatalf("cached name = %+v, %v; a stale read overwrote the newer save", got, err)
	}
}

func TestCachedSnapshots_SaveSucceedsWhenRedisDown(t *testing.T) {
	_, mr := newTestRedis(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	pg := newCountingSnapshotRepo()
	repo := lbredis.NewCachedUserSnapshotRepository(client, pg, profileTTL)
	mr.Close()

	if err := repo.SaveSnapshot(context.Background(), snapshotAt("u-1", "Alpha")); err != nil {
		t.Fatalf("SaveSnapshot err = %v, want nil (PostgreSQL is the source of truth)", err)
	}
	if len(pg.saved) != 1 {
		t.Fatal("snapshot not saved to PostgreSQL")
	}
}

func TestCachedSnapshots_SaveErrorKeepsCacheAndPropagates(t *testing.T) {
	client, mr := newTestRedis(t)
	ctx := context.Background()
	pg := newCountingSnapshotRepo(snapshotAt("u-1", "Alpha"))
	repo := lbredis.NewCachedUserSnapshotRepository(client, pg, profileTTL)
	_, _ = repo.GetSnapshot(ctx, "u-1")
	pg.saveErr = errors.New("db down")

	if err := repo.SaveSnapshot(ctx, snapshotAt("u-1", "Bravo")); err == nil {
		t.Fatal("SaveSnapshot err = nil, want PostgreSQL error")
	}
	if !mr.Exists(profileKey("u-1")) {
		t.Fatal("cache dropped although nothing changed in PostgreSQL")
	}
}

func TestCachedSnapshots_FailedWriteThroughFlushesCacheOnRecovery(t *testing.T) {
	client, mr := newTestRedis(t)
	ctx := context.Background()
	pg := newCountingSnapshotRepo(snapshotAt("u-1", "Old Name"), snapshotAt("u-2", "Bravo"))
	repo := lbredis.NewCachedUserSnapshotRepository(client, pg, profileTTL)
	_, _ = repo.GetSnapshots(ctx, []string{"u-1", "u-2"})

	// The rename commits to PostgreSQL while Redis rejects the write-through, then Redis comes
	// back still holding the old profile (as it does when restarted from its own persistence).
	mr.SetError("LOADING Redis is loading the dataset in memory")
	if err := repo.SaveSnapshot(ctx, snapshotAt("u-1", "New Name")); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	mr.SetError("")

	got, err := repo.GetSnapshots(ctx, []string{"u-1", "u-2"})
	if err != nil || got["u-1"] == nil || got["u-1"].DisplayName != "New Name" {
		t.Fatalf("GetSnapshots after recovery = %+v, %v; want New Name, not the stale cache", got, err)
	}
	if mr.Exists(profileKey("u-2")) && mr.HGet(profileKey("u-2"), "display_name") != "Bravo" {
		t.Fatal("unrelated profile corrupted by the flush")
	}

	loads := len(pg.loaded)
	if _, err := repo.GetSnapshots(ctx, []string{"u-1", "u-2"}); err != nil || len(pg.loaded) != loads {
		t.Fatalf("read after flush = %v (loads %d -> %d); want served from the refilled cache",
			err, loads, len(pg.loaded))
	}
}

func TestCachedSnapshots_StaysOnPostgresWhileFlushFails(t *testing.T) {
	client, mr := newTestRedis(t)
	ctx := context.Background()
	pg := newCountingSnapshotRepo(snapshotAt("u-1", "Old Name"))
	repo := lbredis.NewCachedUserSnapshotRepository(client, pg, profileTTL)
	_, _ = repo.GetSnapshot(ctx, "u-1")

	mr.SetError("LOADING Redis is loading the dataset in memory")
	_ = repo.SaveSnapshot(ctx, snapshotAt("u-1", "New Name"))

	got, err := repo.GetSnapshot(ctx, "u-1")
	if err != nil || got == nil || got.DisplayName != "New Name" {
		t.Fatalf("GetSnapshot while Redis still failing = %+v, %v; want PostgreSQL's New Name", got, err)
	}
}
