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

// Compile-time check.
var _ *aggregate.NutritionPlan = nil

// GenerateDailyPlanCommand yêu cầu sinh thực đơn ngày mới cho người dùng.
type GenerateDailyPlanCommand struct {
	UserID            string
	PlanDate          time.Time
	BiologicalMetrics service.BiologicalMetrics
	NutritionGoal     vo.NutritionGoal // mục tiêu dài hạn ảnh hưởng đến macro ratio
	UserRestrictions  []string
}

type GenerateDailyPlanHandler struct {
	planRepo       repository.NutritionPlanRepository
	historyRepo    repository.MealHistoryRepository
	eventRecorder  port.EventRecorder
	tdeeCalculator *service.TDEECalculator
	menuGenerator  *service.MenuGenerator
	txManager      port.TransactionManager
}

func NewGenerateDailyPlanHandler(
	planRepo repository.NutritionPlanRepository,
	historyRepo repository.MealHistoryRepository,
	eventRecorder port.EventRecorder,
	tdeeCalculator *service.TDEECalculator,
	menuGenerator *service.MenuGenerator,
	txManager ...port.TransactionManager,
) *GenerateDailyPlanHandler {
	var tm port.TransactionManager
	if len(txManager) > 0 {
		tm = txManager[0]
	}
	return &GenerateDailyPlanHandler{
		planRepo:       planRepo,
		historyRepo:    historyRepo,
		eventRecorder:  eventRecorder,
		tdeeCalculator: tdeeCalculator,
		menuGenerator:  menuGenerator,
		txManager:      tm,
	}
}

//nolint:gocritic // cmd is passed by value per command pattern
func (h *GenerateDailyPlanHandler) Handle(ctx context.Context, cmd GenerateDailyPlanCommand) (*aggregate.NutritionPlan, error) {
	existing, err := h.planRepo.FindByUserIDAndDate(ctx, cmd.UserID, cmd.PlanDate)
	if err == nil && existing != nil {
		return existing, nil
	}

	goal := cmd.NutritionGoal
	if goal == "" {
		goal = vo.GoalMaintenance
	}

	allocation, calcErr := h.tdeeCalculator.CalculateBaseTDEE(cmd.BiologicalMetrics, goal)
	if calcErr != nil {
		return nil, fmt.Errorf("generate daily plan: %w", calcErr)
	}

	var lockouts vo.LockoutRegistry
	history, histErr := h.historyRepo.FindByUserID(ctx, cmd.UserID)
	if histErr == nil && history != nil {
		lockouts = history.LockoutRegistry()
	}

	// External AI Generation is intentionally executed outside the DB transaction to prevent connection bottleneck
	plan, genErr := h.menuGenerator.GenerateDailyPlan(
		ctx,
		cmd.UserID,
		cmd.PlanDate,
		allocation,
		lockouts,
		cmd.UserRestrictions,
	)
	if genErr != nil {
		return nil, fmt.Errorf("generate daily plan: %w", genErr)
	}

	savedSchedules, _ := h.planRepo.GetUserMealSchedules(ctx, cmd.UserID)
	if len(savedSchedules) > 0 {
		plan.UpdateMealSchedule(savedSchedules)
	}

	// Wrap local DB persistence and outbox event recording in an atomic transaction
	saveFunc := func(txCtx context.Context) error {
		if saveErr := h.planRepo.Save(txCtx, plan); saveErr != nil {
			return fmt.Errorf("generate daily plan save: %w", saveErr)
		}

		if h.eventRecorder != nil {
			ev := domainEvent.NewNutritionPlanGeneratedEvent(
				plan.ID(),
				plan.UserID(),
				plan.PlanDate(),
				time.Now(),
			)
			if err := h.eventRecorder.RecordEvents(txCtx, []any{ev}); err != nil {
				return fmt.Errorf("record nutrition plan generated event: %w", err)
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
