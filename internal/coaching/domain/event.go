package domain

import "time"

// Event represents an outbox domain event to be published.
type Event struct {
	ID           string
	Type         string
	PartitionKey string
	Payload      []byte
	CreatedAt    time.Time
}

const (
	EventTypeRoadmapInitiated          = "contracts.core.coaching.v1.roadmapInitiated"
	EventTypeWeeklyScheduleGenerated   = "contracts.core.coaching.v1.weeklyScheduleGenerated"
	EventTypeDailyWorkoutPlanGenerated = "contracts.core.coaching.v1.dailyWorkoutPlanGenerated"
)
