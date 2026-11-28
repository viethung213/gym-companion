package redis

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
)

const (
	profileKeyPrefix      = "gamification:profile:"
	profileFieldName      = "display_name"
	profileFieldAvatar    = "avatar_url"
	profileFieldUpdatedAt = "updated_at"

	// flushBatchSize bounds the keys scanned and unlinked per round trip when flushing.
	flushBatchSize = 1000

	// fillProfileLua caches a profile loaded by a reader only if no entry exists yet. A save that
	// committed while the reader was loading has already written the newer profile, so the
	// reader's older copy is discarded instead of overwriting it.
	fillProfileLua = `
if redis.call("EXISTS", KEYS[1]) == 1 then
	return 0
end
redis.call("HSET", KEYS[1], ARGV[1], ARGV[2], ARGV[3], ARGV[4], ARGV[5], ARGV[6])
redis.call("PEXPIRE", KEYS[1], ARGV[7])
return 1
`
)

// CachedUserSnapshotRepository is a read-through Redis Hash cache in front of the PostgreSQL
// snapshot repository, which stays the source of truth. Only profiles that are read or saved are
// cached, each with a TTL, so memory follows activity rather than the total user count.
//
// Redis failures never fail a call: reads fall back to PostgreSQL and writes still persist to
// PostgreSQL. A write-through that fails leaves Redis holding an outdated profile (Redis may
// come back from its own persistence), so every cached profile is flushed before the cache is
// trusted again; until that flush succeeds reads go to PostgreSQL. If the process stops before
// it can flush, the TTL bounds the staleness.
type CachedUserSnapshotRepository struct {
	client *goredis.Client
	source repository.UserSnapshotRepository
	ttl    time.Duration

	// missedWrites counts failed write-throughs; flushedWrites is the count the last successful
	// flush covered. The cache is trusted only while the two are equal.
	missedWrites  atomic.Uint64
	flushedWrites atomic.Uint64
	flushMu       sync.Mutex
}

// Compile-time interface check.
var _ repository.UserSnapshotRepository = (*CachedUserSnapshotRepository)(nil)

// NewCachedUserSnapshotRepository constructs the cache; ttl bounds how long a profile is kept.
func NewCachedUserSnapshotRepository(
	client *goredis.Client,
	source repository.UserSnapshotRepository,
	ttl time.Duration,
) *CachedUserSnapshotRepository {
	return &CachedUserSnapshotRepository{client: client, source: source, ttl: ttl}
}

// SaveSnapshot persists to PostgreSQL first, then writes the new profile through to the cache
// (overwriting any cached copy) so readers see it immediately.
func (r *CachedUserSnapshotRepository) SaveSnapshot(
	ctx context.Context,
	snapshot *entity.UserSnapshot,
) error {
	if err := r.source.SaveSnapshot(ctx, snapshot); err != nil {
		return err
	}
	key := profileKey(snapshot.UserID)
	tx := r.client.TxPipeline()
	tx.HSet(ctx, key, profileFields(snapshot)...)
	tx.PExpire(ctx, key, r.ttl)
	if _, err := tx.Exec(ctx); err != nil {
		r.missedWrites.Add(1)
		r.logDegraded(ctx, "write-through", err)
	}
	return nil
}

// GetSnapshot returns one user's snapshot, or nil when the user has none.
func (r *CachedUserSnapshotRepository) GetSnapshot(
	ctx context.Context,
	userID string,
) (*entity.UserSnapshot, error) {
	found, err := r.GetSnapshots(ctx, []string{userID})
	if err != nil {
		return nil, err
	}
	return found[userID], nil
}

// GetSnapshots serves cached profiles from Redis and loads all misses from PostgreSQL in one
// query, caching what was found. Results are keyed by the requested IDs.
func (r *CachedUserSnapshotRepository) GetSnapshots(
	ctx context.Context,
	userIDs []string,
) (map[string]*entity.UserSnapshot, error) {
	result := make(map[string]*entity.UserSnapshot, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}
	if !r.cacheTrusted(ctx) {
		return r.source.GetSnapshots(ctx, userIDs)
	}

	misses := r.readCached(ctx, userIDs, result)
	if len(misses) == 0 {
		return result, nil
	}

	loaded, err := r.source.GetSnapshots(ctx, misses)
	if err != nil {
		return nil, err
	}
	for id, snapshot := range loaded {
		result[id] = snapshot
	}
	r.writeCache(ctx, loaded)
	return result, nil
}

