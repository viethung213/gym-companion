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
	planRepo      repository.NutritionPlanRepository
	historyRepo   repository.MealHistoryRepository
	eventRecorder port.EventRecorder
	aiService     repository.AIService
	txManager     port.TransactionManager
}

func NewLogMealHandler(
	planRepo repository.NutritionPlanRepository,
	historyRepo repository.MealHistoryRepository,
	eventRecorder port.EventRecorder,
	aiService repository.AIService,
	txManager ...port.TransactionManager,
) *LogMealHandler {
	var tm port.TransactionManager
	if len(txManager) > 0 {
		tm = txManager[0]
	}
	return &LogMealHandler{
		planRepo:      planRepo,
		historyRepo:   historyRepo,
		eventRecorder: eventRecorder,
		aiService:     aiService,
		txManager:     tm,
	}
}

//nolint:gocritic // cmd is passed by value per command pattern
func (h *LogMealHandler) Handle(ctx context.Context, cmd LogMealCommand) (*aggregate.MealLog, error) {
	now := time.Now()
	var calories, protein, carbs, fat float64
	mealName := cmd.MealName
	mealType := cmd.MealType
	var planToUpdate *aggregate.NutritionPlan

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
				planToUpdate = plan
			}
		}
	}

	// External AI estimation is executed outside DB transaction to avoid blocking connection pool
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
		history = aggregate.NewMealHistory(uuid.New().String(), cmd.UserID)
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

	// Wrap local DB persistence and outbox event recording in an atomic transaction
	saveFunc := func(txCtx context.Context) error {
		if planToUpdate != nil {
			if err := h.planRepo.Update(txCtx, planToUpdate); err != nil {
				return fmt.Errorf("update plan option logged: %w", err)
			}
		}

		if saveErr := h.historyRepo.Save(txCtx, history); saveErr != nil {
			return fmt.Errorf("log meal save: %w", saveErr)
		}

		if h.eventRecorder != nil {
			ev1 := domainEvent.NewMealLoggedEvent(
				mealLog.ID(),
				cmd.UserID,
				mealType,
				mealName,
				calories,
				now,
			)
			if err := h.eventRecorder.RecordEvents(txCtx, []any{ev1}); err != nil {
				return fmt.Errorf("record meal logged outbox events: %w", err)
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

	return &mealLog, nil
}
