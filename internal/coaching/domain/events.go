package domain

import "time"

type RoadmapInitiatedEvent struct {
	RoadmapID   string
	UserID      string
	StartDate   time.Time
	EndDate     time.Time
	InitiatedAt time.Time
}

type WeeklyScheduleGeneratedEvent struct {
	WeeklyScheduleID string
	RoadmapID        string
	UserID           string
	WeekNumber       int32
	StartDate        time.Time
	EndDate          time.Time
	GeneratedAt      time.Time
}

type DailyWorkoutPlanGeneratedEvent struct {
	DailyWorkoutPlanID string
	WeeklyScheduleID   string
	RoadmapID          string
	UserID             string
	ScheduledDate      time.Time
	GeneratedAt        time.Time
}
