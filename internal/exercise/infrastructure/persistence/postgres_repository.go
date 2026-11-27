// Package persistence contains storage adapters for Exercise.
package persistence

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"

	"github.com/lib/pq"
	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// slugConstraint is the unique constraint on exercises.slug (02-exercise.sql).
const slugConstraint = "uq_exercises_slug"

type PostgresRepository struct {
	db *gorm.DB
}

var _ port.Repository = (*PostgresRepository)(nil)

func NewPostgresRepository(db *gorm.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Save(
	ctx context.Context,
	exercise *domain.Exercise,
	event *domain.Event,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		info := exercise.Info()
		if err := saveExercise(tx, &info); err != nil {
			return err
		}
		if err := insertOutbox(tx, event); err != nil {
			return err
		}

		return nil
	})
}

func (r *PostgresRepository) FindByID(
	ctx context.Context,
	id string,
) (*domain.Exercise, error) {
	var record exerciseRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrExerciseNotFound
		}

		return nil, fmt.Errorf("find exercise: %w", err)
	}

	exercise, err := r.rehydrate(ctx, &record)
	if err != nil {
		return nil, err
	}

	return exercise, nil
}

func (r *PostgresRepository) SearchActive(
	ctx context.Context,
	filters *port.SearchFilters,
) ([]*domain.Exercise, int32, error) {
	records, totalCount, err := r.searchActiveRecords(ctx, filters)
	if err != nil {
		return nil, 0, err
	}

	exercises, err := r.rehydrateBatch(ctx, records)
	if err != nil {
		return nil, 0, err
	}

	return exercises, totalCount, nil
}

func (r *PostgresRepository) GetRandomAISupported(
	ctx context.Context,
) (*domain.Exercise, error) {
	var ids []string
	err := r.db.WithContext(ctx).
		Model(&exerciseRecord{}).
		Where("status = ?", string(domain.StatusActive)).
		Where("has_ai_supported = ?", true).
		Pluck("id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("find random ai exercise ids: %w", err)
	}
	if len(ids) == 0 {
		return nil, domain.ErrExerciseNotFound
	}

	chosenID := ids[rand.IntN(len(ids))]
	return r.FindByID(ctx, chosenID)
}

func saveExercise(tx *gorm.DB, info *domain.Info) error {
	record := newExerciseRecord(info)
	err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name",
			"primary_muscle",
			"secondary_muscles",
			"equipment",
			"category",
			"level",
			"force",
			"mechanic",
			"instructions",
			"images",
			"video_url",
			"measurement_type",
			"default_rest_seconds",
			"motion_slug",
			"has_ai_supported",
			"status",
			"archived_at",
			"updated_at",
		}),
	}).Create(&record).Error
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Constraint == slugConstraint {
			return fmt.Errorf("%w: %s", domain.ErrDuplicateSlug, info.Slug)
		}
		return fmt.Errorf("upsert exercise: %w", err)
	}

	return nil
}

func (r *PostgresRepository) rehydrate(
	ctx context.Context,
	record *exerciseRecord,
) (*domain.Exercise, error) {
	exercises, err := r.rehydrateBatch(ctx, []exerciseRecord{*record})
	if err != nil {
		return nil, err
	}
	if len(exercises) == 0 {
		return nil, domain.ErrExerciseNotFound
	}

	return exercises[0], nil
}

func (r *PostgresRepository) rehydrateBatch(
	_ context.Context,
	records []exerciseRecord,
) ([]*domain.Exercise, error) {
	exercises := make([]*domain.Exercise, 0, len(records))
	for i := range records {
		exercise, err := domain.RehydrateExercise(records[i].toDomainInfo())
		if err != nil {
			return nil, fmt.Errorf("rehydrate exercise %s: %w", records[i].ID, err)
		}
		exercises = append(exercises, exercise)
	}

	return exercises, nil
}

func (r *PostgresRepository) SetAISupportedWithOutboxLog(
	ctx context.Context,
	exerciseID string,
	supported bool,
	logRecord *port.OutboxLogRecord,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if logRecord != nil && logRecord.EventID != "" {
			var count int64
			if err := tx.Model(&outboxLogRecord{}).Where("event_id = ?", logRecord.EventID).Count(&count).Error; err != nil {
				return fmt.Errorf("check outbox_log: %w", err)
			}
			if count > 0 {
				return nil
			}
		}

		var exRecord exerciseRecord
		if err := tx.Where("id = ?", exerciseID).First(&exRecord).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrExerciseNotFound
			}
			return fmt.Errorf("find exercise for ai_supported: %w", err)
		}

		if err := tx.Model(&exerciseRecord{}).
			Where("id = ?", exerciseID).
			Updates(map[string]interface{}{
				"has_ai_supported": supported,
			}).Error; err != nil {
			return fmt.Errorf("update has_ai_supported: %w", err)
		}

		if logRecord != nil && logRecord.EventID != "" {
			id := logRecord.ID
			if id == "" {
				id = logRecord.EventID
			}
			status := logRecord.Status
			if status == "" {
				status = "SUCCESS"
			}
			rec := outboxLogRecord{
				ID:           id,
				EventID:      logRecord.EventID,
				EventType:    logRecord.EventType,
				Payload:      logRecord.Payload,
				PartitionKey: exerciseID,
				Status:       status,
				ErrorMessage: logRecord.ErrorMessage,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "event_id"}},
				DoNothing: true,
			}).Create(&rec).Error; err != nil {
				return fmt.Errorf("insert outbox_log: %w", err)
			}
		}

		return nil
	})
}
