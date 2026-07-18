package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var _ port.Repository = (*PostgresRepository)(nil)

type roadmapRecord struct {
	ID             string         `gorm:"column:id;primaryKey"`
	UserID         string         `gorm:"column:user_id;not null"`
	Status         string         `gorm:"column:status;not null"`
	StartDate      time.Time      `gorm:"column:start_date;not null"`
	EndDate        time.Time      `gorm:"column:end_date;not null"`
	PlanningInput  datatypes.JSON `gorm:"column:planning_input;type:jsonb;not null"`
	PlannerVersion string         `gorm:"column:planner_version;not null"`
}

func (roadmapRecord) TableName() string { return "coaching.workout_roadmaps" }

type scheduleRecord struct {
	ID         string         `gorm:"column:id;primaryKey"`
	RoadmapID  string         `gorm:"column:roadmap_id;not null"`
	UserID     string         `gorm:"column:user_id;not null"`
	WeekNumber int            `gorm:"column:week_number;not null"`
	StartDate  time.Time      `gorm:"column:start_date;not null"`
	EndDate    time.Time      `gorm:"column:end_date;not null"`
	Days       datatypes.JSON `gorm:"column:schedule_days;type:jsonb;not null"`
}

func (scheduleRecord) TableName() string { return "coaching.weekly_schedules" }

type outboxRecord struct {
	ID           string         `gorm:"column:id;primaryKey"`
	EventType    string         `gorm:"column:event_type;not null"`
	Source       string         `gorm:"column:source;not null"`
	Subject      string         `gorm:"column:subject;not null"`
	PartitionKey string         `gorm:"column:partition_key;not null"`
	EventTime    time.Time      `gorm:"column:event_time;not null"`
	Data         datatypes.JSON `gorm:"column:data;type:jsonb;not null"`
	Published    bool           `gorm:"column:published;not null"`
}

func (outboxRecord) TableName() string { return "coaching.outbox_events" }

type dailyPlanRecord struct {
	ID               string         `gorm:"column:id;primaryKey"`
	UserID           string         `gorm:"column:user_id;not null"`
	RoadmapID        string         `gorm:"column:roadmap_id;not null"`
	WeeklyScheduleID string         `gorm:"column:weekly_schedule_id;not null"`
	ScheduledDate    time.Time      `gorm:"column:scheduled_date;not null"`
	Status           string         `gorm:"column:status;not null"`
	Exercises        datatypes.JSON `gorm:"column:exercises;type:jsonb;not null"`
	WarmUpItems      datatypes.JSON `gorm:"column:warm_up_items;type:jsonb;not null"`
	CoolDownItems    datatypes.JSON `gorm:"column:cool_down_items;type:jsonb;not null"`
	GeneratedAt      time.Time      `gorm:"column:generated_at;not null"`
}

func (dailyPlanRecord) TableName() string { return "coaching.daily_workout_plans" }

type PostgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateRoadmapWithSchedule(
	ctx context.Context,
	roadmap *domain.WorkoutRoadmap,
	schedule *domain.WeeklySchedule,
	events []domain.Event,
) error {
	roadmapRow, err := toRoadmapRecord(roadmap)
	if err != nil {
		return err
	}
	scheduleRow, err := toScheduleRecord(schedule)
	if err != nil {
		return err
	}
	outboxRows, err := toOutboxRecords(events)
	if err != nil {
		return err
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&roadmapRow).Error; err != nil {
			return fmt.Errorf("insert roadmap: %w", err)
		}
		if err := tx.Create(&scheduleRow).Error; err != nil {
			return fmt.Errorf("insert first schedule: %w", err)
		}
		if err := tx.Create(&outboxRows).Error; err != nil {
			return fmt.Errorf("insert outbox events: %w", err)
		}
		return nil
	})
}

func (r *PostgresRepository) SaveSchedule(
	ctx context.Context,
	schedule *domain.WeeklySchedule,
	event domain.Event,
) error {
	scheduleRow, err := toScheduleRecord(schedule)
	if err != nil {
		return err
	}
	outboxRows, err := toOutboxRecords([]domain.Event{event})
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&scheduleRow).Error; err != nil {
			return fmt.Errorf("insert schedule: %w", err)
		}
		if err := tx.Create(&outboxRows[0]).Error; err != nil {
			return fmt.Errorf("insert outbox event: %w", err)
		}
		return nil
	})
}

func (r *PostgresRepository) FindActiveRoadmapByUser(
	ctx context.Context,
	userID string,
) (*domain.WorkoutRoadmap, error) {
	var row roadmapRecord
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, string(domain.RoadmapStatusActive)).
		First(&row).Error
	return mapRoadmapResult(row, err)
}

