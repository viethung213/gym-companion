package port

import (
	"context"

	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
)

// LeaderboardScoreSource reads ranking scores from the source of truth.
type LeaderboardScoreSource interface {
	// ListScores pages through all users ordered by user ID ascending, starting after afterUserID (""
	// = from start).
	ListScores(
		ctx context.Context,
		afterUserID string,
		limit int,
	) ([]entity.LeaderboardScore, error)
	// GetScore returns the user's score, or nil when the user has no XP record.
	GetScore(ctx context.Context, userID string) (*entity.LeaderboardScore, error)
}

// LeaderboardProjection maintains the leaderboard read model.
// Upserts must be idempotent and must never move a user to a worse position.
type LeaderboardProjection interface {
	UpsertScores(ctx context.Context, scores []entity.LeaderboardScore) error
	// Reset discards the read model, making it not ready until MarkReady.
	Reset(ctx context.Context) error
	// BeginRebuild marks the start of a rebuild owned by the caller.
	BeginRebuild(ctx context.Context) error
	// MarkReady declares the read model complete and safe to serve. It fails when the rebuild
	// started by BeginRebuild was interrupted (lock or read model lost meanwhile).
	MarkReady(ctx context.Context) error
	TryAcquireRebuildLock(ctx context.Context) (bool, error)
	// ExtendRebuildLock renews the caller's lock; it fails when the lock is no longer owned.
	ExtendRebuildLock(ctx context.Context) error
	ReleaseRebuildLock(ctx context.Context) error
}
