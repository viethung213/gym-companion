package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
)

// ActivityIngestionCoordinator orchestrates atomic multi-aggregate ingestion (Architecture #5).
// It executes Step 1 (Streak) -> Step 2 (XP) -> Step 3 (Badges) -> Step 4 (FitCoins Wallet)
// within a single atomic database transaction.
// Strict lock acquisition order (user_streak -> user_xp -> user_badges -> user_wallet) eliminates
// deadlocks while guaranteeing 100% ACID consistency.
type ActivityIngestionCoordinator struct {
	xpHandler      *command.ProcessWorkoutXpHandler
	streakHandler  *command.RecordStreakActivityHandler
	userStreakRepo repository.UserStreakRepository
	badgeRepo      repository.BadgeRepository
	txManager      port.TransactionManager
	outboxWriter   port.OutboxWriter
	walletRewarder port.WalletRewarder
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

// WithWalletRewarder attaches an optional WalletRewarder for Step 4 FitCoins rewards.
func (c *ActivityIngestionCoordinator) WithWalletRewarder(
	walletRewarder port.WalletRewarder,
) *ActivityIngestionCoordinator {
	c.walletRewarder = walletRewarder
	return c
}

// IngestWorkoutSession coordinates Step 1 (Streak), Step 2 (XP with up-to-date multiplier),
// Step 3 (Badges), and Step 4 (FitCoins Wallet) atomically inside a single database transaction.
func (c *ActivityIngestionCoordinator) IngestWorkoutSession(
	ctx context.Context,
	cmd *command.ProcessWorkoutXpCommand,
) error {
	if cmd == nil {
		return errors.New("process workout xp command is nil")
	}
	return c.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		// --- Step 1: Streak Evaluation ---
		streakCmd := command.RecordStreakActivityCommand{
			UserID:       cmd.UserID,
			ActivityDate: cmd.ActivityDate,
			ActivityType: "WORKOUT",
			RecordedAt:   cmd.CompletedAt,
		}
		streakResult, err := c.streakHandler.Handle(txCtx, streakCmd)
		if err != nil {
			return fmt.Errorf("coordinator step 1 (streak): %w", err)
		}

		// Use updated streak to calculate XP with the proper multiplier
		cmd.CurrentStreak = streakResult.CurrentStreak

		// --- Step 2: XP Progression ---
		xpResult, err := c.xpHandler.Handle(txCtx, cmd)
		if err != nil {
			return fmt.Errorf("coordinator step 2 (xp): %w", err)
		}

		// --- Step 3: Badges Evaluation ---
		unlockedBadges, err := c.evaluateBadges(
			txCtx,
			cmd.UserID,
			xpResult,
			streakResult,
			cmd.HasNewPR,
			cmd.CompletedAt,
		)
		if err != nil {
			return fmt.Errorf("coordinator step 3 (badges): %w", err)
		}

		// --- Step 4: FitCoins Wallet Rewards (last in lock order: user_wallet) ---
		if err := c.awardWorkoutCoins(txCtx, cmd, streakResult, unlockedBadges); err != nil {
			return fmt.Errorf("coordinator step 4 (wallet): %w", err)
		}

		return nil
	})
}

