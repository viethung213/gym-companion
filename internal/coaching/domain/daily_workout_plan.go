package domain

import "time"

type DailyPlanStatus string

const (
	DailyPlanStatusGenerated DailyPlanStatus = "generated"
	DailyPlanStatusUsed      DailyPlanStatus = "used"
	DailyPlanStatusReplaced  DailyPlanStatus = "replaced"
)

type PrescribedExercise struct {
	ExerciseID   string `json:"exercise_id"`
	ExerciseName string `json:"exercise_name"`
	Sets         int    `json:"sets"`
	Reps         int    `json:"reps"`
	RestSeconds  int    `json:"rest_seconds"`
}

type ExerciseOption struct {
	ID                 string
	Name               string
	DefaultRestSeconds int
}

type PlannedActivity struct {
	Name            string `json:"name"`
	DurationSeconds int    `json:"duration_seconds"`
	Notes           string `json:"notes"`
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
