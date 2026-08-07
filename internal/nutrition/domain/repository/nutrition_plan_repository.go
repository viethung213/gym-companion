package repository

import (
	"context"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
)

type NutritionPlanRepository interface {
	FindByUserIDAndDate(ctx context.Context, userID string, date time.Time) (*aggregate.NutritionPlan, error)
	Save(ctx context.Context, plan *aggregate.NutritionPlan) error
	Update(ctx context.Context, plan *aggregate.NutritionPlan) error
	// FindActiveUserIDs trả về danh sách userID có hoạt động (log bữa ăn) trong N ngày gần nhất.
	// Dùng bởi DailyMenuCronWorker để xác định user cần sinh thực đơn sáng.
	FindActiveUserIDs(ctx context.Context, withinDays int) ([]string, error)
	FindPlansForDate(ctx context.Context, targetDate time.Time) ([]*aggregate.NutritionPlan, error)
	GetUserMealSchedules(ctx context.Context, userID string) (map[string]string, error)
	SaveUserMealSchedules(ctx context.Context, userID string, schedules map[string]string) error
}
