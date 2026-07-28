package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func getDB(ctx context.Context, defaultDB *gorm.DB) *gorm.DB {
	return GetDB(ctx, defaultDB)
}

// PostgresWorkoutSessionRepository implements repository.WorkoutSessionRepository.
type PostgresWorkoutSessionRepository struct {
	db *gorm.DB
}

var _ repository.WorkoutSessionRepository = (*PostgresWorkoutSessionRepository)(nil)

// NewPostgresWorkoutSessionRepository constructs repository.
func NewPostgresWorkoutSessionRepository(db *gorm.DB) *PostgresWorkoutSessionRepository {
	return &PostgresWorkoutSessionRepository{db: db}
}

func (r *PostgresWorkoutSessionRepository) Save(ctx context.Context, session *aggregate.WorkoutSession) error {
	db := getDB(ctx, r.db)
	model := SessionToPersistence(session)

	err := db.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(model).Error

	if err != nil {
		return fmt.Errorf("failed to save workout session: %w", err)
	}

	for _, set := range model.Sets {
		if err := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&set).Error; err != nil {
			return fmt.Errorf("failed to save set log: %w", err)
		}
	}

	return nil
}

func (r *PostgresWorkoutSessionRepository) FindByID(ctx context.Context, id string) (*aggregate.WorkoutSession, error) {
	db := getDB(ctx, r.db)
	var model WorkoutSessionModel
	err := db.Preload("Sets.Reps").Preload("Errors").First(&model, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find session by id: %w", err)
	}
	return SessionToDomain(&model), nil
}

func (r *PostgresWorkoutSessionRepository) FindActiveSessionByUserID(ctx context.Context, userID string) (*aggregate.WorkoutSession, error) {
	db := getDB(ctx, r.db)
	var model WorkoutSessionModel
	err := db.Preload("Sets.Reps").Preload("Errors").
		First(&model, "user_id = ? AND status = ?", userID, "IN_PROGRESS").Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find active session: %w", err)
	}
	return SessionToDomain(&model), nil
}

func (r *PostgresWorkoutSessionRepository) FindTimedOutSessions(ctx context.Context, maxDurationMinutes int) ([]*aggregate.WorkoutSession, error) {
	db := getDB(ctx, r.db)
	threshold := time.Now().UTC().Add(-time.Duration(maxDurationMinutes) * time.Minute)

	var models []WorkoutSessionModel
	err := db.Preload("Sets.Reps").Preload("Errors").
		Where("status = ? AND started_at <= ?", "IN_PROGRESS", threshold).
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find timed out sessions: %w", err)
	}

	res := make([]*aggregate.WorkoutSession, len(models))
	for i := range models {
		res[i] = SessionToDomain(&models[i])
	}
	return res, nil
}

func (r *PostgresWorkoutSessionRepository) FindSessionsWithCriticalInactivity(
	ctx context.Context,
	inactivityThreshold time.Duration,
) ([]*aggregate.WorkoutSession, error) {
	db := getDB(ctx, r.db)
	updatedBefore := time.Now().UTC().Add(-inactivityThreshold)

	var models []WorkoutSessionModel
	err := db.Preload("Sets.Reps").Preload("Errors").
		Where(
			`status = ? AND updated_at <= ? AND EXISTS (
				SELECT 1 FROM workout_execution.session_errors se
				WHERE se.session_id = workout_execution.workout_sessions.id
				  AND (se.severity = 'CRITICAL'
				       OR se.error_code IN ('ERR_BAR_TRAPPED', 'ERR_FALL_DETECTED'))
			)`,
			"IN_PROGRESS", updatedBefore,
		).
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find sessions with critical inactivity: %w", err)
	}

	res := make([]*aggregate.WorkoutSession, len(models))
	for i := range models {
		res[i] = SessionToDomain(&models[i])
	}
	return res, nil
}

