package domain

import (
	"errors"
	"time"
)

var ErrRoadmapIDRequired = errors.New("roadmap id is required")

type RoadmapStatus string

const (
	RoadmapStatusActive    RoadmapStatus = "active"
	RoadmapStatusPaused    RoadmapStatus = "paused"
	RoadmapStatusCompleted RoadmapStatus = "completed"
	RoadmapStatusCancelled RoadmapStatus = "cancelled"
)

type WorkoutRoadmap struct {
	ID             string
	UserID         string
	Status         RoadmapStatus
	StartDate      time.Time
	EndDate        time.Time
	Input          PlanningInput
	PlannerVersion string
}

func NewWorkoutRoadmap(id string, userID string, input PlanningInput, plannerVersion string) (*WorkoutRoadmap, error) {
	if id == "" {
		return nil, ErrRoadmapIDRequired
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}

	startDate := dateOnly(input.StartDate)
	return &WorkoutRoadmap{
		ID:             id,
		UserID:         userID,
		Status:         RoadmapStatusActive,
		StartDate:      startDate,
		EndDate:        startDate.AddDate(0, 0, 27),
		Input:          copyPlanningInput(input),
		PlannerVersion: plannerVersion,
	}, nil
}

func dateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func copyPlanningInput(input PlanningInput) PlanningInput {
	input.PreferredWeekdays = append([]time.Weekday(nil), input.PreferredWeekdays...)
	input.EquipmentIDs = append([]string(nil), input.EquipmentIDs...)
	input.ActiveInjuryAreas = append([]string(nil), input.ActiveInjuryAreas...)
	return input
}
