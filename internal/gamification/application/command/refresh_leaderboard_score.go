package command

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
)

// RefreshLeaderboardScoreCommand signals that a user's XP changed. The score itself is re-read
// from the source of truth, so stale, replayed or orphaned events can never write data that
// PostgreSQL does not hold.
type RefreshLeaderboardScoreCommand struct {
	UserID string
}

// RefreshLeaderboardScoreHandler re-projects one user's current score from the source of truth.
type RefreshLeaderboardScoreHandler struct {
	source     port.LeaderboardScoreSource
	projection port.LeaderboardProjection
}

// NewRefreshLeaderboardScoreHandler constructs RefreshLeaderboardScoreHandler.
func NewRefreshLeaderboardScoreHandler(
	source port.LeaderboardScoreSource,
	projection port.LeaderboardProjection,
) *RefreshLeaderboardScoreHandler {
	return &RefreshLeaderboardScoreHandler{source: source, projection: projection}
}

// Handle projects the user's stored score; users without an XP record are skipped.
func (h *RefreshLeaderboardScoreHandler) Handle(
	ctx context.Context,
	cmd RefreshLeaderboardScoreCommand,
) error {
	if cmd.UserID == "" {
		return fmt.Errorf("refresh leaderboard score: %w", errLeaderboardUserIDRequired)
	}

	score, err := h.source.GetScore(ctx, cmd.UserID)
	if err != nil {
		return fmt.Errorf("refresh leaderboard score: %w", err)
	}
	if score == nil {
		return nil
	}

	if err := h.projection.UpsertScores(ctx, []entity.LeaderboardScore{*score}); err != nil {
		return fmt.Errorf("refresh leaderboard score: %w", err)
	}
	return nil
}
