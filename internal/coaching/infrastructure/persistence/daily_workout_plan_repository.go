package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/persistence/model"
	"gorm.io/gorm"
)


var _ domain.DailyWorkoutPlanRepository = (*GormDailyWorkoutPlanRepository)(nil)

type GormDailyWorkoutPlanRepository struct {
	db *gorm.DB
}

func NewGormDailyWorkoutPlanRepository(db *gorm.DB) *GormDailyWorkoutPlanRepository {
	return &GormDailyWorkoutPlanRepository{db: db}
}

func (r *GormDailyWorkoutPlanRepository) Save(ctx context.Context, plan *domain.DailyWorkoutPlan) error {
	m := model.DailyWorkoutPlanToPersistence(plan)
	if err := r.db.WithContext(ctx).Session(&gorm.Session{FullSaveAssociations: true}).Save(m).Error; err != nil {
		return fmt.Errorf("failed to save daily workout plan: %w", err)
	}
	return nil
}

func (r *GormDailyWorkoutPlanRepository) FindByID(ctx context.Context, id string) (*domain.DailyWorkoutPlan, error) {
	var m model.DailyWorkoutPlanModel
	if err := r.db.WithContext(ctx).Preload("Exercises").Where("id = ?", id).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: plan %s not found", domain.ErrInvalidPlan, id)
		}
		return nil, fmt.Errorf("failed to find daily workout plan by id: %w", err)
	}
	return m.ToDomain(), nil
}

func (r *GormDailyWorkoutPlanRepository) FindByScheduleIDAndDate(ctx context.Context, scheduleID string, dateStr string) (*domain.DailyWorkoutPlan, error) {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid date format %s, expected YYYY-MM-DD", domain.ErrInvalidPlan, dateStr)
	}

	startOfDay := t.Truncate(24 * time.Hour)
	endOfDay := startOfDay.Add(24 * time.Hour)

	var m model.DailyWorkoutPlanModel
	if err := r.db.WithContext(ctx).
		Preload("Exercises").
		Where("schedule_id = ? AND created_at >= ? AND created_at < ?", scheduleID, startOfDay, endOfDay).
		First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: plan for schedule %s on date %s not found", domain.ErrInvalidPlan, scheduleID, dateStr)
		}
		return nil, fmt.Errorf("failed to find daily workout plan by schedule and date: %w", err)
	}
	return m.ToDomain(), nil
}


