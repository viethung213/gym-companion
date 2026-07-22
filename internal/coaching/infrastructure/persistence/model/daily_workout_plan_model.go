package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

type DailyWorkoutPlanModel struct {
	ID         string                 `gorm:"column:id;primaryKey"`
	ScheduleID string                 `gorm:"column:schedule_id"`
	UserID     string                 `gorm:"column:user_id"`
	Status     string                 `gorm:"column:status"`
	CreatedAt  time.Time              `gorm:"column:created_at"`
	UpdatedAt  time.Time              `gorm:"column:updated_at"`
	Exercises  []PlannedExerciseModel `gorm:"foreignKey:PlanID"`
}

func (DailyWorkoutPlanModel) TableName() string {
	return "coaching_daily_workout_plans"
}

type PlannedExerciseModel struct {
	ID            string  `gorm:"column:id;primaryKey"`
	PlanID        string  `gorm:"column:plan_id"`
	ExerciseID    string  `gorm:"column:exercise_id"`
	Sets          int     `gorm:"column:sets"`
	Reps          int     `gorm:"column:reps"`
	Weight        float64 `gorm:"column:weight"`
	RPE           float64 `gorm:"column:rpe"`
	RestSeconds   int     `gorm:"column:rest_seconds"`
	Notes         string  `gorm:"column:notes"`
	SequenceOrder int     `gorm:"column:sequence_order"`
}

func (PlannedExerciseModel) TableName() string {
	return "coaching_planned_exercises"
}

func (m *DailyWorkoutPlanModel) ToDomain() *domain.DailyWorkoutPlan {
	var domainExs []domain.PlannedExercise
	for _, ex := range m.Exercises {
		domainExs = append(domainExs, domain.UnmarshalPlannedExercise(
			ex.ID,
			ex.ExerciseID,
			ex.Sets,
			ex.Reps,
			ex.Weight,
			ex.RPE,
			ex.RestSeconds,
			ex.Notes,
		))
	}
	return domain.UnmarshalDailyWorkoutPlan(
		m.ID,
		m.ScheduleID,
		m.UserID,
		m.Status,
		domainExs,
		m.CreatedAt,
		m.UpdatedAt,
	)
}

func DailyWorkoutPlanToPersistence(d *domain.DailyWorkoutPlan) *DailyWorkoutPlanModel {
	var exsModel []PlannedExerciseModel
	for i, ex := range d.Exercises() {
		exID := ex.ID()
		if exID == "" {
			exID = uuid.New().String()
		}

		exsModel = append(exsModel, PlannedExerciseModel{
			ID:            exID,
			PlanID:        d.ID(),
			ExerciseID:    ex.ExerciseID(),
			Sets:          ex.Prescription().Sets(),
			Reps:          ex.Prescription().Reps(),
			Weight:        ex.Prescription().Weight(),
			RPE:           ex.Prescription().RPE(),
			RestSeconds:   ex.Prescription().RestSeconds(),
			Notes:         ex.Notes(),
			SequenceOrder: i,
		})
	}

	return &DailyWorkoutPlanModel{
		ID:         d.ID(),
		ScheduleID: d.ScheduleID(),
		UserID:     d.UserID(),
		Status:     string(d.Status()),
		CreatedAt:  d.CreatedAt(),
		UpdatedAt:  d.UpdatedAt(),
		Exercises:  exsModel,
	}
}

