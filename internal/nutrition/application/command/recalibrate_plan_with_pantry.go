package command

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/application/port"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	domainEvent "github.com/viethung213/gym-companion/internal/nutrition/domain/event"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

type RecalibratePlanWithPantryCommand struct {
	UserID               string
	PlanDate             time.Time
	AvailableIngredients []string
}

type RecalibratePlanWithPantryHandler struct {
	planRepo      repository.NutritionPlanRepository
	historyRepo   repository.MealHistoryRepository
	eventRecorder port.EventRecorder
	menuGenerator *service.MenuGenerator
	txManager     port.TransactionManager
}

func NewRecalibratePlanWithPantryHandler(
	planRepo repository.NutritionPlanRepository,
	historyRepo repository.MealHistoryRepository,
	eventRecorder port.EventRecorder,
	menuGenerator *service.MenuGenerator,
	txManager ...port.TransactionManager,
) *RecalibratePlanWithPantryHandler {
	var tm port.TransactionManager
	if len(txManager) > 0 {
		tm = txManager[0]
	}
	return &RecalibratePlanWithPantryHandler{
		planRepo:      planRepo,
		historyRepo:   historyRepo,
		eventRecorder: eventRecorder,
		menuGenerator: menuGenerator,
		txManager:     tm,
	}
}

func (h *RecalibratePlanWithPantryHandler) Handle(ctx context.Context, cmd RecalibratePlanWithPantryCommand) (*aggregate.NutritionPlan, error) {
	plan, err := h.planRepo.FindByUserIDAndDate(ctx, cmd.UserID, cmd.PlanDate)
	if err != nil || plan == nil {
		return nil, fmt.Errorf("recalibrate pantry: plan not found for date %s: %w", cmd.PlanDate.Format("2006-01-02"), err)
	}

	var lockouts vo.LockoutRegistry
	history, histErr := h.historyRepo.FindByUserID(ctx, cmd.UserID)
	if histErr == nil && history != nil {
		lockouts = history.LockoutRegistry()
	}

	// External AI Generation is intentionally executed outside the DB transaction to prevent connection bottleneck
	recalibratedPlan, genErr := h.menuGenerator.GeneratePlanWithPantry(
		ctx,
		cmd.UserID,
		cmd.PlanDate,
		plan.CalorieAllocation(),
		lockouts,
		cmd.AvailableIngredients,
	)
	if genErr != nil {
		return nil, fmt.Errorf("recalibrate pantry generate: %w", genErr)
	}

	plan.UpdateRemainingUnconsumedMeals(recalibratedPlan.DailyMeals(), plan.CalorieAllocation())

	// Wrap local DB update and outbox event recording in an atomic transaction
	saveFunc := func(txCtx context.Context) error {
		if updateErr := h.planRepo.Update(txCtx, plan); updateErr != nil {
			return fmt.Errorf("recalibrate pantry update: %w", updateErr)
		}

		if h.eventRecorder != nil {
			ev := domainEvent.NewNutritionPlanRecalibratedEvent(
				plan.ID(),
				plan.UserID(),
				"Pantry ingredient availability recalibration",
				time.Now(),
			)
			if err := h.eventRecorder.RecordEvents(txCtx, []any{ev}); err != nil {
				return fmt.Errorf("record nutrition plan recalibrated event: %w", err)
			}
		}
		return nil
	}

	if h.txManager != nil {
		if err := h.txManager.WithTransaction(ctx, saveFunc); err != nil {
			return nil, err
		}
	} else {
		if err := saveFunc(ctx); err != nil {
			return nil, err
		}
	}

	return plan, nil
}
