package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/event"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
)

type ActivityIngestionCoordinator struct {
	xpHandler        *command.ProcessWorkoutXpHandler
	nutritionHandler *command.ProcessNutritionXpHandler
	prHandler        *command.ProcessPersonalRecordHandler
	streakHandler    *command.RecordStreakActivityHandler
	badgeHandler     *command.EvaluateBadgesHandler
	txManager        port.TransactionManager
	walletRewarder   port.WalletRewarder
}

func NewActivityIngestionCoordinator(
	xpHandler *command.ProcessWorkoutXpHandler,
	streakHandler *command.RecordStreakActivityHandler,
	badgeRepo repository.BadgeRepository,
	txManager port.TransactionManager,
	outboxWriter port.OutboxWriter,
) *ActivityIngestionCoordinator {
	return &ActivityIngestionCoordinator{
		xpHandler:     xpHandler,
		streakHandler: streakHandler,
		badgeHandler:  command.NewEvaluateBadgesHandler(badgeRepo, txManager, outboxWriter),
		txManager:     txManager,
	}
}

func (c *ActivityIngestionCoordinator) WithBadgeHandler(
	badgeHandler *command.EvaluateBadgesHandler,
) *ActivityIngestionCoordinator {
	c.badgeHandler = badgeHandler
	return c
}

func (c *ActivityIngestionCoordinator) WithWalletRewarder(
	walletRewarder port.WalletRewarder,
) *ActivityIngestionCoordinator {
	c.walletRewarder = walletRewarder
	return c
}

func (c *ActivityIngestionCoordinator) WithNutritionAndPRHandlers(
	nutritionHandler *command.ProcessNutritionXpHandler,
	prHandler *command.ProcessPersonalRecordHandler,
) *ActivityIngestionCoordinator {
	c.nutritionHandler = nutritionHandler
	c.prHandler = prHandler
	return c
}

func (c *ActivityIngestionCoordinator) IngestWorkoutSession(
	ctx context.Context,
	cmd *command.ProcessWorkoutXpCommand,
) error {
	if cmd == nil {
		return errors.New("process workout xp command is nil")
	}
	return c.runPipeline(
		ctx,
		cmd.UserID,
		cmd.ActivityDate,
		cmd.CompletedAt,
		&event.WorkoutActivityIngested{
			UserID:           cmd.UserID,
			SessionID:        cmd.SessionID,
			ActivityDate:     cmd.ActivityDate,
			TotalSets:        cmd.TotalSets,
			TotalVolumeKg:    cmd.TotalVolumeKg,
			VolumeRatio:      cmd.VolumeRatio,
			AverageFormScore: cmd.AverageFormScore,
			HasNewPR:         cmd.HasNewPR,
			CompletedAt:      cmd.CompletedAt,
		},
		func(txCtx context.Context, events []event.Event) ([]event.Event, error) {
			streakRes, err := c.streakHandler.Handle(txCtx, command.RecordStreakActivityCommand{
				UserID:       cmd.UserID,
				ActivityDate: cmd.ActivityDate,
				ActivityType: "WORKOUT",
				RecordedAt:   cmd.CompletedAt,
			})
			if err != nil {
				return nil, fmt.Errorf("coordinator step 1 (streak): %w", err)
			}
			events = append(events, streakRes.Events...)

			xpEvents, err := c.xpHandler.HandleEvents(txCtx, streakRes.CurrentStreak, events)
			if err != nil {
				return nil, fmt.Errorf("coordinator step 2 (xp): %w", err)
			}
			return append(events, xpEvents...), nil
		},
	)
}

func (c *ActivityIngestionCoordinator) IngestNutritionActivity(
	ctx context.Context,
	cmd command.ProcessNutritionActivityCommand,
) error {
	return c.runPipeline(
		ctx,
		cmd.UserID,
		cmd.ActivityDate,
		cmd.RecordedAt,
		&event.NutritionActivityIngested{
			UserID:       cmd.UserID,
			ActivityDate: cmd.ActivityDate,
			RecordedAt:   cmd.RecordedAt,
		},
		func(txCtx context.Context, events []event.Event) ([]event.Event, error) {
			streakEvents, err := c.streakHandler.HandleEvents(txCtx, events)
			if err != nil {
				return nil, fmt.Errorf("coordinator step 1 (nutrition streak): %w", err)
			}
			events = append(events, streakEvents...)

			xpEvents, err := c.nutritionHandler.HandleEvents(txCtx, events)
			if err != nil {
				return nil, fmt.Errorf("coordinator step 2 (nutrition xp): %w", err)
			}
			return append(events, xpEvents...), nil
		},
	)
}

//nolint:gocritic // hugeParam: command struct is an immutable value object
func (c *ActivityIngestionCoordinator) IngestPersonalRecord(
	ctx context.Context,
	cmd command.ProcessPersonalRecordCommand,
) error {
	if c.prHandler == nil {
		return nil
	}
	return c.runPipeline(
		ctx,
		cmd.UserID,
		cmd.ActivityDate,
		cmd.AchievedAt,
		&event.PersonalRecordIngested{
			UserID:       cmd.UserID,
			ExerciseID:   cmd.ExerciseID,
			Weight:       cmd.Weight,
			Reps:         cmd.Reps,
			ActivityDate: cmd.ActivityDate,
			AchievedAt:   cmd.AchievedAt,
		},
		func(txCtx context.Context, events []event.Event) ([]event.Event, error) {
			xpEvents, err := c.prHandler.HandleEvents(txCtx, events)
			if err != nil {
				return nil, fmt.Errorf("coordinator step 2 (pr xp): %w", err)
			}
			return append(events, xpEvents...), nil
		},
	)
}

func (c *ActivityIngestionCoordinator) runPipeline(
	ctx context.Context,
	userID, activityDate string,
	occurredAt time.Time,
	seedEvent event.Event,
	progressionSteps func(txCtx context.Context, events []event.Event) ([]event.Event, error),
) error {
	return c.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		pipelineEvents, err := progressionSteps(txCtx, []event.Event{seedEvent})
		if err != nil {
			return err
		}

		if c.badgeHandler != nil {
			badgeEvents, err := c.badgeHandler.HandleEvents(txCtx, userID, occurredAt, pipelineEvents)
			if err != nil {
				return fmt.Errorf("coordinator step 3 (badges): %w", err)
			}
			pipelineEvents = append(pipelineEvents, badgeEvents...)
		}

		if c.walletRewarder != nil {
			if _, err := c.walletRewarder.HandleEvents(txCtx, userID, activityDate, occurredAt, pipelineEvents); err != nil {
				return fmt.Errorf("coordinator step 4 (wallet batch): %w", err)
			}
		}

		return nil
	})
}
