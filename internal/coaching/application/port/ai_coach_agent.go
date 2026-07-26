package port

import (
	"context"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

type RoadmapStrategyParams struct {
	UserID             string
	Goal               string
	ExperienceLevel    string
	AvailableSlots     []string
	AvailableEquipment []string
}

type RoadmapStrategyOutput struct {
	StartDate time.Time
	EndDate   time.Time
}

type WeeklyScheduleParams struct {
	UserID         string
	RoadmapID      string
	WeekNumber     int32
	Goal           string
	AvailableSlots []string
}

type WeeklyScheduleOutput struct {
	MuscleSplitType string
	Days            []domain.ScheduleDay
}

type DailyPrescriptionParams struct {
	UserID                  string
	RoadmapID               string
	WeeklyScheduleID        string
	ScheduledDate           time.Time
	TargetMuscleGroups      []string
	AvailableEquipment      []string
	ActiveInjuries          []string
	AnomalousSessionDetected bool
	IsDeloadWeek            bool
	UserBaseline1RM         map[string]float32
}

type DailyPrescriptionOutput struct {
	Prescription          domain.WorkoutPrescription
	ReasoningExplanation  string
	AdjustmentExplanation string
}

type CoachingAgent interface {
	GenerateRoadmapStrategy(ctx context.Context, params RoadmapStrategyParams) (*RoadmapStrategyOutput, error)
	GenerateWeeklySchedule(ctx context.Context, params WeeklyScheduleParams) (*WeeklyScheduleOutput, error)
	GenerateDailyPrescription(ctx context.Context, params DailyPrescriptionParams) (*DailyPrescriptionOutput, error)
}
