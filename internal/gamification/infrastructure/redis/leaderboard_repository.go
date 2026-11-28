package redis

import (
	"context"
	"errors"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
)

const defaultLeaderboardLimit = 50

type scoredMember struct {
	userID string
	score  float64
}

// LeaderboardRepository serves leaderboard queries from the Redis read model.
//
// Every read checks the ready sentinel inside the same MULTI/EXEC as the data it reads, so a
// concurrent reset or eviction can never be observed as an empty-but-ready board.
type LeaderboardRepository struct {
	client     *goredis.Client
	projection *LeaderboardProjection
	snapshots  repository.UserSnapshotRepository
	source     port.LeaderboardScoreSource
}

// Compile-time interface check.
var _ repository.LeaderboardRepository = (*LeaderboardRepository)(nil)

// NewLeaderboardRepository constructs a LeaderboardRepository. source and projection are used to
// read-repair users missing from the ZSET.
func NewLeaderboardRepository(
	client *goredis.Client,
	projection *LeaderboardProjection,
	snapshots repository.UserSnapshotRepository,
	source port.LeaderboardScoreSource,
) *LeaderboardRepository {
	return &LeaderboardRepository{
		client:     client,
		projection: projection,
		snapshots:  snapshots,
		source:     source,
	}
}

// GetGlobalTop returns users ranked offset+1..offset+limit.
func (r *LeaderboardRepository) GetGlobalTop(
	ctx context.Context,
	limit, offset int,
) ([]*entity.LeaderboardUser, int64, error) {
	if limit <= 0 {
		limit = defaultLeaderboardLimit
	}
	if offset < 0 {
		offset = 0
	}

	tx := r.client.TxPipeline()
	readyCmd := tx.ZScore(ctx, leaderboardKey, ReadySentinel)
	rangeCmd := tx.ZRangeWithScores(ctx, leaderboardKey, int64(offset), int64(offset+limit-1))
	if err := execReadTx(ctx, tx, readyCmd, "read leaderboard top"); err != nil {
		return nil, 0, err
	}

	members := make([]scoredMember, 0, len(rangeCmd.Val()))
	userIDs := make([]string, 0, len(rangeCmd.Val()))
	for _, z := range rangeCmd.Val() {
		id, ok := z.Member.(string)
		if !ok || isInternalMember(id) {
			continue
		}
		members = append(members, scoredMember{userID: id, score: z.Score})
		userIDs = append(userIDs, id)
	}

	snapshots, err := r.snapshots.GetSnapshots(ctx, userIDs)
	if err != nil {
		return nil, 0, fmt.Errorf("load leaderboard snapshots: %w", err)
	}

	result := make([]*entity.LeaderboardUser, 0, len(members))
	for i, m := range members {
		rank := int64(offset + i + 1)
		result = append(result, toLeaderboardUser(rank, m.userID, m.score, snapshots[m.userID]))
	}
	return result, int64(len(result)), nil
}

// GetUserRank returns the user's 1-based rank, or (0, nil, nil) when the user has no XP record.
func (r *LeaderboardRepository) GetUserRank(
	ctx context.Context,
	userID string,
) (int64, *entity.LeaderboardUser, error) {
	member := normalizeMember(userID)
	if member == "" || isInternalMember(member) {
		return 0, nil, nil
	}

	ranked, err := r.readMember(ctx, member)
	if err != nil {
		return 0, nil, err
	}
	if ranked == nil {
		if ranked, err = r.readRepair(ctx, member); err != nil || ranked == nil {
			return 0, nil, err
		}
	}

	snapshot, err := r.snapshots.GetSnapshot(ctx, member)
	if err != nil {
		return 0, nil, fmt.Errorf("load leaderboard snapshot: %w", err)
	}

	entry := toLeaderboardUser(ranked.rank+1, member, ranked.score, snapshot)
	return entry.Rank, entry, nil
}

type rankedMember struct {
	rank  int64 // 0-based ZRANK
	score float64
}

// readMember reads a member's rank and score atomically with the ready check; nil when absent.
func (r *LeaderboardRepository) readMember(
	ctx context.Context,
	member string,
) (*rankedMember, error) {
	tx := r.client.TxPipeline()
	readyCmd := tx.ZScore(ctx, leaderboardKey, ReadySentinel)
	rankCmd := tx.ZRank(ctx, leaderboardKey, member)
	scoreCmd := tx.ZScore(ctx, leaderboardKey, member)
	if err := execReadTx(ctx, tx, readyCmd, "read leaderboard rank"); err != nil {
		return nil, err
	}
	if errors.Is(rankCmd.Err(), goredis.Nil) {
		return nil, nil
	}
	return &rankedMember{rank: rankCmd.Val(), score: scoreCmd.Val()}, nil
}

// readRepair projects a member missing from the ZSET (e.g. a skipped event) from the source of
// truth and re-reads it. It returns nil when the user has no XP record.
func (r *LeaderboardRepository) readRepair(
	ctx context.Context,
	member string,
) (*rankedMember, error) {
	score, err := r.source.GetScore(ctx, member)
	if err != nil {
		return nil, fmt.Errorf("read-repair load score: %w", err)
	}
	if score == nil {
		return nil, nil
	}
	if err = r.projection.UpsertScores(ctx, []entity.LeaderboardScore{*score}); err != nil {
		return nil, fmt.Errorf("read-repair: %w", err)
	}

	ranked, err := r.readMember(ctx, member)
	if err != nil {
		return nil, fmt.Errorf("read-repair: %w", err)
	}
	if ranked == nil {
		return nil, fmt.Errorf("read-repair: member vanished: %w", ErrLeaderboardNotReady)
	}
	return ranked, nil
}

// execReadTx runs a read transaction whose first command is the ready check. Missing members
// (redis.Nil) are not errors; a missing ready sentinel is ErrLeaderboardNotReady.
func execReadTx(
	ctx context.Context,
	tx goredis.Pipeliner,
	readyCmd *goredis.FloatCmd,
	operation string,
) error {
	if _, err := tx.Exec(ctx); err != nil && !errors.Is(err, goredis.Nil) {
		return redisError(operation, err)
	}
	if errors.Is(readyCmd.Err(), goredis.Nil) {
		return ErrLeaderboardNotReady
	}
	if err := readyCmd.Err(); err != nil {
		return redisError(operation, err)
	}
	return nil
}

func toLeaderboardUser(
	rank int64,
	userID string,
	score float64,
	snapshot *entity.UserSnapshot,
) *entity.LeaderboardUser {
	xp, updatedAt := decodeScore(score)
	user := &entity.LeaderboardUser{
		Rank:        rank,
		UserID:      userID,
		DisplayName: entity.DefaultDisplayName,
		Xp:          xp,
		Level:       vo.CalculateLevel(xp),
		UpdatedAt:   updatedAt,
	}
	if snapshot != nil {
		user.DisplayName = entity.DisplayNameOrDefault(snapshot.DisplayName)
		user.AvatarURL = snapshot.AvatarURL
	}
	return user
}
