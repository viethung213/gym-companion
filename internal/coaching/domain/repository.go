package domain

import "context"

// RoadmapRepository defines the persistence port for WorkoutRoadmap aggregate.
type RoadmapRepository interface {
	Save(ctx context.Context, roadmap *WorkoutRoadmap, event *Event) error
	FindActiveByUserID(ctx context.Context, userID string) (*WorkoutRoadmap, error)
}

// WeeklyScheduleRepository defines the persistence port for WeeklySchedule aggregate.
type WeeklyScheduleRepository interface {
	Save(ctx context.Context, schedule *WeeklySchedule, event *Event) error
}

// DailyWorkoutPlanRepository defines the persistence port for DailyWorkoutPlan aggregate.
type DailyWorkoutPlanRepository interface {
	SaveBatch(ctx context.Context, plans []*DailyWorkoutPlan, events []*Event) error
}
