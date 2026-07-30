// Package persistence contains storage adapters for Exercise.
package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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
		if err := replaceSecondaryMuscles(tx, &info); err != nil {
			return err
		}
		if err := replaceTags(tx, &info); err != nil {
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
) ([]*domain.Exercise, error) {
	records, err := r.searchActiveRecords(ctx, filters)
	if err != nil {
		return nil, err
	}

	exercises := make([]*domain.Exercise, 0, len(records))
	for i := range records {
		exercise, err := r.rehydrate(ctx, &records[i])
		if err != nil {
			return nil, err
		}
		exercises = append(exercises, exercise)
	}

	return exercises, nil
}

func (r *PostgresRepository) GetMetadata(ctx context.Context) (port.Metadata, error) {
	bodyParts, err := queryBodyParts(ctx, r.db)
	if err != nil {
		return port.Metadata{}, err
	}
	equipments, err := queryEquipments(ctx, r.db)
	if err != nil {
		return port.Metadata{}, err
	}
	muscles, err := queryMuscles(ctx, r.db)
	if err != nil {
		return port.Metadata{}, err
	}
	tags, err := queryTags(ctx, r.db)
	if err != nil {
		return port.Metadata{}, err
	}

	return port.Metadata{
		BodyParts:  bodyParts,
		Equipments: equipments,
		Muscles:    muscles,
		Tags:       tags,
	}, nil
}

func saveExercise(tx *gorm.DB, info *domain.Info) error {
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

func (r *PostgresRepository) rehydrate(
	ctx context.Context,
	record *exerciseRecord,
) (*domain.Exercise, error) {
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

// BodyPart CRUD
func (r *PostgresRepository) CreateBodyPart(ctx context.Context, bp *port.BodyPart) error {
	record := bodyPartRecord{ID: bp.ID, Name: bp.Name}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return fmt.Errorf("create body part: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetBodyPart(ctx context.Context, id string) (*port.BodyPart, error) {
	var record bodyPartRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrBodyPartNotFound
		}
		return nil, fmt.Errorf("get body part: %w", err)
	}
	return &port.BodyPart{ID: record.ID, Name: record.Name}, nil
}

func (r *PostgresRepository) ListBodyParts(ctx context.Context, limit, offset int) ([]port.BodyPart, int, error) {
	var records []bodyPartRecord
	var total int64

	if err := r.db.WithContext(ctx).Model(&bodyPartRecord{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count body parts: %w", err)
	}

	if err := r.db.WithContext(ctx).Order("name").Limit(limit).Offset(offset).Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("list body parts: %w", err)
	}

	bps := make([]port.BodyPart, 0, len(records))
	for _, record := range records {
		bps = append(bps, port.BodyPart{ID: record.ID, Name: record.Name})
	}
	return bps, int(total), nil
}

func (r *PostgresRepository) UpdateBodyPart(ctx context.Context, bp *port.BodyPart) error {
	result := r.db.WithContext(ctx).Model(&bodyPartRecord{}).Where("id = ?", bp.ID).Update("name", bp.Name)
	if result.Error != nil {
		return fmt.Errorf("update body part: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrBodyPartNotFound
	}
	return nil
}

func (r *PostgresRepository) DeleteBodyPart(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&bodyPartRecord{}, "id = ?", id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return domain.ErrBodyPartNotFound
		}
		return fmt.Errorf("delete body part: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrBodyPartNotFound
	}
	return nil
}

// Equipment CRUD
func (r *PostgresRepository) CreateEquipment(ctx context.Context, eq *port.Equipment) error {
	record := equipmentRecord{ID: eq.ID, Name: eq.Name}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return fmt.Errorf("create equipment: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetEquipment(ctx context.Context, id string) (*port.Equipment, error) {
	var record equipmentRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrEquipmentNotFound
		}
		return nil, fmt.Errorf("get equipment: %w", err)
	}
	return &port.Equipment{ID: record.ID, Name: record.Name}, nil
}

func (r *PostgresRepository) ListEquipments(ctx context.Context, limit, offset int) ([]port.Equipment, int, error) {
	var records []equipmentRecord
	var total int64

	if err := r.db.WithContext(ctx).Model(&equipmentRecord{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count equipments: %w", err)
	}

	if err := r.db.WithContext(ctx).Order("name").Limit(limit).Offset(offset).Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("list equipments: %w", err)
	}

	eqs := make([]port.Equipment, 0, len(records))
	for _, record := range records {
		eqs = append(eqs, port.Equipment{ID: record.ID, Name: record.Name})
	}
	return eqs, int(total), nil
}

