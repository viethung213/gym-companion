package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
	"gorm.io/gorm"
)

// PostgresRoadmapRepository implements domain.RoadmapRepository
type PostgresRoadmapRepository struct {
	db *gorm.DB
}

var _ domain.RoadmapRepository = (*PostgresRoadmapRepository)(nil)

func NewPostgresRoadmapRepository(db *gorm.DB) *PostgresRoadmapRepository {
	return &PostgresRoadmapRepository{db: db}
}

func (r *PostgresRoadmapRepository) getDB(ctx context.Context) *gorm.DB {
	if tx := GetTx(ctx); tx != nil {
		return tx
	}
	return r.db.WithContext(ctx)
}

func (r *PostgresRoadmapRepository) Save(ctx context.Context, roadmap *domain.WorkoutRoadmap, event *domain.Event) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rec := roadmapRecord{
			ID:        roadmap.ID(),
			UserID:    roadmap.UserID(),
			Status:    string(roadmap.Status()),
			StartDate: roadmap.StartDate(),
			EndDate:   roadmap.EndDate(),
			CreatedAt: roadmap.CreatedAt(),
			UpdatedAt: roadmap.UpdatedAt(),
		}

		if err := tx.Save(&rec).Error; err != nil {
			return fmt.Errorf("save roadmap record: %w", err)
		}

		if event != nil {
			if err := insertOutbox(tx, event); err != nil {
				return fmt.Errorf("insert roadmap outbox event: %w", err)
			}
		}
		return nil
	})
}

func (r *PostgresRoadmapRepository) FindActiveByUserID(ctx context.Context, userID string) (*domain.WorkoutRoadmap, error) {
	var rec roadmapRecord
	err := r.getDB(ctx).
		Where("user_id = ? AND status = ?", userID, string(domain.RoadmapStatusActive)).
		First(&rec).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find active roadmap by user id: %w", err)
	}

	return domain.RehydrateRoadmap(
		rec.ID, rec.UserID, domain.RoadmapStatus(rec.Status),
		rec.StartDate, rec.EndDate, rec.CreatedAt, rec.UpdatedAt,
	)
}

// PostgresWeeklyScheduleRepository implements domain.WeeklyScheduleRepository
type PostgresWeeklyScheduleRepository struct {
	db *gorm.DB
}

var _ domain.WeeklyScheduleRepository = (*PostgresWeeklyScheduleRepository)(nil)

func NewPostgresWeeklyScheduleRepository(db *gorm.DB) *PostgresWeeklyScheduleRepository {
	return &PostgresWeeklyScheduleRepository{db: db}
}

func (r *PostgresWeeklyScheduleRepository) getDB(ctx context.Context) *gorm.DB {
	if tx := GetTx(ctx); tx != nil {
		return tx
	}
	return r.db.WithContext(ctx)
}

type scheduleDayDTO struct {
	Date               time.Time               `json:"date"`
	DayOfWeek          string                  `json:"day_of_week"`
	Status             domain.WorkoutDayStatus `json:"status"`
	TargetMuscleGroups []string                `json:"target_muscle_groups"`
	DailyPlanID        string                  `json:"daily_plan_id"`
}

type prescribedExerciseDTO struct {
	ExerciseID      string  `json:"exercise_id"`
	ExerciseName    string  `json:"exercise_name"`
	TargetSets      int     `json:"target_sets"`
	TargetReps      int     `json:"target_reps"`
	TargetWeight    float64 `json:"target_weight"`
	DurationSeconds int     `json:"duration_seconds"`
	Notes           string  `json:"notes"`
}

type workoutPrescriptionDTO struct {
	WarmUps       []prescribedExerciseDTO `json:"warm_ups"`
	MainExercises []prescribedExerciseDTO `json:"main_exercises"`
	CoolDowns     []prescribedExerciseDTO `json:"cool_downs"`
}

func toScheduleDayDTOs(days []domain.ScheduleDay) []scheduleDayDTO {
	dtos := make([]scheduleDayDTO, len(days))
	for i, d := range days {
		dtos[i] = scheduleDayDTO{
			Date:               d.Date,
			DayOfWeek:          d.DayOfWeek,
			Status:             d.Status,
			TargetMuscleGroups: d.TargetMuscleGroups,
			DailyPlanID:        d.DailyPlanID,
		}
	}
	return dtos
}

func toWorkoutPrescriptionDTO(p domain.WorkoutPrescription) workoutPrescriptionDTO {
	toExDTOs := func(exs []domain.PrescribedExercise) []prescribedExerciseDTO {
		res := make([]prescribedExerciseDTO, len(exs))
		for i, e := range exs {
			res[i] = prescribedExerciseDTO{
				ExerciseID:      e.ExerciseID,
				ExerciseName:    e.ExerciseName,
				TargetSets:      e.TargetSets,
				TargetReps:      e.TargetReps,
				TargetWeight:    e.TargetWeight,
				DurationSeconds: e.DurationSeconds,
				Notes:           e.Notes,
			}
		}
		return res
	}
	return workoutPrescriptionDTO{
		WarmUps:       toExDTOs(p.WarmUps),
		MainExercises: toExDTOs(p.MainExercises),
		CoolDowns:     toExDTOs(p.CoolDowns),
	}
}