func (c *ActivityIngestionCoordinator) awardWorkoutCoins(
	ctx context.Context,
	cmd *command.ProcessWorkoutXpCommand,
	streakRes *command.RecordStreakActivityResult,
	unlockedBadges []string,
) error {
	if c.walletRewarder == nil {
		return nil
	}

	sessionRef := cmd.SessionID
	// 1. Daily Workout Reward (+10 FitCoins, Max 1/day)
	if _, err := c.walletRewarder.AwardCoins(ctx, port.AwardCoinsInput{
		UserID:         cmd.UserID,
		Amount:         vo.WorkoutCoinReward,
		Reason:         vo.ReasonEarnWorkout,
		ReferenceID:    &sessionRef,
		IdempotencyKey: fmt.Sprintf("WORKOUT:%s:%s", cmd.UserID, cmd.ActivityDate),
		ActivityDate:   cmd.ActivityDate,
		OccurredAt:     cmd.CompletedAt,
	}); err != nil {
		return fmt.Errorf("award workout coins: %w", err)
	}

	// 2. Daily Personal Record Reward (+20 FitCoins, Max 1/day)
	if cmd.HasNewPR {
		if _, err := c.walletRewarder.AwardCoins(ctx, port.AwardCoinsInput{
			UserID:         cmd.UserID,
			Amount:         vo.PersonalRecordCoinReward,
			Reason:         vo.ReasonEarnPR,
			ReferenceID:    &sessionRef,
			IdempotencyKey: fmt.Sprintf("PR:%s:%s", cmd.UserID, cmd.ActivityDate),
			ActivityDate:   cmd.ActivityDate,
			OccurredAt:     cmd.CompletedAt,
		}); err != nil {
			return fmt.Errorf("award pr coins: %w", err)
		}
	}

	// 3. Streak Milestone Reward (+30/+75/+150/+250/+500/+1500 FitCoins)
	if streakRes != nil && streakRes.Advanced {
		milestoneReward := vo.StreakMilestoneCoinReward(streakRes.CurrentStreak)
		if milestoneReward > 0 {
			milestoneRef := fmt.Sprintf("STREAK_%d", streakRes.CurrentStreak)
			if _, err := c.walletRewarder.AwardCoins(ctx, port.AwardCoinsInput{
				UserID:         cmd.UserID,
				Amount:         milestoneReward,
				Reason:         vo.ReasonEarnStreakMilestone,
				ReferenceID:    &milestoneRef,
				IdempotencyKey: fmt.Sprintf("STREAK_MILESTONE:%s:%d:%s", cmd.UserID, streakRes.CurrentStreak, cmd.ActivityDate),
				ActivityDate:   cmd.ActivityDate,
				OccurredAt:     cmd.CompletedAt,
			}); err != nil {
				return fmt.Errorf("award streak milestone coins: %w", err)
			}
		}
	}

	// 4. Newly Unlocked Badges Reward (+15 FitCoins per badge)
	for _, badgeCode := range unlockedBadges {
		codeRef := badgeCode
		if _, err := c.walletRewarder.AwardCoins(ctx, port.AwardCoinsInput{
			UserID:         cmd.UserID,
			Amount:         vo.BadgeUnlockedCoinReward,
			Reason:         vo.ReasonEarnBadge,
			ReferenceID:    &codeRef,
			IdempotencyKey: fmt.Sprintf("BADGE:%s:%s", cmd.UserID, badgeCode),
			ActivityDate:   cmd.ActivityDate,
			OccurredAt:     cmd.CompletedAt,
		}); err != nil {
			return fmt.Errorf("award badge coins (%s): %w", badgeCode, err)
		}
	}

	return nil
}

func (c *ActivityIngestionCoordinator) evaluateBadges(
	ctx context.Context,
	userID string,
	xpRes *command.ProcessWorkoutXpResult,
	streakRes *command.RecordStreakActivityResult,
	hasNewPR bool,
	now time.Time,
) ([]string, error) {
	if c.badgeRepo == nil || c.txManager == nil || c.outboxWriter == nil {
		return nil, nil
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

	badgesToUnlock := make([]*aggregate.UserBadge, 0, len(candidateCodes))
	unlockedCodes := make([]string, 0, len(candidateCodes))
	outboxEvents := make([]any, 0, len(candidateCodes))

	for _, code := range candidateCodes {
		has, err := c.badgeRepo.HasBadge(ctx, userID, code)
		if err != nil {
			return nil, fmt.Errorf("check badge %s for user %s: %w", code, userID, err)
		}
		if has {
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
			return nil, fmt.Errorf("unlock badge %s for user %s: %w", code, userID, err)
		}

		badgesToUnlock = append(badgesToUnlock, badge)
		unlockedCodes = append(unlockedCodes, code)
		outboxEvents = append(outboxEvents, ev)
	}

	if len(badgesToUnlock) == 0 {
		return nil, nil
	}

	// Execute all unlocked badges in the active transaction
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
		return nil, fmt.Errorf("save and write badge events: %w", err)
	}

	return unlockedCodes, nil
}
