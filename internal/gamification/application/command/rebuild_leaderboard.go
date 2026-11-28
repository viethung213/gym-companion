package command

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/viethung213/gym-companion/internal/gamification/application/port"
)

const defaultRebuildBatchSize = 1000

// RebuildLeaderboardCommand requests a full re-projection of the leaderboard read model.
// Reset discards the existing read model first, removing entries no longer in the source of truth.
type RebuildLeaderboardCommand struct {
	Reset bool
}

// RebuildLeaderboardResult reports what a rebuild did.
type RebuildLeaderboardResult struct {
	Skipped   bool // another instance holds the rebuild lock
	Projected int
}

// RebuildLeaderboardHandler re-projects every user's score from the source of truth.
// Upserts never lower a score, so it is safe to run while live projections continue.
type RebuildLeaderboardHandler struct {
	source     port.LeaderboardScoreSource
	projection port.LeaderboardProjection
	batchSize  int
}

// NewRebuildLeaderboardHandler constructs RebuildLeaderboardHandler; batchSize <= 0 uses 1000.
func NewRebuildLeaderboardHandler(
	source port.LeaderboardScoreSource,
	projection port.LeaderboardProjection,
	batchSize int,
) *RebuildLeaderboardHandler {
	if batchSize <= 0 {
		batchSize = defaultRebuildBatchSize
	}
	return &RebuildLeaderboardHandler{source: source, projection: projection, batchSize: batchSize}
}

// Handle rebuilds the read model and marks it ready only after every user was projected.
func (h *RebuildLeaderboardHandler) Handle(
	ctx context.Context,
	cmd RebuildLeaderboardCommand,
) (RebuildLeaderboardResult, error) {
	acquired, err := h.projection.TryAcquireRebuildLock(ctx)
	if err != nil {
		return RebuildLeaderboardResult{}, fmt.Errorf("rebuild leaderboard: %w", err)
	}
	if !acquired {
		return RebuildLeaderboardResult{Skipped: true}, nil
	}
	defer h.releaseLock(ctx)

	projected, err := h.rebuild(ctx, cmd.Reset)
	result := RebuildLeaderboardResult{Projected: projected}
	if err != nil {
		return result, fmt.Errorf("rebuild leaderboard: %w", err)
	}
	return result, nil
}

func (h *RebuildLeaderboardHandler) rebuild(ctx context.Context, reset bool) (int, error) {
	if reset {
		if err := h.projection.Reset(ctx); err != nil {
			return 0, err
		}
	}
	if err := h.projection.BeginRebuild(ctx); err != nil {
		return 0, err
	}
	projected, err := h.projectAll(ctx)
	if err != nil {
		return projected, err
	}
	return projected, h.projection.MarkReady(ctx)
}

// projectAll pages through the source of truth, renewing the lock after every page so a long
// rebuild never outlives it.
func (h *RebuildLeaderboardHandler) projectAll(ctx context.Context) (int, error) {
	projected := 0
	cursor := ""
	for {
		page, err := h.source.ListScores(ctx, cursor, h.batchSize)
		if err != nil {
			return projected, err
		}
		if len(page) == 0 {
			return projected, nil
		}
		if err := h.projection.UpsertScores(ctx, page); err != nil {
			return projected, err
		}
		projected += len(page)
		if err := h.projection.ExtendRebuildLock(ctx); err != nil {
			return projected, err
		}
		if len(page) < h.batchSize {
			return projected, nil
		}
		cursor = page[len(page)-1].UserID
	}
}

func (h *RebuildLeaderboardHandler) releaseLock(ctx context.Context) {
	if err := h.projection.ReleaseRebuildLock(context.WithoutCancel(ctx)); err != nil {
		slog.WarnContext(ctx, "Failed to release leaderboard rebuild lock; it expires on its own",
			slog.String("module", "gamification"), slog.String("error", err.Error()))
	}
}
