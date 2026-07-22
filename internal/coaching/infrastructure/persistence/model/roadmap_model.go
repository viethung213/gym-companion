package model

import (
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

type WorkoutRoadmapModel struct {
	ID           string     `gorm:"column:id;primaryKey"`
	UserID       string     `gorm:"column:user_id"`
	Status       string     `gorm:"column:status"`
	PlanningTier string     `gorm:"column:planning_tier"`
	StartDate    time.Time  `gorm:"column:start_date"`
	EndDate      *time.Time `gorm:"column:end_date"`
	CreatedAt    time.Time  `gorm:"column:created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at"`
}

func (WorkoutRoadmapModel) TableName() string {
	return "coaching_roadmaps"
}


func (m *WorkoutRoadmapModel) ToDomain() *domain.WorkoutRoadmap {
	return domain.UnmarshalWorkoutRoadmap(
		m.ID,
		m.UserID,
		m.Status,
		m.StartDate,
		m.EndDate,
		m.PlanningTier,
		m.CreatedAt,
		m.UpdatedAt,
	)
}

func WorkoutRoadmapToPersistence(d *domain.WorkoutRoadmap) *WorkoutRoadmapModel {
	return &WorkoutRoadmapModel{
		ID:           d.ID(),
		UserID:       d.UserID(),
		Status:       string(d.Status()),
		PlanningTier: string(d.PlanningTier()),
		StartDate:    d.StartDate(),
		EndDate:      d.EndDate(),
		CreatedAt:    d.CreatedAt(),
		UpdatedAt:    d.UpdatedAt(),
	}
}
