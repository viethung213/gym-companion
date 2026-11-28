package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/event"
)

var (
	errLeaderboardUserIDRequired    = errors.New("user id is required")
	errLeaderboardUpdatedAtRequired = errors.New("updated at is required")
)

// ProjectLeaderboardScoreCommand carries a user's absolute lifetime XP and the time it last
// changed.
type ProjectLeaderboardScoreCommand struct {
	UserID    string
	Xp        int64
	UpdatedAt time.Time
}

func (c ProjectLeaderboardScoreCommand) validate() error {
	switch {
	case c.UserID == "":
		return errLeaderboardUserIDRequired
	case c.Xp < 0:
		return fmt.Errorf("xp must not be negative, got %d", c.Xp)
	case c.UpdatedAt.IsZero():
		return errLeaderboardUpdatedAtRequired
	}
	return nil
}

// ProjectLeaderboardScoreHandler pushes a user's score into the leaderboard read model.
type ProjectLeaderboardScoreHandler struct {
	projection port.LeaderboardProjection
}

// NewProjectLeaderboardScoreHandler constructs ProjectLeaderboardScoreHandler.
func NewProjectLeaderboardScoreHandler(
	projection port.LeaderboardProjection,
) *ProjectLeaderboardScoreHandler {
	return &ProjectLeaderboardScoreHandler{projection: projection}
}

// Handle projects one score. Projection is idempotent, so redelivered commands are safe.
func (h *ProjectLeaderboardScoreHandler) Handle(
	ctx context.Context,
	cmd ProjectLeaderboardScoreCommand,
) error {
	if err := cmd.validate(); err != nil {
		return fmt.Errorf("project leaderboard score: %w", err)
	}
	score := entity.LeaderboardScore{UserID: cmd.UserID, Xp: cmd.Xp, UpdatedAt: cmd.UpdatedAt}
	if err := h.projection.UpsertScores(ctx, []entity.LeaderboardScore{score}); err != nil {
		return fmt.Errorf("project leaderboard score: %w", err)
	}
	return nil
}

// ProjectBestEffort projects scores after the source of truth has committed. Failures are only
// logged: the Kafka consumer and periodic reconcile repair the read model. A nil handler is a
// no-op, which is how the Redis read model is disabled.
func (h *ProjectLeaderboardScoreHandler) ProjectBestEffort(
	ctx context.Context,
	cmds ...ProjectLeaderboardScoreCommand,
) {
	if h == nil {
		return
	}
	for _, cmd := range cmds {
		if err := h.Handle(ctx, cmd); err != nil {
			slog.WarnContext(
				ctx,
				"Leaderboard fast-path projection failed; relying on consumer/reconcile",
				slog.String("module", "gamification"),
				slog.String("user_id", cmd.UserID),
				slog.String("error", err.Error()),
			)
		}
	}
}

// XpEarnedProjections extracts the leaderboard scores carried by XpEarned events.
func XpEarnedProjections(events []event.Event) []ProjectLeaderboardScoreCommand {
	cmds := make([]ProjectLeaderboardScoreCommand, 0, len(events))
	for _, ev := range events {
		var xp *event.XpEarned
		switch e := ev.(type) {
		case *event.XpEarned:
			xp = e
		case event.XpEarned:
			xp = &e
		}
		if xp == nil {
			continue
		}
		cmds = append(
			cmds,
			ProjectLeaderboardScoreCommand{
				UserID:    xp.UserID,
				Xp:        xp.NewXp,
				UpdatedAt: xp.EarnedAt,
			},
		)
	}
	return cmds
}