func (r *PostgresWorkoutSessionRepository) FindHistoryByUserID(
	ctx context.Context,
	userID string,
	limit, offset int,
) ([]*aggregate.WorkoutSession, error) {
	db := getDB(ctx, r.db)
	var models []WorkoutSessionModel
	err := db.Preload("Sets.Reps").Preload("Errors").
		Where("user_id = ?", userID).
		Order("started_at DESC").
		Limit(limit).Offset(offset).
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find workout history: %w", err)
	}

	res := make([]*aggregate.WorkoutSession, len(models))
	for i := range models {
		res[i] = SessionToDomain(&models[i])
	}
	return res, nil
}

// GetRecentVolumesForMuscleGroup implements service.SessionVolumeHistoryProvider.
func (r *PostgresWorkoutSessionRepository) GetRecentVolumesForMuscleGroup(
	ctx context.Context,
	userID, _ string,
	limit int,
) ([]float32, error) {
	db := getDB(ctx, r.db)
	var models []WorkoutSessionModel
	err := db.Where("user_id = ? AND status = ?", userID, "COMPLETED").
		Order("ended_at DESC").Limit(limit).Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch recent session volumes: %w", err)
	}

	vols := make([]float32, len(models))
	for i := range models {
		vols[i] = models[i].TotalVolume
	}
	return vols, nil
}

// PostgresPersonalRecordRepository implements repository.PersonalRecordRepository.
type PostgresPersonalRecordRepository struct {
	db *gorm.DB
}

var _ repository.PersonalRecordRepository = (*PostgresPersonalRecordRepository)(nil)

// NewPostgresPersonalRecordRepository constructs repo.
func NewPostgresPersonalRecordRepository(db *gorm.DB) *PostgresPersonalRecordRepository {
	return &PostgresPersonalRecordRepository{db: db}
}

func (r *PostgresPersonalRecordRepository) Save(ctx context.Context, pr *aggregate.PersonalRecord) error {
	db := getDB(ctx, r.db)
	model := PersonalRecordToPersistence(pr)
	err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "exercise_id"}},
		UpdateAll: true,
	}).Create(model).Error
	if err != nil {
		return fmt.Errorf("failed to save personal record: %w", err)
	}
	return nil
}

func (r *PostgresPersonalRecordRepository) FindByUserIDAndExerciseID(
	ctx context.Context,
	userID, exerciseID string,
) (*aggregate.PersonalRecord, error) {
	db := getDB(ctx, r.db)
	var model PersonalRecordModel
	err := db.First(&model, "user_id = ? AND exercise_id = ?", userID, exerciseID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find personal record: %w", err)
	}
	return PersonalRecordToDomain(&model), nil
}

func (r *PostgresPersonalRecordRepository) FindByUserIDAndExerciseIDs(
	ctx context.Context,
	userID string,
	exerciseIDs []string,
) ([]*aggregate.PersonalRecord, error) {
	db := getDB(ctx, r.db)
	var models []PersonalRecordModel
	q := db.Where("user_id = ?", userID)
	if len(exerciseIDs) > 0 {
		q = q.Where("exercise_id IN ?", exerciseIDs)
	}
	err := q.Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find personal records: %w", err)
	}

	res := make([]*aggregate.PersonalRecord, len(models))
	for i := range models {
		res[i] = PersonalRecordToDomain(&models[i])
	}
	return res, nil
}

// PostgresMotionSpecificationRepository implements repository.MotionSpecificationRepository.
type PostgresMotionSpecificationRepository struct {
	db *gorm.DB
}

var _ repository.MotionSpecificationRepository = (*PostgresMotionSpecificationRepository)(nil)

// NewPostgresMotionSpecificationRepository constructs repo.
func NewPostgresMotionSpecificationRepository(db *gorm.DB) *PostgresMotionSpecificationRepository {
	return &PostgresMotionSpecificationRepository{db: db}
}

