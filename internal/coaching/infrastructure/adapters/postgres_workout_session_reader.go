package adapters

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"gorm.io/gorm"
)

// workoutSessionDBModel represents the database model for workout_execution.workout_sessions table.
type workoutSessionDBModel struct {
	ID               string     `gorm:"column:id;primaryKey"`
	UserID           string     `gorm:"column:user_id"`
	PlanID           string     `gorm:"column:plan_id"`
	Status           string     `gorm:"column:status"`
	TotalSets        int        `gorm:"column:total_sets"`
	TotalVolume      float64    `gorm:"column:total_volume"`
	AverageFormScore *float64   `gorm:"column:average_form_score"`
	AverageRPE       *float64   `gorm:"column:average_rpe"`
	ScheduledAt      *time.Time `gorm:"column:scheduled_at"`
	StartedAt        *time.Time `gorm:"column:started_at"`
	EndedAt          *time.Time `gorm:"column:ended_at"`
}

func (workoutSessionDBModel) TableName() string {
	return "workout_execution.workout_sessions"
}

// workoutSetLogDBModel represents the database model for workout_execution.workout_set_logs table.
type workoutSetLogDBModel struct {
	ID         string    `gorm:"column:id;primaryKey"`
	SessionID  string    `gorm:"column:session_id"`
	SetNumber  int       `gorm:"column:set_number"`
	ExerciseID string    `gorm:"column:exercise_id"`
	TargetReps int       `gorm:"column:target_reps"`
	ActualReps int       `gorm:"column:actual_reps"`
	Weight     float64   `gorm:"column:weight"`
	FormScore  *float64  `gorm:"column:form_score"`
	RPE        float64   `gorm:"column:rpe"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

func (workoutSetLogDBModel) TableName() string {
	return "workout_execution.workout_set_logs"
}

// personalRecordDBModel represents the database model for workout_execution.personal_records table.
type personalRecordDBModel struct {
	ID         string    `gorm:"column:id;primaryKey"`
	UserID     string    `gorm:"column:user_id"`
	ExerciseID string    `gorm:"column:exercise_id"`
	OneRepMax  float64   `gorm:"column:one_rep_max"`
	Weight     float64   `gorm:"column:weight"`
	Reps       int       `gorm:"column:reps"`
	AchievedAt time.Time `gorm:"column:achieved_at"`
}

func (personalRecordDBModel) TableName() string {
	return "workout_execution.personal_records"
}

// PostgresWorkoutSessionReader implements port.WorkoutSessionReader querying workout_execution schema.
type PostgresWorkoutSessionReader struct {
	db *gorm.DB
}

var _ port.WorkoutSessionReader = (*PostgresWorkoutSessionReader)(nil)

// NewPostgresWorkoutSessionReader creates a new PostgresWorkoutSessionReader.
func NewPostgresWorkoutSessionReader(db *gorm.DB) *PostgresWorkoutSessionReader {
	return &PostgresWorkoutSessionReader{db: db}
}

// GetRecentSessions fetches completed or aborted workout sessions for the user since given time.
func (r *PostgresWorkoutSessionReader) GetRecentSessions(ctx context.Context, userID string, since time.Time) ([]port.WorkoutSession, error) {
	if r.db == nil {
		log.Printf("[PostgresWorkoutSessionReader] DB pool is nil for userID=%s, returning empty sessions", userID)
		return []port.WorkoutSession{}, nil
	}

	var records []workoutSessionDBModel
	query := r.db.WithContext(ctx).
		Where("user_id = ? AND status IN ('COMPLETED', 'ABORTED')", userID)

	if !since.IsZero() {
		query = query.Where("(ended_at >= ? OR started_at >= ?)", since, since)
	}

	err := query.Order("ended_at DESC, started_at DESC").Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("fetch recent workout sessions: %w", err)
	}

	sessions := make([]port.WorkoutSession, 0, len(records))
	for _, rec := range records {
		completedAt := time.Now()
		if rec.EndedAt != nil {
			completedAt = *rec.EndedAt
		} else if rec.StartedAt != nil {
			completedAt = *rec.StartedAt
		}

		avgRPE := 0.0
		if rec.AverageRPE != nil {
			avgRPE = *rec.AverageRPE
		}

		avgForm := 0.0
		if rec.AverageFormScore != nil {
			avgForm = *rec.AverageFormScore
		}

		sessions = append(sessions, port.WorkoutSession{
			SessionID:        rec.ID,
			UserID:           rec.UserID,
			PlanID:           rec.PlanID,
			CompletedAt:      completedAt,
			TotalSets:        rec.TotalSets,
			AverageRPE:       avgRPE,
			AverageFormScore: avgForm,
			Aborted:          rec.Status == "ABORTED",
		})
	}

	return sessions, nil
}

// GetSetLogs fetches recent set logs for a user and exerciseID.
// If set logs are not found, it checks personal_records. If still not found, returns an empty slice []port.SetLog{}.
func (r *PostgresWorkoutSessionReader) GetSetLogs(ctx context.Context, userID string, exerciseID string, limit int) ([]port.SetLog, error) {
	if r.db == nil {
		log.Printf("[PostgresWorkoutSessionReader] DB pool is nil for userID=%s exerciseID=%s, returning empty set logs", userID, exerciseID)
		return []port.SetLog{}, nil
	}

	if limit <= 0 {
		limit = 10
	}

	var setLogs []workoutSetLogDBModel
	subQuery := r.db.WithContext(ctx).Model(&workoutSessionDBModel{}).Select("id").Where("user_id = ?", userID)

	err := r.db.WithContext(ctx).Model(&workoutSetLogDBModel{}).
		Where("session_id IN (?) AND exercise_id = ?", subQuery, exerciseID).
		Order("created_at DESC").
		Limit(limit).
		Find(&setLogs).Error

	if err == nil && len(setLogs) > 0 {
		result := make([]port.SetLog, 0, len(setLogs))
		for _, logItem := range setLogs {
			result = append(result, port.SetLog{
				ExerciseID:  logItem.ExerciseID,
				Weight:      logItem.Weight,
				Reps:        logItem.ActualReps,
				RPE:         logItem.RPE,
				CompletedAt: logItem.CreatedAt,
			})
		}
		return result, nil
	}

	// Fallback to checking personal_records table if no set logs recorded yet
	var prRecord personalRecordDBModel
	prErr := r.db.WithContext(ctx).
		Where("user_id = ? AND exercise_id = ?", userID, exerciseID).
		First(&prRecord).Error

	if prErr == nil && prRecord.Weight > 0 {
		return []port.SetLog{
			{
				ExerciseID:  exerciseID,
				Weight:      prRecord.Weight,
				Reps:        prRecord.Reps,
				RPE:         8.0,
				CompletedAt: prRecord.AchievedAt,
			},
		}, nil
	}

	// No logs or PR recorded for this exercise yet -> return clean empty slice so agent can safely handle initial weight
	return []port.SetLog{}, nil
}
