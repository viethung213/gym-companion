package command

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
)

type ProcessNutritionActivityCommand struct {
	UserID       string
	ActivityDate string
	RecordedAt   time.Time
}

type ProcessNutritionActivityHandler struct {
	streakHandler    *RecordStreakActivityHandler
	nutritionHandler *ProcessNutritionXpHandler
	txManager        port.TransactionManager
	walletRewarder   port.WalletRewarder
}

func NewProcessNutritionActivityHandler(
	streakHandler *RecordStreakActivityHandler,
	nutritionHandler *ProcessNutritionXpHandler,
	txManager port.TransactionManager,
) *ProcessNutritionActivityHandler {
	return &ProcessNutritionActivityHandler{
		streakHandler:    streakHandler,
		nutritionHandler: nutritionHandler,
		txManager:        txManager,
	}
}

// WithWalletRewarder attaches an optional WalletRewarder to credit daily nutrition FitCoins.
func (h *ProcessNutritionActivityHandler) WithWalletRewarder(
	walletRewarder port.WalletRewarder,
) *ProcessNutritionActivityHandler {
	h.walletRewarder = walletRewarder
	return h
}

func (h *ProcessNutritionActivityHandler) Handle(ctx context.Context, cmd ProcessNutritionActivityCommand) error {
	if h.txManager == nil {
		return h.execute(ctx, cmd)
	}

	return h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		return h.execute(txCtx, cmd)
	})
}

func (h *ProcessNutritionActivityHandler) execute(ctx context.Context, cmd ProcessNutritionActivityCommand) error {
	var streakRes *RecordStreakActivityResult
	if h.streakHandler != nil {
		var err error
		streakRes, err = h.streakHandler.Handle(ctx, RecordStreakActivityCommand{
			UserID:       cmd.UserID,
			ActivityDate: cmd.ActivityDate,
			ActivityType: "NUTRITION",
			RecordedAt:   cmd.RecordedAt,
		})
		if err != nil {
			return fmt.Errorf("record nutrition streak activity: %w", err)
		}
	}

	if h.nutritionHandler != nil {
		_, err := h.nutritionHandler.Handle(ctx, ProcessNutritionXpCommand{
			UserID:       cmd.UserID,
			ActivityDate: cmd.ActivityDate,
			AchievedAt:   cmd.RecordedAt,
		})
		if err != nil {
			return fmt.Errorf("process nutrition xp: %w", err)
		}
	}

	if h.walletRewarder != nil {
		if _, err := h.walletRewarder.AwardCoins(ctx, port.AwardCoinsInput{
			UserID:         cmd.UserID,
			Amount:         vo.NutritionDailyCoinReward,
			Reason:         vo.ReasonEarnNutrition,
			ReferenceID:    nil,
			IdempotencyKey: fmt.Sprintf("NUTRITION:%s:%s", cmd.UserID, cmd.ActivityDate),
			ActivityDate:   cmd.ActivityDate,
			OccurredAt:     cmd.RecordedAt,
		}); err != nil {
			return fmt.Errorf("award nutrition coins: %w", err)
		}

		if streakRes != nil && streakRes.Advanced {
			milestoneReward := vo.StreakMilestoneCoinReward(streakRes.CurrentStreak)
			if milestoneReward > 0 {
				milestoneRef := fmt.Sprintf("STREAK_%d", streakRes.CurrentStreak)
				if _, err := h.walletRewarder.AwardCoins(ctx, port.AwardCoinsInput{
					UserID:         cmd.UserID,
					Amount:         milestoneReward,
					Reason:         vo.ReasonEarnStreakMilestone,
					ReferenceID:    &milestoneRef,
					IdempotencyKey: fmt.Sprintf("STREAK_MILESTONE:%s:%d:%s", cmd.UserID, streakRes.CurrentStreak, cmd.ActivityDate),
					ActivityDate:   cmd.ActivityDate,
					OccurredAt:     cmd.RecordedAt,
				}); err != nil {
					return fmt.Errorf("award nutrition streak milestone coins: %w", err)
				}
			}
		}
	}

	return nil
}