func (r *PostgresRepository) UpdateEquipment(ctx context.Context, eq *port.Equipment) error {
	result := r.db.WithContext(ctx).Model(&equipmentRecord{}).Where("id = ?", eq.ID).Update("name", eq.Name)
	if result.Error != nil {
		return fmt.Errorf("update equipment: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrEquipmentNotFound
	}
	return nil
}

func (r *PostgresRepository) DeleteEquipment(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&equipmentRecord{}, "id = ?", id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return domain.ErrEquipmentNotFound
		}
		return fmt.Errorf("delete equipment: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrEquipmentNotFound
	}
	return nil
}

// Muscle CRUD
func (r *PostgresRepository) CreateMuscle(ctx context.Context, m *port.Muscle) error {
	record := muscleRecord{ID: m.ID, Name: m.Name, BodyPartID: m.BodyPartID}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return fmt.Errorf("create muscle: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetMuscle(ctx context.Context, id string) (*port.Muscle, error) {
	var record muscleRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrMuscleNotFound
		}
		return nil, fmt.Errorf("get muscle: %w", err)
	}
	return &port.Muscle{ID: record.ID, Name: record.Name, BodyPartID: record.BodyPartID}, nil
}

func (r *PostgresRepository) ListMuscles(ctx context.Context, bodyPartID string, limit, offset int) ([]port.Muscle, int, error) {
	var records []muscleRecord
	var total int64

	query := r.db.WithContext(ctx)
	if bodyPartID != "" {
		query = query.Where("body_part_id = ?", bodyPartID)
	}

	if err := query.Model(&muscleRecord{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count muscles: %w", err)
	}

	if err := query.Order("name").Limit(limit).Offset(offset).Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("list muscles: %w", err)
	}

	ms := make([]port.Muscle, 0, len(records))
	for _, record := range records {
		ms = append(ms, port.Muscle{ID: record.ID, Name: record.Name, BodyPartID: record.BodyPartID})
	}
	return ms, int(total), nil
}

func (r *PostgresRepository) UpdateMuscle(ctx context.Context, m *port.Muscle) error {
	result := r.db.WithContext(ctx).Model(&muscleRecord{}).Where("id = ?", m.ID).
		Updates(map[string]interface{}{"name": m.Name, "body_part_id": m.BodyPartID})
	if result.Error != nil {
		return fmt.Errorf("update muscle: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrMuscleNotFound
	}
	return nil
}

func (r *PostgresRepository) DeleteMuscle(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&muscleRecord{}, "id = ?", id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return domain.ErrMuscleNotFound
		}
		return fmt.Errorf("delete muscle: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrMuscleNotFound
	}
	return nil
}

// Tag CRUD
func (r *PostgresRepository) CreateTag(ctx context.Context, t *port.Tag) error {
	record := tagRecord{ID: t.ID, Name: t.Name}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return fmt.Errorf("create tag: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetTag(ctx context.Context, id string) (*port.Tag, error) {
	var record tagRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrTagNotFound
		}
		return nil, fmt.Errorf("get tag: %w", err)
	}
	return &port.Tag{ID: record.ID, Name: record.Name}, nil
}

func (r *PostgresRepository) ListTags(ctx context.Context, limit, offset int) ([]port.Tag, int, error) {
	var records []tagRecord
	var total int64

	if err := r.db.WithContext(ctx).Model(&tagRecord{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count tags: %w", err)
	}

	if err := r.db.WithContext(ctx).Order("name").Limit(limit).Offset(offset).Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("list tags: %w", err)
	}

	ts := make([]port.Tag, 0, len(records))
	for _, record := range records {
		ts = append(ts, port.Tag{ID: record.ID, Name: record.Name})
	}
	return ts, int(total), nil
}

func (r *PostgresRepository) UpdateTag(ctx context.Context, t *port.Tag) error {
	result := r.db.WithContext(ctx).Model(&tagRecord{}).Where("id = ?", t.ID).Update("name", t.Name)
	if result.Error != nil {
		return fmt.Errorf("update tag: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrTagNotFound
	}
	return nil
}

func (r *PostgresRepository) DeleteTag(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&tagRecord{}, "id = ?", id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return domain.ErrTagNotFound
		}
		return fmt.Errorf("delete tag: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrTagNotFound
	}
	return nil
}
