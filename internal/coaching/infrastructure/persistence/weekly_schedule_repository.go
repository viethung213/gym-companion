package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/persistence/model"
	"gorm.io/gorm"
)

var _ domain.WeeklyScheduleRepository = (*GormWeeklyScheduleRepository)(nil)

type GormWeeklyScheduleRepository struct {
	db *gorm.DB
}

func NewGormWeeklyScheduleRepository(db *gorm.DB) *GormWeeklyScheduleRepository {
	return &GormWeeklyScheduleRepository{db: db}
}

func (r *GormWeeklyScheduleRepository) Save(ctx context.Context, schedule *domain.WeeklySchedule) error {
	m, err := model.WeeklyScheduleToPersistence(schedule)
	if err != nil {
		return fmt.Errorf("failed to map weekly schedule to persistence model: %w", err)
	}
	
	if err := r.db.WithContext(ctx).Session(&gorm.Session{FullSaveAssociations: true}).Save(m).Error; err != nil {
		return fmt.Errorf("failed to save weekly schedule: %w", err)
	}
	return nil
}

func (r *GormWeeklyScheduleRepository) FindByID(ctx context.Context, id string) (*domain.WeeklySchedule, error) {
	var m model.WeeklyScheduleModel
	if err := r.db.WithContext(ctx).Preload("Days").Where("id = ?", id).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: schedule %s not found", domain.ErrInvalidSchedule, id)
		}
		return nil, fmt.Errorf("failed to find weekly schedule by id: %w", err)
	}
	
	d, err := m.ToDomain()
	if err != nil {
		return nil, fmt.Errorf("failed to map to domain: %w", err)
	}
	return d, nil
}

func (r *GormWeeklyScheduleRepository) FindByRoadmapIDAndWeek(ctx context.Context, roadmapID string, week int) (*domain.WeeklySchedule, error) {
	var m model.WeeklyScheduleModel
	if err := r.db.WithContext(ctx).Preload("Days").Where("roadmap_id = ? AND week_number = ?", roadmapID, week).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: schedule for roadmap %s week %d not found", domain.ErrInvalidSchedule, roadmapID, week)
		}
		return nil, fmt.Errorf("failed to find weekly schedule by roadmap and week: %w", err)
	}
	
	d, err := m.ToDomain()
	if err != nil {
		return nil, fmt.Errorf("failed to map to domain: %w", err)
	}
	return d, nil
}
