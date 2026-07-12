// Package persistence contains storage adapters for Exercise.
package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/viethung213/gym-companion/internal/exercise/application"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PostgresRepository struct {
	db *gorm.DB
}

var _ application.Repository = (*PostgresRepository)(nil)

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
		if err := saveExercise(tx, info); err != nil {
			return err
		}
		if err := replaceSecondaryMuscles(tx, info); err != nil {
			return err
		}
		if err := replaceTags(tx, info); err != nil {
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

	exercise, err := r.rehydrate(ctx, record)
	if err != nil {
		return nil, err
	}

	return exercise, nil
}

func (r *PostgresRepository) SearchActive(
	ctx context.Context,
	filters application.SearchFilters,
) ([]*domain.Exercise, error) {
	records, err := r.searchActiveRecords(ctx, filters)
	if err != nil {
		return nil, err
	}

	exercises := make([]*domain.Exercise, 0, len(records))
	for _, record := range records {
		exercise, err := r.rehydrate(ctx, record)
		if err != nil {
			return nil, err
		}
		exercises = append(exercises, exercise)
	}

	return exercises, nil
}

func (r *PostgresRepository) GetMetadata(ctx context.Context) (application.Metadata, error) {
	bodyParts, err := queryBodyParts(ctx, r.db)
	if err != nil {
		return application.Metadata{}, err
	}
	equipments, err := queryEquipments(ctx, r.db)
	if err != nil {
		return application.Metadata{}, err
	}
	muscles, err := queryMuscles(ctx, r.db)
	if err != nil {
		return application.Metadata{}, err
	}
	tags, err := queryTags(ctx, r.db)
	if err != nil {
		return application.Metadata{}, err
	}

	return application.Metadata{
		BodyParts:  bodyParts,
		Equipments: equipments,
		Muscles:    muscles,
		Tags:       tags,
	}, nil
}

func saveExercise(tx *gorm.DB, info domain.Info) error {
	record := newExerciseRecord(info)
	err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name",
			"body_part_id",
			"equipment_id",
			"target_muscle_id",
			"instructions",
			"thumbnail_url",
			"media_url",
			"video_url",
			"difficulty",
			"default_rest_seconds",
			"status",
			"archived_at",
			"updated_at",
		}),
	}).Create(&record).Error
	if err != nil {
		return fmt.Errorf("upsert exercise: %w", err)
	}

	return nil
}

func (r *PostgresRepository) rehydrate(ctx context.Context, record exerciseRecord) (*domain.Exercise, error) {
	secondaryMuscleIDs, err := querySecondaryMuscleIDs(ctx, r.db, record.ID)
	if err != nil {
		return nil, err
	}
	tagIDs, err := queryTagIDs(ctx, r.db, record.ID)
	if err != nil {
		return nil, err
	}

	exercise, err := domain.RehydrateExercise(record.toDomainInfo(secondaryMuscleIDs, tagIDs))
	if err != nil {
		return nil, err
	}

	return exercise, nil
}
