package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
)

// ActivityIngestionCoordinator orchestrates sequential multi-aggregate ingestion (Architecture #5).
// It runs 2 separate, short atomic local transactions (user_xp -> user_streak) to eliminate deadlocks.
type ActivityIngestionCoordinator struct {
	xpHandler      *command.ProcessWorkoutXpHandler
	streakHandler  *command.RecordStreakActivityHandler
	userStreakRepo repository.UserStreakRepository
	badgeRepo      repository.BadgeRepository
	txManager      port.TransactionManager
	outboxWriter   port.OutboxWriter
}

// NewActivityIngestionCoordinator constructs ActivityIngestionCoordinator.
func NewActivityIngestionCoordinator(
	xpHandler *command.ProcessWorkoutXpHandler,
	streakHandler *command.RecordStreakActivityHandler,
	userStreakRepo repository.UserStreakRepository,
	badgeRepo repository.BadgeRepository,
	txManager port.TransactionManager,
	outboxWriter port.OutboxWriter,
) *ActivityIngestionCoordinator {
	return &ActivityIngestionCoordinator{
		xpHandler:      xpHandler,
		streakHandler:  streakHandler,
		userStreakRepo: userStreakRepo,
		badgeRepo:      badgeRepo,
		txManager:      txManager,
		outboxWriter:   outboxWriter,
	}
}

// IngestWorkoutSession coordinates Step 1 (Streak), Step 2 (XP with up-to-date multiplier), and Step 3 (Badges) sequentially.
func (c *ActivityIngestionCoordinator) IngestWorkoutSession(
	ctx context.Context,
	cmd command.ProcessWorkoutXpCommand,
) error {
	// --- Step 1: Atomic Streak Evaluation ---
	// Update streak first so today's session advances the streak count immediately.
	streakCmd := command.RecordStreakActivityCommand{
		UserID:       cmd.UserID,
		ActivityDate: cmd.ActivityDate,
		ActivityType: "WORKOUT",
		RecordedAt:   cmd.CompletedAt,
	}
	streakResult, err := c.streakHandler.Handle(ctx, streakCmd)
	if err != nil {
		return fmt.Errorf("coordinator step 1 (streak): %w", err)
	}

	// Use updated streak to calculate XP with the proper multiplier
	cmd.CurrentStreak = streakResult.CurrentStreak

	// --- Step 2: Atomic XP Progression ---
	xpResult, err := c.xpHandler.Handle(ctx, cmd)
	if err != nil {
		return fmt.Errorf("coordinator step 2 (xp): %w", err)
	}

	// --- Step 3: Badges Evaluation ---
	c.evaluateBadges(ctx, cmd.UserID, xpResult, streakResult, cmd.HasNewPR, cmd.CompletedAt)

	return nil
}

func (c *ActivityIngestionCoordinator) evaluateBadges(
	ctx context.Context,
	userID string,
	xpRes *command.ProcessWorkoutXpResult,
	streakRes *command.RecordStreakActivityResult,
	hasNewPR bool,
	now time.Time,
) {
	if c.badgeRepo == nil || c.txManager == nil || c.outboxWriter == nil {
		return
	}

	var candidateCodes []string

	// Badge 1: First Workout
	candidateCodes = append(candidateCodes, "FIRST_WORKOUT")

	// Badge 2: PR Breaker
	if hasNewPR {
		candidateCodes = append(candidateCodes, "PR_BREAKER")
	}

	// Badge 3: Streak Milestones (Exact catalog codes: STREAK_7, STREAK_14, STREAK_30, STREAK_50, STREAK_100, STREAK_365)
	if streakRes != nil {
		switch streakRes.CurrentStreak {
		case 7:
			candidateCodes = append(candidateCodes, "STREAK_7")
		case 14:
			candidateCodes = append(candidateCodes, "STREAK_14")
		case 30:
			candidateCodes = append(candidateCodes, "STREAK_30")
		case 50:
			candidateCodes = append(candidateCodes, "STREAK_50")
		case 100:
			candidateCodes = append(candidateCodes, "STREAK_100")
		case 365:
			candidateCodes = append(candidateCodes, "STREAK_365")
		}
	}

	// Badge 4: Level Milestones using range checking (oldLevel < X && newLevel >= X)
	if xpRes != nil {
		oldLvl := xpRes.OldLevel
		newLvl := xpRes.NewLevel
		if oldLvl < 10 && newLvl >= 10 {
			candidateCodes = append(candidateCodes, "LEVEL_10")
		}
		if oldLvl < 25 && newLvl >= 25 {
			candidateCodes = append(candidateCodes, "LEVEL_25")
		}
		if oldLvl < 50 && newLvl >= 50 {
			candidateCodes = append(candidateCodes, "LEVEL_50")
		}
		if oldLvl < 100 && newLvl >= 100 {
			candidateCodes = append(candidateCodes, "LEVEL_100")
		}
	}

	var badgesToUnlock []*aggregate.UserBadge
	var outboxEvents []any

	for _, code := range candidateCodes {
		has, err := c.badgeRepo.HasBadge(ctx, userID, code)
		if err != nil || has {
			continue
		}

		def, _ := vo.GetBadgeDefinition(code)
		badge, ev, err := aggregate.UnlockBadge(
			uuid.NewString(),
			userID,
			code,
			def.Name,
			def.Category,
			nil,
			now,
		)
		if err != nil {
			log.Printf("[Gamification] Error constructing badge %s for user %s: %v", code, userID, err)
			continue
		}

		badgesToUnlock = append(badgesToUnlock, badge)
		outboxEvents = append(outboxEvents, ev)
	}

	if len(badgesToUnlock) == 0 {
		return
	}

	// Execute all unlocked badges in a single, atomic local transaction
	err := c.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := c.badgeRepo.SaveBatch(txCtx, badgesToUnlock); err != nil {
			return fmt.Errorf("batch save badges: %w", err)
		}
		if err := c.outboxWriter.WriteEvents(txCtx, "user_badges", userID, outboxEvents); err != nil {
			return fmt.Errorf("write badge outbox events: %w", err)
		}
		return nil
	})
	if err != nil {
		log.Printf("[Gamification] Failed to batch unlock badges for user %s: %v", userID, err)
	}
}