func (r *PostgresMotionSpecificationRepository) Save(ctx context.Context, spec *aggregate.MotionSpecification) error {
	db := getDB(ctx, r.db)
	dialogueBytes, _ := json.Marshal(spec.DialogueEngine())

	model := &MotionSpecificationModel{
		ExerciseID:             spec.ExerciseID(),
		OnnxModelURL:           spec.OnnxModelURL(),
		LocalRulesURL:          spec.LocalRulesURL(),
		DialogueEngineJSON:     dialogueBytes,
		RecommendedCameraAngle: spec.RecommendedCameraAngle(),
		CreatedAt:              time.Now().UTC(),
		UpdatedAt:              time.Now().UTC(),
	}

	err := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(model).Error
	if err != nil {
		return fmt.Errorf("failed to save motion spec: %w", err)
	}
	return nil
}

func (r *PostgresMotionSpecificationRepository) FindByExerciseID(ctx context.Context, exerciseID string) (*aggregate.MotionSpecification, error) {
	db := getDB(ctx, r.db)
	var model MotionSpecificationModel
	err := db.First(&model, "exercise_id = ?", exerciseID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find motion specification: %w", err)
	}

	var dialogue vo.DialogueEngineConfig
	if len(model.DialogueEngineJSON) > 0 {
		_ = json.Unmarshal(model.DialogueEngineJSON, &dialogue)
	}

	return aggregate.NewMotionSpecification(
		model.ExerciseID, model.OnnxModelURL, model.LocalRulesURL,
		dialogue, model.RecommendedCameraAngle,
	), nil
}

// PostgresOutboxRepository implements port.OutboxRepository.
type PostgresOutboxRepository struct {
	db *gorm.DB
}

var _ port.OutboxRepository = (*PostgresOutboxRepository)(nil)

// NewPostgresOutboxRepository constructs repo.
func NewPostgresOutboxRepository(db *gorm.DB) *PostgresOutboxRepository {
	return &PostgresOutboxRepository{db: db}
}

func (r *PostgresOutboxRepository) Save(ctx context.Context, record *port.OutboxRecord) error {
	db := getDB(ctx, r.db)
	eventID := record.EventID
	if eventID == "" {
		eventID = record.ID
	}
	partitionKey := record.PartitionKey
	if partitionKey == "" {
		partitionKey = record.AggregateID
	}
	var pubAt sql.NullTime
	if record.PublishedAt != nil {
		pubAt = sql.NullTime{Time: *record.PublishedAt, Valid: true}
	}
	model := OutboxModel{
		ID:            record.ID,
		EventID:       eventID,
		AggregateType: record.AggregateType,
		AggregateID:   record.AggregateID,
		EventType:     record.EventType,
		Payload:       record.Payload,
		PartitionKey:  partitionKey,
		Published:     record.Published,
		PublishedAt:   pubAt,
		CreatedAt:     record.CreatedAt,
	}
	return db.Create(&model).Error
}

func (r *PostgresOutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]*port.OutboxRecord, error) {
	db := getDB(ctx, r.db)
	var models []OutboxModel
	err := db.Where("published = ?", false).Order("created_at ASC").Limit(limit).Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch unpublished outbox events: %w", err)
	}

	res := make([]*port.OutboxRecord, len(models))
	for i, m := range models {
		res[i] = OutboxToDomain(&m)
	}
	return res, nil
}

func (r *PostgresOutboxRepository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	db := getDB(ctx, r.db)
	now := time.Now().UTC()
	return db.Model(&OutboxModel{}).Where("id IN ?", ids).Updates(map[string]interface{}{
		"published":    true,
		"published_at": now,
	}).Error
}

func (r *PostgresOutboxRepository) ExecuteInLock(ctx context.Context, lockID int64, fn func(txCtx context.Context) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var acquired bool
		if err := tx.Raw("SELECT pg_try_advisory_xact_lock(?)", lockID).Scan(&acquired).Error; err != nil {
			return err
		}
		if !acquired {
			return nil
		}
		txCtx := WithTx(ctx, tx)
		return fn(txCtx)
	})
}