func (r *PostgresWeeklyScheduleRepository) Save(ctx context.Context, schedule *domain.WeeklySchedule, event *domain.Event) error {
	daysData, err := json.Marshal(toScheduleDayDTOs(schedule.ScheduleDays()))
	if err != nil {
		return fmt.Errorf("marshal schedule days: %w", err)
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rec := weeklyScheduleRecord{
			ID:              schedule.ID(),
			RoadmapID:       schedule.RoadmapID(),
			UserID:          schedule.UserID(),
			WeekNumber:      schedule.WeekNumber(),
			StartDate:       schedule.StartDate(),
			EndDate:         schedule.EndDate(),
			MuscleSplitType: schedule.MuscleSplitType(),
			ScheduleDays:    daysData,
			CreatedAt:       schedule.CreatedAt(),
			UpdatedAt:       schedule.UpdatedAt(),
		}

		if err := tx.Save(&rec).Error; err != nil {
			return fmt.Errorf("save weekly schedule record: %w", err)
		}

		if event != nil {
			if err := insertOutbox(tx, event); err != nil {
				return fmt.Errorf("insert schedule outbox event: %w", err)
			}
		}
		return nil
	})
}

// PostgresDailyWorkoutPlanRepository implements domain.DailyWorkoutPlanRepository
type PostgresDailyWorkoutPlanRepository struct {
	db *gorm.DB
}

var _ domain.DailyWorkoutPlanRepository = (*PostgresDailyWorkoutPlanRepository)(nil)

func NewPostgresDailyWorkoutPlanRepository(db *gorm.DB) *PostgresDailyWorkoutPlanRepository {
	return &PostgresDailyWorkoutPlanRepository{db: db}
}

func (r *PostgresDailyWorkoutPlanRepository) getDB(ctx context.Context) *gorm.DB {
	if tx := GetTx(ctx); tx != nil {
		return tx
	}
	return r.db.WithContext(ctx)
}

func (r *PostgresDailyWorkoutPlanRepository) SaveBatch(ctx context.Context, plans []*domain.DailyWorkoutPlan, events []*domain.Event) error {
	if len(plans) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, plan := range plans {
			prescriptionData, err := json.Marshal(plan.WorkoutPrescription())
			if err != nil {
				return fmt.Errorf("marshal plan prescription for %s: %w", plan.ID(), err)
			}

			var reasoningPtr, adjustmentPtr *string
			if plan.ReasoningExplanation() != "" {
				s := plan.ReasoningExplanation()
				reasoningPtr = &s
			}
			if plan.AdjustmentExplanation() != "" {
				s := plan.AdjustmentExplanation()
				adjustmentPtr = &s
			}

			rec := dailyWorkoutPlanRecord{
				ID:                    plan.ID(),
				WeeklyScheduleID:      plan.WeeklyScheduleID(),
				RoadmapID:             plan.RoadmapID(),
				UserID:                plan.UserID(),
				ScheduledDate:         plan.ScheduledDate(),
				Status:                string(plan.Status()),
				WorkoutPrescription:   prescriptionData,
				ReasoningExplanation:  reasoningPtr,
				AdjustmentExplanation: adjustmentPtr,
				CreatedAt:             plan.CreatedAt(),
				UpdatedAt:             plan.UpdatedAt(),
			}

			if err := tx.Save(&rec).Error; err != nil {
				return fmt.Errorf("save daily plan record %s: %w", plan.ID(), err)
			}

			if i < len(events) && events[i] != nil {
				if err := insertOutbox(tx, events[i]); err != nil {
					return fmt.Errorf("insert plan outbox event for %s: %w", plan.ID(), err)
				}
			}
		}
		return nil
	})
}

// PostgresInboxRepository implements port.InboxRepository
type PostgresInboxRepository struct {
	db *gorm.DB
}

var _ port.InboxRepository = (*PostgresInboxRepository)(nil)

func NewPostgresInboxRepository(db *gorm.DB) *PostgresInboxRepository {
	return &PostgresInboxRepository{db: db}
}

func (r *PostgresInboxRepository) getDB(ctx context.Context) *gorm.DB {
	if tx := GetTx(ctx); tx != nil {
		return tx
	}
	return r.db.WithContext(ctx)
}

func (r *PostgresInboxRepository) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	var count int64
	err := r.getDB(ctx).
		Model(&outboxLogRecord{}).
		Where("event_id = ?", eventID).
		Count(&count).Error

	if err != nil {
		return false, fmt.Errorf("check event_id processed in outbox_log: %w", err)
	}
	return count > 0, nil
}

func (r *PostgresInboxRepository) MarkProcessed(ctx context.Context, eventID, eventType string, payload []byte, partitionKey string) error {
	logID := uuid.New().String()
	rec := outboxLogRecord{
		ID:           logID,
		EventID:      eventID,
		EventType:    eventType,
		Payload:      payload,
		PartitionKey: partitionKey,
		ProcessedAt:  time.Now(),
		Status:       "SUCCESS",
	}

	if err := r.getDB(ctx).Create(&rec).Error; err != nil {
		return fmt.Errorf("mark event processed in outbox_log: %w", err)
	}
	return nil
}
