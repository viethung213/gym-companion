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
	planRepo       repository.NutritionPlanRepository
	historyRepo    repository.MealHistoryRepository
	eventPublisher port.EventPublisher
	menuGenerator  *service.MenuGenerator
}

func NewRecalibratePlanWithPantryHandler(
	planRepo repository.NutritionPlanRepository,
	historyRepo repository.MealHistoryRepository,
	eventPublisher port.EventPublisher,
	menuGenerator *service.MenuGenerator,
) *RecalibratePlanWithPantryHandler {
	return &RecalibratePlanWithPantryHandler{
		planRepo:       planRepo,
		historyRepo:    historyRepo,
		eventPublisher: eventPublisher,
		menuGenerator:  menuGenerator,
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

	userRestrictions := make([]string, 0)
	if len(cmd.AvailableIngredients) > 0 {
		userRestrictions = append(userRestrictions, cmd.AvailableIngredients...)
	}

	recalibratedPlan, genErr := h.menuGenerator.GenerateDailyPlan(
		ctx,
		cmd.UserID,
		cmd.PlanDate,
		plan.CalorieAllocation(),
		lockouts,
		userRestrictions,
	)
	if genErr != nil {
		return nil, fmt.Errorf("recalibrate pantry generate: %w", genErr)
	}

	plan.UpdateRemainingUnconsumedMeals(recalibratedPlan.DailyMeals(), plan.CalorieAllocation())

	if updateErr := h.planRepo.Update(ctx, plan); updateErr != nil {
		return nil, fmt.Errorf("recalibrate pantry update: %w", updateErr)
	}

	if h.eventPublisher != nil {
		ev := domainEvent.NewNutritionPlanRecalibratedEvent(
			plan.ID(),
			plan.UserID(),
			"Pantry ingredient availability recalibration",
			time.Now(),
		)
		_ = h.eventPublisher.PublishEvents(ctx, []any{ev})
	}

	return plan, nil
}
