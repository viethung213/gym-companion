package command

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/application/port"
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

func (h *ProcessNutritionActivityHandler) Handle(ctx context.Context, cmd ProcessNutritionActivityCommand) error {
	if h.txManager == nil {
		return h.execute(ctx, cmd)
	}

	return h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		return h.execute(txCtx, cmd)
	})
}

func (h *ProcessNutritionActivityHandler) execute(ctx context.Context, cmd ProcessNutritionActivityCommand) error {
	if h.streakHandler != nil {
		_, err := h.streakHandler.Handle(ctx, RecordStreakActivityCommand{
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

	return nil
}
