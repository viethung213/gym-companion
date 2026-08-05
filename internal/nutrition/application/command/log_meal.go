package command

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/nutrition/application/port"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	domainEvent "github.com/viethung213/gym-companion/internal/nutrition/domain/event"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

type LogMealCommand struct {
	UserID          string
	PlanDate        time.Time
	PlannedOptionID string
	MealType        string
	MealName        string
	Portion         string
	Calories        float64
	Protein         float64
	Carbs           float64
	Fat             float64
}

type LogMealHandler struct {
	planRepo       repository.NutritionPlanRepository
	historyRepo    repository.MealHistoryRepository
	eventPublisher port.EventPublisher
	aiService      repository.AIService
}

func NewLogMealHandler(
	planRepo repository.NutritionPlanRepository,
	historyRepo repository.MealHistoryRepository,
	eventPublisher port.EventPublisher,
	aiService repository.AIService,
) *LogMealHandler {
	return &LogMealHandler{
		planRepo:       planRepo,
		historyRepo:    historyRepo,
		eventPublisher: eventPublisher,
		aiService:      aiService,
	}
}

func (h *LogMealHandler) Handle(ctx context.Context, cmd LogMealCommand) (*aggregate.MealLog, error) {
	now := time.Now()
	var calories, protein, carbs, fat float64
	mealName := cmd.MealName
	mealType := cmd.MealType

	if cmd.PlannedOptionID != "" {
		plan, planErr := h.planRepo.FindByUserIDAndDate(ctx, cmd.UserID, cmd.PlanDate)
		if planErr == nil && plan != nil {
			option, markErr := plan.MarkOptionLogged(cmd.PlannedOptionID)
			if markErr == nil && option != nil {
				mealName = option.MealName()
				calories = option.Calories()
				protein = option.ProteinGrams()
				carbs = option.CarbGrams()
				fat = option.FatGrams()
				_ = h.planRepo.Update(ctx, plan)
			}
		}
	}

	if cmd.PlannedOptionID == "" && cmd.Calories <= 0 {
		if h.aiService != nil {
			aiRes, aiErr := h.aiService.EstimateNutrient(ctx, cmd.MealName, cmd.Portion)
			if aiErr == nil && aiRes != nil {
				calories = aiRes.Calories
				protein = aiRes.Protein
				carbs = aiRes.Carbs
				fat = aiRes.Fat
			}
		}
		if calories <= 0 {
			calories = 350.0
			protein = 20.0
			carbs = 40.0
			fat = 10.0
		}
	} else if cmd.PlannedOptionID == "" {
		calories = cmd.Calories
		protein = cmd.Protein
		carbs = cmd.Carbs
		fat = cmd.Fat
	}

	history, histErr := h.historyRepo.FindByUserID(ctx, cmd.UserID)
	if histErr != nil || history == nil {
		history = aggregate.NewMealHistory(uuid.New().String(), cmd.UserID, vo.NewLockoutRegistry(nil))
	}

	mealLog := aggregate.NewMealLog(
		uuid.New().String(),
		history.ID(),
		cmd.UserID,
		mealType,
		mealName,
		cmd.Portion,
		calories,
		protein,
		carbs,
		fat,
		now,
	)

	history.AddMealLog(mealLog)

	history.ApplyLockoutRule(vo.LockoutTypeProtein, mealName, vo.DurationProtein, now)
	history.ApplyLockoutRule(vo.LockoutTypeCategory, mealType, vo.DurationCategory, now)

	if saveErr := h.historyRepo.Save(ctx, history); saveErr != nil {
		return nil, fmt.Errorf("log meal save: %w", saveErr)
	}

	if h.eventPublisher != nil {
		ev1 := domainEvent.NewMealLoggedEvent(
			mealLog.ID(),
			cmd.UserID,
			mealType,
			mealName,
			calories,
			now,
		)
		ev2 := domainEvent.NewLockoutAppliedEvent(
			cmd.UserID,
			vo.LockoutTypeProtein,
			mealName,
			now.Add(vo.DurationProtein),
		)
		_ = h.eventPublisher.PublishEvents(ctx, []any{ev1, ev2})
	}

	return &mealLog, nil
}