// cacheTrusted reports whether cached profiles can be served, flushing them first when a
// write-through was missed. Only one request flushes at a time; the others read PostgreSQL.
func (r *CachedUserSnapshotRepository) cacheTrusted(ctx context.Context) bool {
	if r.missedWrites.Load() == r.flushedWrites.Load() {
		return true
	}
	if !r.flushMu.TryLock() {
		return false
	}
	defer r.flushMu.Unlock()

	// A write-through failing after this load is not covered by the flush below; it keeps the
	// counters apart, so the next read flushes again.
	missed := r.missedWrites.Load()
	if missed == r.flushedWrites.Load() {
		return true
	}
	if err := r.flushProfiles(ctx); err != nil {
		r.logDegraded(ctx, "flush", err)
		return false
	}
	r.flushedWrites.Store(missed)
	slog.InfoContext(ctx, "Leaderboard profile cache flushed after missed write-through",
		slog.String("module", "gamification"))
	return true
}

// flushProfiles removes every cached profile; they are refilled from PostgreSQL on demand.
func (r *CachedUserSnapshotRepository) flushProfiles(ctx context.Context) error {
	iter := r.client.Scan(ctx, 0, profileKeyPrefix+"*", flushBatchSize).Iterator()
	batch := make([]string, 0, flushBatchSize)
	for iter.Next(ctx) {
		batch = append(batch, iter.Val())
		if len(batch) == flushBatchSize {
			if err := r.client.Unlink(ctx, batch...).Err(); err != nil {
				return fmt.Errorf("unlink cached profiles: %w", err)
			}
			batch = batch[:0]
		}
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("scan cached profiles: %w", err)
	}
	if len(batch) == 0 {
		return nil
	}
	if err := r.client.Unlink(ctx, batch...).Err(); err != nil {
		return fmt.Errorf("unlink cached profiles: %w", err)
	}
	return nil
}

// readCached fills result with cached profiles and returns the IDs that must be loaded.
func (r *CachedUserSnapshotRepository) readCached(
	ctx context.Context,
	userIDs []string,
	result map[string]*entity.UserSnapshot,
) []string {
	pipe := r.client.Pipeline()
	cmds := make([]*goredis.SliceCmd, len(userIDs))
	for i, id := range userIDs {
		cmds[i] = pipe.HMGet(ctx, profileKey(id),
			profileFieldName, profileFieldAvatar, profileFieldUpdatedAt)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		r.logDegraded(ctx, "read", err)
		return userIDs
	}

	var misses []string
	for i, id := range userIDs {
		snapshot, ok := decodeProfile(id, cmds[i].Val())
		if !ok {
			misses = append(misses, id)
			continue
		}
		result[id] = snapshot
	}
	return misses
}

func (r *CachedUserSnapshotRepository) writeCache(
	ctx context.Context,
	snapshots map[string]*entity.UserSnapshot,
) {
	if len(snapshots) == 0 {
		return
	}
	ttlMillis := r.ttl.Milliseconds()
	pipe := r.client.Pipeline()
	for _, s := range snapshots {
		args := append(profileFields(s), ttlMillis)
		pipe.Eval(ctx, fillProfileLua, []string{profileKey(s.UserID)}, args...)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		r.logDegraded(ctx, "fill", err)
	}
}

// profileFields lists the hash field/value pairs stored for a snapshot.
func profileFields(s *entity.UserSnapshot) []any {
	return []any{
		profileFieldName, s.DisplayName,
		profileFieldAvatar, s.AvatarURL,
		profileFieldUpdatedAt, s.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func (r *CachedUserSnapshotRepository) logDegraded(
	ctx context.Context,
	operation string,
	err error,
) {
	slog.WarnContext(ctx, "Leaderboard profile cache unavailable, using PostgreSQL",
		slog.String("module", "gamification"),
		slog.String("operation", operation),
		slog.String("error", err.Error()))
}

func profileKey(userID string) string {
	return profileKeyPrefix + normalizeMember(userID)
}

// decodeProfile maps HMGET values back to a snapshot; ok is false for a missing or corrupt entry,
// which is then treated as a cache miss.
func decodeProfile(userID string, values []any) (*entity.UserSnapshot, bool) {
	if len(values) != 3 {
		return nil, false
	}
	name, nameOK := values[0].(string)
	avatar, avatarOK := values[1].(string)
	rawUpdatedAt, updatedOK := values[2].(string)
	if !nameOK || !avatarOK || !updatedOK {
		return nil, false
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, rawUpdatedAt)
	if err != nil {
		return nil, false
	}
	return entity.NewUserSnapshot(userID, name, avatar, updatedAt), true
}