func (r *PostgresRepository) FindRoadmap(
	ctx context.Context,
	userID string,
	roadmapID string,
) (*domain.WorkoutRoadmap, error) {
	var row roadmapRecord
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", roadmapID, userID).First(&row).Error
	return mapRoadmapResult(row, err)
}

func (r *PostgresRepository) ListRoadmaps(
	ctx context.Context,
	userID string,
) ([]*domain.WorkoutRoadmap, error) {
	var rows []roadmapRecord
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("start_date DESC, id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query roadmaps: %w", err)
	}
	roadmaps := make([]*domain.WorkoutRoadmap, 0, len(rows))
	for _, row := range rows {
		roadmap, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		roadmaps = append(roadmaps, roadmap)
	}
	return roadmaps, nil
}

func (r *PostgresRepository) FindSchedule(
	ctx context.Context,
	userID string,
	scheduleID string,
) (*domain.WeeklySchedule, error) {
	var row scheduleRecord
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", scheduleID, userID).First(&row).Error
	return mapScheduleResult(row, err)
}

func (r *PostgresRepository) FindScheduleByWeek(
	ctx context.Context,
	roadmapID string,
	weekNumber int,
) (*domain.WeeklySchedule, error) {
	var row scheduleRecord
	err := r.db.WithContext(ctx).
		Where("roadmap_id = ? AND week_number = ?", roadmapID, weekNumber).
		First(&row).Error
	return mapScheduleResult(row, err)
}

func (r *PostgresRepository) ListSchedules(
	ctx context.Context,
	userID string,
	roadmapID string,
) ([]*domain.WeeklySchedule, error) {
	var rows []scheduleRecord
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND roadmap_id = ?", userID, roadmapID).
		Order("week_number").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query schedules: %w", err)
	}
	schedules := make([]*domain.WeeklySchedule, 0, len(rows))
	for _, row := range rows {
		schedule, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, schedule)
	}
	return schedules, nil
}

func (r *PostgresRepository) SaveDailyPlan(
	ctx context.Context,
	schedule *domain.WeeklySchedule,
	plan *domain.DailyWorkoutPlan,
	event domain.Event,
) error {
	scheduleRow, err := toScheduleRecord(schedule)
	if err != nil {
		return err
	}
	planRow, err := toDailyPlanRecord(plan)
	if err != nil {
		return err
	}
	outboxRows, err := toOutboxRecords([]domain.Event{event})
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&scheduleRow).Error; err != nil {
			return fmt.Errorf("update schedule: %w", err)
		}
		if err := tx.Create(&planRow).Error; err != nil {
			return fmt.Errorf("insert daily plan: %w", err)
		}
		if err := tx.Create(&outboxRows[0]).Error; err != nil {
			return fmt.Errorf("insert outbox event: %w", err)
		}
		return nil
	})
}

func (r *PostgresRepository) FindDailyPlan(
	ctx context.Context,
	userID string,
	planID string,
) (*domain.DailyWorkoutPlan, error) {
	var row dailyPlanRecord
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", planID, userID).First(&row).Error
	return mapDailyPlanResult(row, err)
}

func (r *PostgresRepository) FindDailyPlanByDate(
	ctx context.Context,
	scheduleID string,
	scheduledDate time.Time,
) (*domain.DailyWorkoutPlan, error) {
	var row dailyPlanRecord
	err := r.db.WithContext(ctx).
		Where("weekly_schedule_id = ? AND scheduled_date = ?", scheduleID, scheduledDate).
		First(&row).Error
	return mapDailyPlanResult(row, err)
}

func toRoadmapRecord(roadmap *domain.WorkoutRoadmap) (roadmapRecord, error) {
	payload, err := json.Marshal(roadmap.Input)
	if err != nil {
		return roadmapRecord{}, fmt.Errorf("marshal planning input: %w", err)
	}
	return roadmapRecord{
		ID: roadmap.ID, UserID: roadmap.UserID, Status: string(roadmap.Status),
		StartDate: roadmap.StartDate, EndDate: roadmap.EndDate,
		PlanningInput: payload, PlannerVersion: roadmap.PlannerVersion,
	}, nil
}

func toScheduleRecord(schedule *domain.WeeklySchedule) (scheduleRecord, error) {
	payload, err := json.Marshal(schedule.Days)
	if err != nil {
		return scheduleRecord{}, fmt.Errorf("marshal schedule days: %w", err)
	}
	return scheduleRecord{
		ID: schedule.ID, RoadmapID: schedule.RoadmapID, UserID: schedule.UserID,
		WeekNumber: schedule.WeekNumber, StartDate: schedule.StartDate,
		EndDate: schedule.EndDate, Days: payload,
	}, nil
}

