package command

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	planRepo repository.NutritionPlanRepository
}

func NewUpdateMealScheduleHandler(planRepo repository.NutritionPlanRepository) *UpdateMealScheduleHandler {
	return &UpdateMealScheduleHandler{planRepo: planRepo}
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

	if len(schedulesMap) > 0 {
		if err := h.planRepo.SaveUserMealSchedules(ctx, cmd.UserID, schedulesMap); err != nil {
			return nil, fmt.Errorf("update meal schedule save user preferences: %w", err)
		}
	}

	now := time.Now()
	plan, err := h.planRepo.FindByUserIDAndDate(ctx, cmd.UserID, now)
	if err == nil && plan != nil {
		plan.UpdateMealSchedule(schedulesMap)
		_ = h.planRepo.Update(ctx, plan)
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
