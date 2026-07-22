package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

type WeeklyScheduleModel struct {
	ID              string             `gorm:"column:id;primaryKey"`
	RoadmapID       string             `gorm:"column:roadmap_id"`
	UserID          string             `gorm:"column:user_id"`
	WeekNumber      int                `gorm:"column:week_number"`
	StartDate       time.Time          `gorm:"column:start_date"`
	EndDate         time.Time          `gorm:"column:end_date"`
	MuscleSplitType string             `gorm:"column:muscle_split_type"`
	CreatedAt       time.Time          `gorm:"column:created_at"`
	UpdatedAt       time.Time          `gorm:"column:updated_at"`
	Days            []ScheduleDayModel `gorm:"foreignKey:ScheduleID"`
}

func (WeeklyScheduleModel) TableName() string {
	return "coaching_schedules"
}


type ScheduleDayModel struct {
	ID                     string   `gorm:"column:id;primaryKey"`
	ScheduleID             string   `gorm:"column:schedule_id"`
	ScheduledDate          time.Time `gorm:"column:scheduled_date"`
	DayOfWeek              int      `gorm:"column:day_of_week"`
	Status                 string   `gorm:"column:status"`
	TargetMuscleGroups     []string `gorm:"column:target_muscle_groups;serializer:json"`
	DailyWorkoutPlanID     string   `gorm:"column:daily_workout_plan_id"`
	TimeWindow             string   `gorm:"column:time_window"`
	PlannedDurationMinutes int      `gorm:"column:planned_duration_minutes"`
}

func (ScheduleDayModel) TableName() string {
	return "coaching_schedule_days"
}

func (m *WeeklyScheduleModel) ToDomain() (*domain.WeeklySchedule, error) {
	var domainDays []domain.ScheduleDay
	for _, dayModel := range m.Days {
		domainDays = append(domainDays, domain.UnmarshalScheduleDay(
			dayModel.ID,
			dayModel.ScheduledDate,
			dayModel.DayOfWeek,
			dayModel.Status,
			dayModel.TargetMuscleGroups,
			dayModel.DailyWorkoutPlanID,
			dayModel.TimeWindow,
			dayModel.PlannedDurationMinutes,
		))
	}

	return domain.UnmarshalWeeklySchedule(
		m.ID,
		m.RoadmapID,
		m.UserID,
		m.WeekNumber,
		m.StartDate,
		m.EndDate,
		m.MuscleSplitType,
		domainDays,
	), nil
}

func WeeklyScheduleToPersistence(d *domain.WeeklySchedule) (*WeeklyScheduleModel, error) {
	var daysModel []ScheduleDayModel
	for _, dayDomain := range d.Days() {
		dayID := dayDomain.ID()
		if dayID == "" {
			dayID = uuid.New().String()
		}

		daysModel = append(daysModel, ScheduleDayModel{
			ID:                     dayID,
			ScheduleID:             d.ID(),
			ScheduledDate:          dayDomain.ScheduledDate(),
			DayOfWeek:              dayDomain.DayOfWeek(),
			Status:                 string(dayDomain.Status()),
			TargetMuscleGroups:     dayDomain.TargetMuscleGroups(),
			DailyWorkoutPlanID:     dayDomain.DailyWorkoutPlanID(),
			TimeWindow:             dayDomain.TimeWindow(),
			PlannedDurationMinutes: dayDomain.PlannedDurationMinutes(),
		})
	}

	return &WeeklyScheduleModel{
		ID:              d.ID(),
		RoadmapID:       d.RoadmapID(),
		UserID:          d.UserID(),
		WeekNumber:      d.WeekNumber(),
		StartDate:       d.StartDate(),
		EndDate:         d.EndDate(),
		MuscleSplitType: d.MuscleSplitType(),
		Days:            daysModel,
	}, nil
}

