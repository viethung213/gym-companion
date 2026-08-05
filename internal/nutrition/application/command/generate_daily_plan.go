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
	eventPublisher port.EventPublisher
	tdeeCalculator *service.TDEECalculator
	menuGenerator  *service.MenuGenerator
}

func NewGenerateDailyPlanHandler(
	planRepo repository.NutritionPlanRepository,
	historyRepo repository.MealHistoryRepository,
	eventPublisher port.EventPublisher,
	tdeeCalculator *service.TDEECalculator,
	menuGenerator *service.MenuGenerator,
) *GenerateDailyPlanHandler {
	return &GenerateDailyPlanHandler{
		planRepo:       planRepo,
		historyRepo:    historyRepo,
		eventPublisher: eventPublisher,
		tdeeCalculator: tdeeCalculator,
		menuGenerator:  menuGenerator,
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

	if saveErr := h.planRepo.Save(ctx, plan); saveErr != nil {
		return nil, fmt.Errorf("generate daily plan save: %w", saveErr)
	}

	if h.eventPublisher != nil {
		ev := domainEvent.NewNutritionPlanGeneratedEvent(
			plan.ID(),
			plan.UserID(),
			plan.PlanDate(),
			time.Now(),
		)
		_ = h.eventPublisher.PublishEvents(ctx, []any{ev})
	}

	return plan, nil
}
