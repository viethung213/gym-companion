package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/application/port"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
)

type MealScheduleItemInput struct {
	MealType      string
	ScheduledTime string
}

type UpdateMealScheduleCommand struct {
	UserID    string
	Schedules []MealScheduleItemInput
}

type UpdateMealScheduleResult struct {
	Success   bool
	Message   string
	Schedules []MealScheduleItemInput
}

type UpdateMealScheduleHandler struct {
	planRepo  repository.NutritionPlanRepository
	txManager port.TransactionManager
}

func NewUpdateMealScheduleHandler(
	planRepo repository.NutritionPlanRepository,
	txManager ...port.TransactionManager,
) *UpdateMealScheduleHandler {
	var tm port.TransactionManager
	if len(txManager) > 0 {
		tm = txManager[0]
	}
	return &UpdateMealScheduleHandler{
		planRepo:  planRepo,
		txManager: tm,
	}
}

func (h *UpdateMealScheduleHandler) Handle(ctx context.Context, cmd UpdateMealScheduleCommand) (*UpdateMealScheduleResult, error) {
	if cmd.UserID == "" {
		return nil, errors.New("update meal schedule: user_id is required")
	}

	schedulesMap := make(map[string]string)
	for _, item := range cmd.Schedules {
		if item.MealType != "" && item.ScheduledTime != "" {
			schedulesMap[item.MealType] = item.ScheduledTime
		}
	}

	now := time.Now()
	plan, _ := h.planRepo.FindByUserIDAndDate(ctx, cmd.UserID, now)
	if plan != nil {
		plan.UpdateMealSchedule(schedulesMap)
	}

	// Wrap local DB preference save and current plan schedule update in an atomic transaction
	saveFunc := func(txCtx context.Context) error {
		if len(schedulesMap) > 0 {
			if err := h.planRepo.SaveUserMealSchedules(txCtx, cmd.UserID, schedulesMap); err != nil {
				return fmt.Errorf("update meal schedule save user preferences: %w", err)
			}
		}
		if plan != nil {
			if err := h.planRepo.Update(txCtx, plan); err != nil {
				return fmt.Errorf("update current plan schedule: %w", err)
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

	savedSchedules, err := h.planRepo.GetUserMealSchedules(ctx, cmd.UserID)
	if err != nil || len(savedSchedules) == 0 {
		savedSchedules = schedulesMap
	}

	updatedSchedules := make([]MealScheduleItemInput, 0, len(savedSchedules))
	for mealType, scheduledTime := range savedSchedules {
		updatedSchedules = append(updatedSchedules, MealScheduleItemInput{
			MealType:      mealType,
			ScheduledTime: scheduledTime,
		})
	}

	return &UpdateMealScheduleResult{
		Success:   true,
		Message:   "Lịch trình các bữa ăn đã được cập nhật vĩnh viễn cho các thực đơn về sau",
		Schedules: updatedSchedules,
	}, nil
}
