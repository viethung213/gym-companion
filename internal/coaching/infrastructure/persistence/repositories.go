package persistence

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

type GormWorkoutRoadmapRepository struct {
	db *gorm.DB
}

func NewGormWorkoutRoadmapRepository(db *gorm.DB) *GormWorkoutRoadmapRepository {
	return &GormWorkoutRoadmapRepository{db: db}
}

func (r *GormWorkoutRoadmapRepository) Save(ctx context.Context, roadmap *domain.WorkoutRoadmap) error {
	model := RoadmapToPersistence(roadmap)
	return r.db.WithContext(ctx).Save(model).Error
}

func (r *GormWorkoutRoadmapRepository) FindByID(ctx context.Context, id string) (*domain.WorkoutRoadmap, error) {
	var model WorkoutRoadmapModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return RoadmapToDomain(&model), nil
}

func (r *GormWorkoutRoadmapRepository) FindActiveByUserID(ctx context.Context, userID string) (*domain.WorkoutRoadmap, error) {
	var model WorkoutRoadmapModel
	if err := r.db.WithContext(ctx).First(&model, "user_id = ? AND status = ?", userID, int32(domain.RoadmapStatusActive)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return RoadmapToDomain(&model), nil
}

type GormWeeklyScheduleRepository struct {
	db *gorm.DB
}

func NewGormWeeklyScheduleRepository(db *gorm.DB) *GormWeeklyScheduleRepository {
	return &GormWeeklyScheduleRepository{db: db}
}

func (r *GormWeeklyScheduleRepository) Save(ctx context.Context, schedule *domain.WeeklySchedule) error {
	model, err := WeeklyScheduleToPersistence(schedule)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Save(model).Error
}

func (r *GormWeeklyScheduleRepository) FindByID(ctx context.Context, id string) (*domain.WeeklySchedule, error) {
	var model WeeklyScheduleModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return WeeklyScheduleToDomain(&model)
}

func (r *GormWeeklyScheduleRepository) FindCurrentByRoadmapID(ctx context.Context, roadmapID string, weekNumber int32) (*domain.WeeklySchedule, error) {
	var model WeeklyScheduleModel
	if err := r.db.WithContext(ctx).First(&model, "roadmap_id = ? AND week_number = ?", roadmapID, weekNumber).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return WeeklyScheduleToDomain(&model)
}

type GormDailyWorkoutPlanRepository struct {
	db *gorm.DB
}

func NewGormDailyWorkoutPlanRepository(db *gorm.DB) *GormDailyWorkoutPlanRepository {
	return &GormDailyWorkoutPlanRepository{db: db}
}

func (r *GormDailyWorkoutPlanRepository) Save(ctx context.Context, plan *domain.DailyWorkoutPlan) error {
	model, err := DailyWorkoutPlanToPersistence(plan)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Save(model).Error
}

func (r *GormDailyWorkoutPlanRepository) FindByID(ctx context.Context, id string) (*domain.DailyWorkoutPlan, error) {
	var model DailyWorkoutPlanModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return DailyWorkoutPlanToDomain(&model)
}

func (r *GormDailyWorkoutPlanRepository) FindByUserAndDate(ctx context.Context, userID string, date time.Time) (*domain.DailyWorkoutPlan, error) {
	var model DailyWorkoutPlanModel
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	endOfDay := startOfDay.AddDate(0, 0, 1)

	if err := r.db.WithContext(ctx).First(&model, "user_id = ? AND scheduled_date >= ? AND scheduled_date < ?", userID, startOfDay, endOfDay).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return DailyWorkoutPlanToDomain(&model)
}