func toOutboxRecords(events []domain.Event) ([]outboxRecord, error) {
	rows := make([]outboxRecord, 0, len(events))
	for _, event := range events {
		payload, err := json.Marshal(event.Data)
		if err != nil {
			return nil, fmt.Errorf("marshal event data: %w", err)
		}
		rows = append(rows, outboxRecord{
			ID: event.ID, EventType: event.Type, Source: event.Source,
			Subject: event.Subject, PartitionKey: event.PartitionKey,
			EventTime: event.Time, Data: payload,
		})
	}
	return rows, nil
}

func toDailyPlanRecord(plan *domain.DailyWorkoutPlan) (dailyPlanRecord, error) {
	payload, err := json.Marshal(plan.Exercises)
	if err != nil {
		return dailyPlanRecord{}, fmt.Errorf("marshal prescribed exercises: %w", err)
	}
	warmUpItems, err := json.Marshal(plan.WarmUpItems)
	if err != nil {
		return dailyPlanRecord{}, fmt.Errorf("marshal warm-up items: %w", err)
	}
	coolDownItems, err := json.Marshal(plan.CoolDownItems)
	if err != nil {
		return dailyPlanRecord{}, fmt.Errorf("marshal cool-down items: %w", err)
	}
	return dailyPlanRecord{
		ID: plan.ID, UserID: plan.UserID, RoadmapID: plan.RoadmapID,
		WeeklyScheduleID: plan.WeeklyScheduleID, ScheduledDate: plan.ScheduledDate,
		Status: string(plan.Status), Exercises: payload, WarmUpItems: warmUpItems,
		CoolDownItems: coolDownItems, GeneratedAt: plan.GeneratedAt,
	}, nil
}

func mapRoadmapResult(row roadmapRecord, err error) (*domain.WorkoutRoadmap, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query roadmap: %w", err)
	}
	return row.toDomain()
}

func mapScheduleResult(row scheduleRecord, err error) (*domain.WeeklySchedule, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query schedule: %w", err)
	}
	return row.toDomain()
}

func mapDailyPlanResult(row dailyPlanRecord, err error) (*domain.DailyWorkoutPlan, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query daily plan: %w", err)
	}
	var exercises []domain.PrescribedExercise
	if err := json.Unmarshal(row.Exercises, &exercises); err != nil {
		return nil, fmt.Errorf("unmarshal prescribed exercises: %w", err)
	}
	var warmUpItems []domain.PlannedActivity
	if err := json.Unmarshal(row.WarmUpItems, &warmUpItems); err != nil {
		return nil, fmt.Errorf("unmarshal warm-up items: %w", err)
	}
	var coolDownItems []domain.PlannedActivity
	if err := json.Unmarshal(row.CoolDownItems, &coolDownItems); err != nil {
		return nil, fmt.Errorf("unmarshal cool-down items: %w", err)
	}
	return &domain.DailyWorkoutPlan{
		ID: row.ID, UserID: row.UserID, RoadmapID: row.RoadmapID,
		WeeklyScheduleID: row.WeeklyScheduleID, ScheduledDate: row.ScheduledDate,
		Status: domain.DailyPlanStatus(row.Status), Exercises: exercises,
		WarmUpItems: warmUpItems, CoolDownItems: coolDownItems,
		GeneratedAt: row.GeneratedAt,
	}, nil
}

func (row roadmapRecord) toDomain() (*domain.WorkoutRoadmap, error) {
	var input domain.PlanningInput
	if err := json.Unmarshal(row.PlanningInput, &input); err != nil {
		return nil, fmt.Errorf("unmarshal planning input: %w", err)
	}
	return &domain.WorkoutRoadmap{
		ID: row.ID, UserID: row.UserID, Status: domain.RoadmapStatus(row.Status),
		StartDate: row.StartDate, EndDate: row.EndDate, Input: input,
		PlannerVersion: row.PlannerVersion,
	}, nil
}

func (row scheduleRecord) toDomain() (*domain.WeeklySchedule, error) {
	var days []domain.ScheduleDay
	if err := json.Unmarshal(row.Days, &days); err != nil {
		return nil, fmt.Errorf("unmarshal schedule days: %w", err)
	}
	return &domain.WeeklySchedule{
		ID: row.ID, RoadmapID: row.RoadmapID, UserID: row.UserID,
		WeekNumber: row.WeekNumber, StartDate: row.StartDate,
		EndDate: row.EndDate, Days: days,
	}, nil
}
