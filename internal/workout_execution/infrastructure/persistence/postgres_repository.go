package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
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

	var count int64
	if err := db.Model(&WorkoutSessionModel{}).Where("id = ?", model.ID).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check session existence: %w", err)
	}

	if count == 0 {
		err := db.Create(model).Error
		if err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "uq_workout_sessions_active_user") {
				return derror.ErrActiveSessionAlreadyExists
			}
			return fmt.Errorf("failed to save workout session: %w", err)
		}
	} else {
		oldVersion := session.Version()
		res := db.Model(&WorkoutSessionModel{}).
			Where("id = ? AND version = ?", model.ID, oldVersion).
			Updates(map[string]interface{}{
				"status":             model.Status,
				"total_sets":         model.TotalSets,
				"total_volume":       model.TotalVolume,
				"average_form_score": model.AverageFormScore,
				"average_rpe":        model.AverageRPE,
				"scheduled_at":       model.ScheduledAt,
				"started_at":         model.StartedAt,
				"ended_at":           model.EndedAt,
				"updated_at":         model.UpdatedAt,
				"version":            oldVersion + 1,
			})
		if res.Error != nil {
			return fmt.Errorf("failed to update workout session: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return derror.ErrOptimisticLocking
		}
	}

	for _, set := range model.Sets {
		if err := db.Omit("Reps").Clauses(clause.OnConflict{UpdateAll: true}).Create(&set).Error; err != nil {
			return fmt.Errorf("failed to save set log: %w", err)
		}
		if len(set.Reps) > 0 {
			if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&set.Reps).Error; err != nil {
				return fmt.Errorf("failed to save rep logs: %w", err)
			}
		}
	}

	for _, sessErr := range model.Errors {
		if err := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&sessErr).Error; err != nil {
			return fmt.Errorf("failed to save session error: %w", err)
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

func (r *PostgresWorkoutSessionRepository) FindByIDForUpdate(ctx context.Context, id string) (*aggregate.WorkoutSession, error) {
	db := getDB(ctx, r.db)
	var model WorkoutSessionModel
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Sets.Reps").Preload("Errors").First(&model, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find session by id for update: %w", err)
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

	model := &MotionSpecificationModel{
		ExerciseID:             spec.ExerciseID(),
		OnnxDetectorURL:        spec.OnnxDetectorURL(),
		OnnxSkeletonURL:        spec.OnnxSkeletonURL(),
		LocalRulesURL:          spec.LocalRulesURL(),
		DialogueEngineURL:      spec.DialogueEngineURL(),
		RecommendedCameraAngle: spec.RecommendedCameraAngle(),
		IsReady:                spec.IsReady(),
		CreatedAt:              spec.CreatedAt(),
		UpdatedAt:              spec.UpdatedAt(),
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
			return nil, derror.ErrNotFound
		}
		return nil, fmt.Errorf("failed to find motion specification: %w", err)
	}

	return aggregate.RestoreMotionSpecification(
		model.ExerciseID, model.OnnxDetectorURL, model.OnnxSkeletonURL,
		model.LocalRulesURL, model.DialogueEngineURL, model.RecommendedCameraAngle, model.IsReady,
		model.CreatedAt, model.UpdatedAt,
	), nil
}

func (r *PostgresMotionSpecificationRepository) Delete(ctx context.Context, exerciseID string) error {
	db := getDB(ctx, r.db)
	res := db.Delete(&MotionSpecificationModel{}, "exercise_id = ?", exerciseID)
	if res.Error != nil {
		return fmt.Errorf("failed to delete motion specification: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return derror.ErrNotFound
	}
	return nil
}

func (r *PostgresMotionSpecificationRepository) List(ctx context.Context, limit, offset int) ([]*aggregate.MotionSpecification, int, error) {
	db := getDB(ctx, r.db)

	var total int64
	if err := db.Model(&MotionSpecificationModel{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count motion specifications: %w", err)
	}

	var models []MotionSpecificationModel
	err := db.Limit(limit).Offset(offset).Order("exercise_id ASC").Find(&models).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list motion specifications: %w", err)
	}

	specs := make([]*aggregate.MotionSpecification, len(models))
	for i, m := range models {
		specs[i] = aggregate.RestoreMotionSpecification(
			m.ExerciseID, m.OnnxDetectorURL, m.OnnxSkeletonURL,
			m.LocalRulesURL, m.DialogueEngineURL, m.RecommendedCameraAngle, m.IsReady,
			m.CreatedAt, m.UpdatedAt,
		)
	}

	return specs, int(total), nil
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
		"status":       "PUBLISHED",
	}).Error
}

