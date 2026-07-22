package domain

import (
	"context"
)

type WorkoutRoadmapRepository interface {
	Save(ctx context.Context, r *WorkoutRoadmap) error
	FindByID(ctx context.Context, id string) (*WorkoutRoadmap, error)
}

type WeeklyScheduleRepository interface {
	Save(ctx context.Context, s *WeeklySchedule) error
	FindByID(ctx context.Context, id string) (*WeeklySchedule, error)
}

type DailyWorkoutPlanRepository interface {
	Save(ctx context.Context, p *DailyWorkoutPlan) error
	FindByID(ctx context.Context, id string) (*DailyWorkoutPlan, error)
}
