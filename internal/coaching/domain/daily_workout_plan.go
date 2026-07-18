package domain

import "time"

type DailyPlanStatus string

const (
	DailyPlanStatusGenerated DailyPlanStatus = "generated"
	DailyPlanStatusUsed      DailyPlanStatus = "used"
	DailyPlanStatusReplaced  DailyPlanStatus = "replaced"
)

type PrescribedExercise struct {
	ExerciseID  string
	Sets        int
	Reps        int
	RestSeconds int
}

type DailyWorkoutPlan struct {
	ID               string
	UserID           string
	RoadmapID        string
	WeeklyScheduleID string
	ScheduledDate    time.Time
	Status           DailyPlanStatus
	Exercises        []PrescribedExercise
	GeneratedAt      time.Time
}

func NewDailyWorkoutPlan(id string, userID string, roadmapID string, weeklyScheduleID string, scheduledDate time.Time, exercises []PrescribedExercise, generatedAt time.Time) *DailyWorkoutPlan {
	copiedExercises := append([]PrescribedExercise(nil), exercises...)
	return &DailyWorkoutPlan{
		ID:               id,
		UserID:           userID,
		RoadmapID:        roadmapID,
		WeeklyScheduleID: weeklyScheduleID,
		ScheduledDate:    dateOnly(scheduledDate),
		Status:           DailyPlanStatusGenerated,
		Exercises:        copiedExercises,
		GeneratedAt:      generatedAt,
	}
}