func (r *PostgresOutboxRepository) ClaimBatch(
	ctx context.Context,
	limit int,
	lockDuration time.Duration,
) ([]*port.OutboxRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if lockDuration <= 0 {
		lockDuration = 30 * time.Second
	}

	var res []*port.OutboxRecord
	now := time.Now().UTC()
	lockedUntil := now.Add(lockDuration)

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var models []OutboxModel
		err := tx.
			Clauses(clause.Locking{
				Strength: "UPDATE",
				Options:  "SKIP LOCKED",
			}).
			Where("published = ? AND (status = ? OR locked_until IS NULL OR locked_until < ?)", false, "PENDING", now).
			Order("created_at ASC").
			Limit(limit).
			Find(&models).
			Error

		if err != nil {
			return fmt.Errorf("failed to fetch unpublished outbox events for update: %w", err)
		}

		if len(models) == 0 {
			return nil
		}

		res = make([]*port.OutboxRecord, len(models))
		ids := make([]string, len(models))
		for i, m := range models {
			res[i] = OutboxToDomain(&m)
			ids[i] = m.ID
		}

		err = tx.Model(&OutboxModel{}).
			Where("id IN ?", ids).
			Updates(map[string]interface{}{
				"status":       "PROCESSING",
				"locked_until": lockedUntil,
			}).
			Error

		if err != nil {
			return fmt.Errorf("failed to claim outbox events: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return res, nil
}

func (r *PostgresOutboxRepository) ProcessBatch(
	ctx context.Context,
	limit int,
	publishFn func(ctx context.Context, records []*port.OutboxRecord) error,
) error {
	records, err := r.ClaimBatch(ctx, limit, 30*time.Second)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}

	if pubErr := publishFn(ctx, records); pubErr != nil {
		return fmt.Errorf("publish outbox batch: %w", pubErr)
	}

	ids := make([]string, len(records))
	for i, rec := range records {
		ids[i] = rec.ID
	}

	return r.MarkPublished(ctx, ids)
}

// PostgresOutboxLogRepository implements port.OutboxLogRepository for consumer idempotency tracking.
type PostgresOutboxLogRepository struct {
	db *gorm.DB
}

var _ port.OutboxLogRepository = (*PostgresOutboxLogRepository)(nil)

// NewPostgresOutboxLogRepository constructs repository.
func NewPostgresOutboxLogRepository(db *gorm.DB) *PostgresOutboxLogRepository {
	return &PostgresOutboxLogRepository{db: db}
}

func (r *PostgresOutboxLogRepository) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	if eventID == "" {
		return false, nil
	}
	db := getDB(ctx, r.db)
	var count int64
	err := db.Model(&OutboxLogModel{}).
		Where("event_id = ? AND status = ?", eventID, "PROCESSED").
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check outbox_log idempotency: %w", err)
	}
	return count > 0, nil
}

func (r *PostgresOutboxLogRepository) SaveLog(ctx context.Context, record *port.OutboxLogRecord) error {
	if record == nil {
		return nil
	}
	db := getDB(ctx, r.db)

	id := record.ID
	if id == "" {
		id = record.EventID
	}
	if id == "" {
		id = uuid.New().String()
	}

	model := &OutboxLogModel{
		ID:           id,
		EventID:      record.EventID,
		EventType:    record.EventType,
		Payload:      record.Payload,
		PartitionKey: record.PartitionKey,
		ProcessedAt:  time.Now().UTC(),
		Status:       record.Status,
		ErrorMessage: record.ErrorMessage,
	}

	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(model).Error; err != nil {
		return fmt.Errorf("failed to save outbox_log record: %w", err)
	}
	return nil
}
