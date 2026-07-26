package port

import (
	"context"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

type InitiateRoadmapCommand struct {
	UserID             string
	ProfileSnapshotID  string
	Goal               string
	ExperienceLevel    string
	AvailableSlots     []string
	AvailableEquipment []string
}

type InitiateRoadmapResult struct {
	Roadmap        *domain.WorkoutRoadmap
	WeeklySchedule *domain.WeeklySchedule
}

type InitiateRoadmapUseCase interface {
	Execute(ctx context.Context, cmd InitiateRoadmapCommand) (*InitiateRoadmapResult, error)
}

type GenerateDailyPlanCommand struct {
	UserID                  string
	ScheduledDate           time.Time
	CheckInAnswers          map[string]string
	AnomalousSessionDetected bool
	IsDeloadWeek            bool
}

type GenerateDailyPlanUseCase interface {
	Execute(ctx context.Context, cmd GenerateDailyPlanCommand) (*domain.DailyWorkoutPlan, error)
}

type ProcessPostWorkoutCommand struct {
	UserID           string
	DailyPlanID      string
	RPE              float32
	FormScore        float32
	CompletedSets    int32
	CompletedReps    int32
	MaxWeightLifted  float32
	ActiveInjuries   []string
}

type ProcessPostWorkoutResult struct {
	CoachMessage       string
	NextDailyPlanDraft *domain.DailyWorkoutPlan
}

type ProcessPostWorkoutUseCase interface {
	Execute(ctx context.Context, cmd ProcessPostWorkoutCommand) (*ProcessPostWorkoutResult, error)
}
