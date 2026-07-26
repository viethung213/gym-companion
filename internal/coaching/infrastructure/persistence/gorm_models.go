package persistence

import "time"

type WorkoutRoadmapModel struct {
	ID        string    `gorm:"primaryKey;column:id"`
	UserID    string    `gorm:"column:user_id;index:idx_roadmaps_user_status"`
	Status    int32     `gorm:"column:status;index:idx_roadmaps_user_status"`
	StartDate time.Time `gorm:"column:start_date"`
	EndDate   time.Time `gorm:"column:end_date"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (WorkoutRoadmapModel) TableName() string {
	return "coaching.workout_roadmaps"
}

type WeeklyScheduleModel struct {
	ID              string    `gorm:"primaryKey;column:id"`
	RoadmapID       string    `gorm:"column:roadmap_id;index:idx_schedules_roadmap_week"`
	UserID          string    `gorm:"column:user_id"`
	WeekNumber      int32     `gorm:"column:week_number;index:idx_schedules_roadmap_week"`
	StartDate       time.Time `gorm:"column:start_date"`
	EndDate         time.Time `gorm:"column:end_date"`
	MuscleSplitType string    `gorm:"column:muscle_split_type"`
	ScheduleDaysJSON string   `gorm:"column:schedule_days;type:jsonb"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

func (WeeklyScheduleModel) TableName() string {
	return "coaching.weekly_schedules"
}

type DailyWorkoutPlanModel struct {
	ID                    string    `gorm:"primaryKey;column:id"`
	UserID                string    `gorm:"column:user_id;index:idx_daily_plans_user_date"`
	RoadmapID             string    `gorm:"column:roadmap_id"`
	WeeklyScheduleID      string    `gorm:"column:weekly_schedule_id"`
	ScheduledDate         time.Time `gorm:"column:scheduled_date;index:idx_daily_plans_user_date"`
	Status                int32     `gorm:"column:status"`
	PrescriptionJSON      string    `gorm:"column:prescription;type:jsonb"`
	ReasoningExplanation  string    `gorm:"column:reasoning_explanation"`
	AdjustmentExplanation string    `gorm:"column:adjustment_explanation"`
	GeneratedAt           time.Time `gorm:"column:generated_at"`
	CreatedAt             time.Time `gorm:"column:created_at"`
	UpdatedAt             time.Time `gorm:"column:updated_at"`
}

func (DailyWorkoutPlanModel) TableName() string {
	return "coaching.daily_workout_plans"
}
