package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrPlanningGoalRequired      = errors.New("planning goal is required")
	ErrExperienceLevelRequired   = errors.New("experience level is required")
	ErrTrainingDaysOutOfRange    = errors.New("training days must be between 1 and 6")
	ErrSessionDurationOutOfRange = errors.New("session duration must be between 15 and 240 minutes")
	ErrEquipmentRequired         = errors.New("at least one equipment id is required")
	ErrTimezoneRequired          = errors.New("timezone is required")
	ErrRoadmapStartDateRequired  = errors.New("roadmap start date is required")
)

type PlanningGoal string

const (
	PlanningGoalMuscleGain     PlanningGoal = "muscle_gain"
	PlanningGoalFatLoss        PlanningGoal = "fat_loss"
	PlanningGoalGeneralFitness PlanningGoal = "general_fitness"
)

type ExperienceLevel string

const (
	ExperienceLevelBeginner     ExperienceLevel = "beginner"
	ExperienceLevelIntermediate ExperienceLevel = "intermediate"
	ExperienceLevelAdvanced     ExperienceLevel = "advanced"
)

type PlanningInput struct {
	ProfileSnapshotID   string
	Goal                PlanningGoal
	ExperienceLevel     ExperienceLevel
	TrainingDaysPerWeek int
	PreferredWeekdays   []time.Weekday
	MaxSessionMinutes   int
	EquipmentIDs        []string
	ActiveInjuryAreas   []string
	Timezone            string
	StartDate           time.Time
}

func (i PlanningInput) Validate() error {
	if i.Goal == "" {
		return ErrPlanningGoalRequired
	}
	if i.ExperienceLevel == "" {
		return ErrExperienceLevelRequired
	}
	if i.TrainingDaysPerWeek < 1 || i.TrainingDaysPerWeek > 6 {
		return ErrTrainingDaysOutOfRange
	}
	if i.MaxSessionMinutes < 15 || i.MaxSessionMinutes > 240 {
		return ErrSessionDurationOutOfRange
	}
	if len(i.EquipmentIDs) == 0 {
		return ErrEquipmentRequired
	}
	if strings.TrimSpace(i.Timezone) == "" {
		return ErrTimezoneRequired
	}
	if i.StartDate.IsZero() {
		return ErrRoadmapStartDateRequired
	}

	return nil
}
