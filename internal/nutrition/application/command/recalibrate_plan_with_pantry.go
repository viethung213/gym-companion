package command

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/application/port"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/domainerror"
	domainEvent "github.com/viethung213/gym-companion/internal/nutrition/domain/event"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
)

type RecalibratePlanWithPantryCommand struct {
	UserID               string
	PlanDate             time.Time
	MealType             string
	AvailableIngredients []string
	UserRestrictions     []string
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

func (h *RecalibratePlanWithPantryHandler) Handle(ctx context.Context, cmd *RecalibratePlanWithPantryCommand) (*aggregate.NutritionPlan, error) {
	if cmd == nil {
		return nil, errors.New("recalibrate pantry: command cannot be nil")
	}
	if strings.TrimSpace(cmd.MealType) == "" {
		return nil, domainerror.ErrMealTypeRequired
	}

	plan, err := h.planRepo.FindByUserIDAndDate(ctx, cmd.UserID, cmd.PlanDate)
	if err != nil || plan == nil {
		return nil, fmt.Errorf("recalibrate pantry: plan not found for date %s: %w", cmd.PlanDate.Format("2006-01-02"), err)
	}

	// Kiểm tra trạng thái bữa ăn trên Aggregate trước khi gọi AI để tránh lãng phí request
	isLogged, checkErr := plan.IsMealLogged(cmd.MealType)
	if checkErr != nil {
		return nil, fmt.Errorf("recalibrate pantry check meal: %w", checkErr)
	}
	if isLogged {
		return nil, domainerror.ErrMealAlreadyLogged
	}

	// External AI Generation is intentionally executed outside the DB transaction to prevent connection bottleneck
	options, genErr := h.menuGenerator.GenerateMealOptionsWithPantry(
		ctx,
		cmd.UserID,
		cmd.PlanDate,
		cmd.MealType,
		plan.CalorieAllocation(),
		cmd.AvailableIngredients,
		cmd.UserRestrictions,
	)
	if genErr != nil {
		return nil, fmt.Errorf("recalibrate pantry generate: %w", genErr)
	}

	if err := plan.UpdateMealOptions(cmd.MealType, options); err != nil {
		return nil, fmt.Errorf("recalibrate pantry update meal options: %w", err)
	}

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
