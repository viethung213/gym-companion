package domain

import "time"

type DailyPlanStatus string

const (
	DailyPlanStatusGenerated DailyPlanStatus = "generated"
	DailyPlanStatusUsed      DailyPlanStatus = "used"
	DailyPlanStatusReplaced  DailyPlanStatus = "replaced"
)

type PrescribedExercise struct {
	ExerciseID   string
	ExerciseName string
	Sets         int
	Reps         int
	RestSeconds  int
}

type ExerciseOption struct {
	ID                 string
	Name               string
	DefaultRestSeconds int
}

type PlannedActivity struct {
	Name            string
	DurationSeconds int
	Notes           string
}

type DailyWorkoutPlan struct {
	ID               string
	UserID           string
	RoadmapID        string
	WeeklyScheduleID string
	ScheduledDate    time.Time
	Status           DailyPlanStatus
	Exercises        []PrescribedExercise
	WarmUpItems      []PlannedActivity
	CoolDownItems    []PlannedActivity
	GeneratedAt      time.Time
}

func NewDailyWorkoutPlan(
	id string,
	userID string,
	roadmapID string,
	weeklyScheduleID string,
	scheduledDate time.Time,
	exercises []PrescribedExercise,
	warmUpItems []PlannedActivity,
	coolDownItems []PlannedActivity,
	generatedAt time.Time,
) *DailyWorkoutPlan {
	copiedExercises := append([]PrescribedExercise(nil), exercises...)
	return &DailyWorkoutPlan{
		ID:               id,
		UserID:           userID,
		RoadmapID:        roadmapID,
		WeeklyScheduleID: weeklyScheduleID,
		ScheduledDate:    dateOnly(scheduledDate),
		Status:           DailyPlanStatusGenerated,
		Exercises:        copiedExercises,
		WarmUpItems:      append([]PlannedActivity(nil), warmUpItems...),
		CoolDownItems:    append([]PlannedActivity(nil), coolDownItems...),
		GeneratedAt:      generatedAt,
	}
}
