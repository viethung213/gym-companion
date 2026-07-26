package domain

import (
	"context"
	"time"
)

type WorkoutRoadmapRepository interface {
	Save(ctx context.Context, roadmap *WorkoutRoadmap) error
	FindByID(ctx context.Context, id string) (*WorkoutRoadmap, error)
	FindActiveByUserID(ctx context.Context, userID string) (*WorkoutRoadmap, error)
}

type WeeklyScheduleRepository interface {
	Save(ctx context.Context, schedule *WeeklySchedule) error
	FindByID(ctx context.Context, id string) (*WeeklySchedule, error)
	FindCurrentByRoadmapID(ctx context.Context, roadmapID string, weekNumber int32) (*WeeklySchedule, error)
}

type DailyWorkoutPlanRepository interface {
	Save(ctx context.Context, plan *DailyWorkoutPlan) error
	FindByID(ctx context.Context, id string) (*DailyWorkoutPlan, error)
	FindByUserAndDate(ctx context.Context, userID string, date time.Time) (*DailyWorkoutPlan, error)
}
