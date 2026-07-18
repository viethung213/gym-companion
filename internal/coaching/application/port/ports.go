package port

import (
	"context"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

type Repository interface {
	CreateRoadmapWithSchedule(
		ctx context.Context,
		roadmap *domain.WorkoutRoadmap,
		schedule *domain.WeeklySchedule,
		events []domain.Event,
	) error
	SaveSchedule(ctx context.Context, schedule *domain.WeeklySchedule, event domain.Event) error
	FindActiveRoadmapByUser(ctx context.Context, userID string) (*domain.WorkoutRoadmap, error)
	FindRoadmap(ctx context.Context, userID string, roadmapID string) (*domain.WorkoutRoadmap, error)
	ListRoadmaps(ctx context.Context, userID string) ([]*domain.WorkoutRoadmap, error)
	FindSchedule(ctx context.Context, userID string, scheduleID string) (*domain.WeeklySchedule, error)
	FindScheduleByWeek(
		ctx context.Context,
		roadmapID string,
		weekNumber int,
	) (*domain.WeeklySchedule, error)
	ListSchedules(
		ctx context.Context,
		userID string,
		roadmapID string,
	) ([]*domain.WeeklySchedule, error)
	SaveDailyPlan(
		ctx context.Context,
		schedule *domain.WeeklySchedule,
		plan *domain.DailyWorkoutPlan,
		event domain.Event,
	) error
	FindDailyPlan(ctx context.Context, userID string, planID string) (*domain.DailyWorkoutPlan, error)
	FindDailyPlanByDate(
		ctx context.Context,
		scheduleID string,
		scheduledDate time.Time,
	) (*domain.DailyWorkoutPlan, error)
}

type ExerciseSearchCriteria struct {
	MuscleGroupIDs []string
	EquipmentIDs   []string
	Difficulty     string
	Limit          int
}

type ExerciseCandidate struct {
	ID                 string
	Name               string
	TargetMuscleID     string
	EquipmentID        string
	Difficulty         string
	DefaultRestSeconds int
}

type ExerciseSearcher interface {
	Search(ctx context.Context, criteria ExerciseSearchCriteria) ([]ExerciseCandidate, error)
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() (string, error)
}
