package persistence

import (
	"encoding/json"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

type ScheduleDayDTO struct {
	ScheduledDate      time.Time `json:"scheduled_date"`
	DayOfWeek          string    `json:"day_of_week"`
	Status             int32     `json:"status"`
	TargetMuscleGroups []string  `json:"target_muscle_groups"`
	DailyPlanID        string    `json:"daily_plan_id"`
}

type PrescribedExerciseDTO struct {
	ExerciseID      string  `json:"exercise_id"`
	ExerciseName    string  `json:"exercise_name"`
	TargetSets      int32   `json:"target_sets"`
	TargetReps      int32   `json:"target_reps"`
	TargetWeight    float32 `json:"target_weight"`
	DurationSeconds int32   `json:"duration_seconds"`
	Notes           string  `json:"notes"`
	RestSetSec      int32   `json:"rest_set_sec"`
	RestExerciseSec int32   `json:"rest_exercise_sec"`
	TargetRPE       float32 `json:"target_rpe"`
}

type WorkoutPrescriptionDTO struct {
	WarmUps       []PrescribedExerciseDTO `json:"warm_ups"`
	MainExercises []PrescribedExerciseDTO `json:"main_exercises"`
	CoolDowns     []PrescribedExerciseDTO `json:"cool_downs"`
}

func RoadmapToPersistence(r *domain.WorkoutRoadmap) *WorkoutRoadmapModel {
	return &WorkoutRoadmapModel{
		ID:        r.ID(),
		UserID:    r.UserID(),
		Status:    int32(r.Status()),
		StartDate: r.StartDate(),
		EndDate:   r.EndDate(),
		CreatedAt: r.CreatedAt(),
		UpdatedAt: r.UpdatedAt(),
	}
}

func RoadmapToDomain(m *WorkoutRoadmapModel) *domain.WorkoutRoadmap {
	return domain.ReconstituteWorkoutRoadmap(
		m.ID,
		m.UserID,
		domain.RoadmapStatus(m.Status),
		m.StartDate,
		m.EndDate,
		m.CreatedAt,
		m.UpdatedAt,
	)
}

func WeeklyScheduleToPersistence(s *domain.WeeklySchedule) (*WeeklyScheduleModel, error) {
	dtos := make([]ScheduleDayDTO, len(s.ScheduleDays()))
	for i, d := range s.ScheduleDays() {
		dtos[i] = ScheduleDayDTO{
			ScheduledDate:      d.ScheduledDate(),
			DayOfWeek:          d.DayOfWeek(),
			Status:             int32(d.Status()),
			TargetMuscleGroups: d.TargetMuscleGroups(),
			DailyPlanID:        d.DailyPlanID(),
		}
	}
	daysJSON, err := json.Marshal(dtos)
	if err != nil {
		return nil, err
	}

	return &WeeklyScheduleModel{
		ID:               s.ID(),
		RoadmapID:        s.RoadmapID(),
		UserID:           s.UserID(),
		WeekNumber:       s.WeekNumber(),
		StartDate:        s.StartDate(),
		EndDate:          s.EndDate(),
		MuscleSplitType:  s.MuscleSplitType(),
		ScheduleDaysJSON: string(daysJSON),
		CreatedAt:        s.CreatedAt(),
		UpdatedAt:        s.UpdatedAt(),
	}, nil
}

func WeeklyScheduleToDomain(m *WeeklyScheduleModel) (*domain.WeeklySchedule, error) {
	var dtos []ScheduleDayDTO
	if err := json.Unmarshal([]byte(m.ScheduleDaysJSON), &dtos); err != nil {
		return nil, err
	}

	days := make([]domain.ScheduleDay, len(dtos))
	for i, dto := range dtos {
		days[i] = domain.NewScheduleDay(
			dto.ScheduledDate,
			dto.DayOfWeek,
			domain.WorkoutDayStatus(dto.Status),
			dto.TargetMuscleGroups,
			dto.DailyPlanID,
		)
	}

	return domain.ReconstituteWeeklySchedule(
		m.ID,
		m.RoadmapID,
		m.UserID,
		m.WeekNumber,
		m.StartDate,
		m.EndDate,
		m.MuscleSplitType,
		days,
		m.CreatedAt,
		m.UpdatedAt,
	), nil
}

func DailyWorkoutPlanToPersistence(d *domain.DailyWorkoutPlan) (*DailyWorkoutPlanModel, error) {
	mapEx := func(exs []domain.PrescribedExercise) []PrescribedExerciseDTO {
		res := make([]PrescribedExerciseDTO, len(exs))
		for i, ex := range exs {
			res[i] = PrescribedExerciseDTO{
				ExerciseID:      ex.ExerciseID(),
				ExerciseName:    ex.ExerciseName(),
				TargetSets:      ex.TargetSets(),
				TargetReps:      ex.TargetReps(),
				TargetWeight:    ex.TargetWeight(),
				DurationSeconds: ex.DurationSeconds(),
				Notes:           ex.Notes(),
				RestSetSec:      ex.RestSetSec(),
				RestExerciseSec: ex.RestExerciseSec(),
				TargetRPE:       ex.TargetRPE(),
			}
		}
		return res
	}

	prescriptionDTO := WorkoutPrescriptionDTO{
		WarmUps:       mapEx(d.Prescription().WarmUps()),
		MainExercises: mapEx(d.Prescription().MainExercises()),
		CoolDowns:     mapEx(d.Prescription().CoolDowns()),
	}

	prescriptionJSON, err := json.Marshal(prescriptionDTO)
	if err != nil {
		return nil, err
	}

	return &DailyWorkoutPlanModel{
		ID:                    d.ID(),
		UserID:                d.UserID(),
		RoadmapID:             d.RoadmapID(),
		WeeklyScheduleID:      d.WeeklyScheduleID(),
		ScheduledDate:         d.ScheduledDate(),
		Status:                int32(d.Status()),
		PrescriptionJSON:      string(prescriptionJSON),
		ReasoningExplanation:  d.ReasoningExplanation(),
		AdjustmentExplanation: d.AdjustmentExplanation(),
		GeneratedAt:           d.GeneratedAt(),
		CreatedAt:             d.CreatedAt(),
		UpdatedAt:             d.UpdatedAt(),
	}, nil
}

func DailyWorkoutPlanToDomain(m *DailyWorkoutPlanModel) (*domain.DailyWorkoutPlan, error) {
	var dto WorkoutPrescriptionDTO
	if err := json.Unmarshal([]byte(m.PrescriptionJSON), &dto); err != nil {
		return nil, err
	}

	mapEx := func(dtos []PrescribedExerciseDTO) []domain.PrescribedExercise {
		res := make([]domain.PrescribedExercise, len(dtos))
		for i, d := range dtos {
			res[i] = domain.NewPrescribedExercise(
				d.ExerciseID,
				d.ExerciseName,
				d.TargetSets,
				d.TargetReps,
				d.TargetWeight,
				d.DurationSeconds,
				d.Notes,
				d.RestSetSec,
				d.RestExerciseSec,
				d.TargetRPE,
			)
		}
		return res
	}

	prescription := domain.NewWorkoutPrescription(
		mapEx(dto.WarmUps),
		mapEx(dto.MainExercises),
		mapEx(dto.CoolDowns),
	)

	return domain.ReconstituteDailyWorkoutPlan(
		m.ID,
		m.UserID,
		m.RoadmapID,
		m.WeeklyScheduleID,
		m.ScheduledDate,
		domain.DailyPlanStatus(m.Status),
		prescription,
		m.ReasoningExplanation,
		m.AdjustmentExplanation,
		m.GeneratedAt,
		m.CreatedAt,
		m.UpdatedAt,
	), nil
}
