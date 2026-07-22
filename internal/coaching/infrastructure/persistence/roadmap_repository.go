package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/persistence/model"
	"gorm.io/gorm"
)

var _ domain.WorkoutRoadmapRepository = (*GormWorkoutRoadmapRepository)(nil)

type GormWorkoutRoadmapRepository struct {
	db *gorm.DB
}

func NewGormWorkoutRoadmapRepository(db *gorm.DB) *GormWorkoutRoadmapRepository {
	return &GormWorkoutRoadmapRepository{db: db}
}

func (r *GormWorkoutRoadmapRepository) Save(ctx context.Context, roadmap *domain.WorkoutRoadmap) error {
	m := model.WorkoutRoadmapToPersistence(roadmap)
	if err := r.db.WithContext(ctx).Save(m).Error; err != nil {
		return fmt.Errorf("failed to save workout roadmap: %w", err)
	}
	return nil
}

func (r *GormWorkoutRoadmapRepository) FindByID(ctx context.Context, id string) (*domain.WorkoutRoadmap, error) {
	var m model.WorkoutRoadmapModel
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: roadmap %s not found", domain.ErrInvalidRoadmap, id)
		}
		return nil, fmt.Errorf("failed to find workout roadmap by id: %w", err)
	}
	return m.ToDomain(), nil
}

func (r *GormWorkoutRoadmapRepository) FindActiveByUserID(ctx context.Context, userID string) (*domain.WorkoutRoadmap, error) {
	var m model.WorkoutRoadmapModel
	if err := r.db.WithContext(ctx).Where("user_id = ? AND status = ?", userID, string(domain.PlanStatusActive)).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: active roadmap for user %s not found", domain.ErrInvalidRoadmap, userID)
		}
		return nil, fmt.Errorf("failed to find active workout roadmap by user id: %w", err)
	}
	return m.ToDomain(), nil
}
