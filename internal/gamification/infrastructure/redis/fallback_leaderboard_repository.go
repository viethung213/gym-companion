package redis

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
)

// FallbackLeaderboardRepository serves from primary (Redis) and falls back to the source of truth
// whenever primary fails or is not ready, so leaderboard reads never depend on Redis availability.
//
// After a Redis failure (ErrRedisUnavailable, not caused by the caller cancelling) primary is
// skipped for the cooldown, so an unreachable Redis costs one slow request per cooldown instead
// of a dial timeout on every request.
type FallbackLeaderboardRepository struct {
	primary    repository.LeaderboardRepository
	fallback   repository.LeaderboardRepository
	onNotReady func()
	cooldown   time.Duration
	skipUntil  atomic.Int64 // unix nanos; primary is bypassed until then
}

// Compile-time interface check.
var _ repository.LeaderboardRepository = (*FallbackLeaderboardRepository)(nil)

// NewFallbackLeaderboardRepository constructs the decorator. onNotReady (optional) is invoked when
// primary reports ErrLeaderboardNotReady, e.g. to request an early rebuild.
func NewFallbackLeaderboardRepository(
	primary, fallback repository.LeaderboardRepository,
	onNotReady func(),
	cooldown time.Duration,
) *FallbackLeaderboardRepository {
	return &FallbackLeaderboardRepository{
		primary:    primary,
		fallback:   fallback,
		onNotReady: onNotReady,
		cooldown:   cooldown,
	}
}

// GetGlobalTop delegates to primary, falling back on error or while primary is skipped.
func (r *FallbackLeaderboardRepository) GetGlobalTop(
	ctx context.Context,
	limit, offset int,
) ([]*entity.LeaderboardUser, int64, error) {
	if r.primaryAvailable() {
		users, total, err := r.primary.GetGlobalTop(ctx, limit, offset)
		if err == nil {
			return users, total, nil
		}
		r.handlePrimaryError(ctx, "GetGlobalTop", err)
	}
	return r.fallback.GetGlobalTop(ctx, limit, offset)
}

// GetUserRank delegates to primary, falling back on error or while primary is skipped.
func (r *FallbackLeaderboardRepository) GetUserRank(
	ctx context.Context,
	userID string,
) (int64, *entity.LeaderboardUser, error) {
	if r.primaryAvailable() {
		rank, entry, err := r.primary.GetUserRank(ctx, userID)
		if err == nil {
			return rank, entry, nil
		}
		r.handlePrimaryError(ctx, "GetUserRank", err)
	}
	return r.fallback.GetUserRank(ctx, userID)
}

func (r *FallbackLeaderboardRepository) primaryAvailable() bool {
	return time.Now().UnixNano() >= r.skipUntil.Load()
}

func (r *FallbackLeaderboardRepository) handlePrimaryError(
	ctx context.Context,
	operation string,
	err error,
) {
	notReady := errors.Is(err, ErrLeaderboardNotReady)
	attrs := []any{
		slog.String("module", "gamification"),
		slog.String("operation", operation),
		slog.String("error", err.Error()),
	}
	if redisDown(ctx, err) {
		r.skipUntil.Store(time.Now().Add(r.cooldown).UnixNano())
		attrs = append(attrs, slog.Duration("skip_redis_for", r.cooldown))
	}
	slog.WarnContext(ctx, "Leaderboard read model unavailable, falling back to PostgreSQL", attrs...)

	if notReady && r.onNotReady != nil {
		r.onNotReady()
	}
}

// redisDown reports whether err says Redis itself is failing, as opposed to the caller giving up
// (cancellation, deadline) or a PostgreSQL lookup inside the Redis repository failing.
func redisDown(ctx context.Context, err error) bool {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	return errors.Is(err, ErrRedisUnavailable)
}
