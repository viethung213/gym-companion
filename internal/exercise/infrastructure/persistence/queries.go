package persistence

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/exercise/application"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"gorm.io/gorm"
)

func replaceSecondaryMuscles(tx *gorm.DB, info *domain.Info) error {
	err := tx.Where("exercise_id = ?", info.ID).
		Delete(&exerciseSecondaryMuscleRecord{}).
		Error
	if err != nil {
		return fmt.Errorf("delete secondary muscles: %w", err)
	}

	records := newSecondaryMuscleRecords(info.ID, info.SecondaryMuscleIDs)
	if len(records) == 0 {
		return nil
	}
	if err := tx.Create(&records).Error; err != nil {
		return fmt.Errorf("insert secondary muscles: %w", err)
	}

	return nil
}

func replaceTags(tx *gorm.DB, info *domain.Info) error {
	if err := tx.Where("exercise_id = ?", info.ID).Delete(&exerciseTagRecord{}).Error; err != nil {
		return fmt.Errorf("delete exercise tags: %w", err)
	}

	records := newTagRecords(info.ID, info.TagIDs)
	if len(records) == 0 {
		return nil
	}
	if err := tx.Create(&records).Error; err != nil {
		return fmt.Errorf("insert exercise tags: %w", err)
	}

	return nil
}

func (r *PostgresRepository) searchActiveRecords(
	ctx context.Context,
	filters *application.SearchFilters,
) ([]exerciseRecord, error) {
	query := r.db.WithContext(ctx).
		Table("exercise.exercises AS e").
		Select("e.*").
		Where("e.status = ?", string(domain.StatusActive))

	if filters.BodyPartID != "" {
		query = query.Where("e.body_part_id = ?", filters.BodyPartID)
	}
	if filters.EquipmentID != "" {
		query = query.Where("e.equipment_id = ?", filters.EquipmentID)
	}
	if filters.TargetMuscleID != "" {
		query = query.Where("e.target_muscle_id = ?", filters.TargetMuscleID)
	}
	if filters.Keyword != "" {
		query = query.Where("e.name ILIKE '%' || ? || '%'", filters.Keyword)
	}
	if filters.Difficulty != "" {
		query = query.Where("e.difficulty = ?", filters.Difficulty)
	}
	for _, muscleID := range filters.SecondaryMuscleIDs {
		query = query.Where(
			`EXISTS (
				SELECT 1 FROM exercise.exercise_secondary_muscles esm
				WHERE esm.exercise_id = e.id AND esm.muscle_id = ?
			)`,
			muscleID,
		)
	}
	for _, tagID := range filters.TagIDs {
		query = query.Where(
			`EXISTS (
				SELECT 1 FROM exercise.exercise_tags et
				WHERE et.exercise_id = e.id AND et.tag_id = ?
			)`,
			tagID,
		)
	}

	limit := filters.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	query = query.Order("e.name").Limit(int(limit))
	if filters.Offset > 0 {
		query = query.Offset(int(filters.Offset))
	}

	var records []exerciseRecord
	if err := query.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("query active exercises: %w", err)
	}

	return records, nil
}

func querySecondaryMuscleIDs(
	ctx context.Context,
	db *gorm.DB,
	exerciseID string,
) ([]string, error) {
	var ids []string
	err := db.WithContext(ctx).
		Model(&exerciseSecondaryMuscleRecord{}).
		Where("exercise_id = ?", exerciseID).
		Order("muscle_id").
		Pluck("muscle_id", &ids).
		Error
	if err != nil {
		return nil, fmt.Errorf("load secondary muscles: %w", err)
	}

	return ids, nil
}

func queryTagIDs(ctx context.Context, db *gorm.DB, exerciseID string) ([]string, error) {
	var ids []string
	err := db.WithContext(ctx).
		Model(&exerciseTagRecord{}).
		Where("exercise_id = ?", exerciseID).
		Order("tag_id").
		Pluck("tag_id", &ids).
		Error
	if err != nil {
		return nil, fmt.Errorf("load tags: %w", err)
	}

	return ids, nil
}

func queryBodyParts(ctx context.Context, db *gorm.DB) ([]application.BodyPart, error) {
	var records []bodyPartRecord
	if err := db.WithContext(ctx).Order("name").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("query body parts: %w", err)
	}

	bodyParts := make([]application.BodyPart, 0, len(records))
	for _, record := range records {
		bodyParts = append(bodyParts, application.BodyPart{ID: record.ID, Name: record.Name})
	}

	return bodyParts, nil
}

func queryEquipments(ctx context.Context, db *gorm.DB) ([]application.Equipment, error) {
	var records []equipmentRecord
	if err := db.WithContext(ctx).Order("name").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("query equipments: %w", err)
	}

	equipments := make([]application.Equipment, 0, len(records))
	for _, record := range records {
		equipments = append(equipments, application.Equipment{ID: record.ID, Name: record.Name})
	}

	return equipments, nil
}

func queryMuscles(ctx context.Context, db *gorm.DB) ([]application.Muscle, error) {
	var records []muscleRecord
	if err := db.WithContext(ctx).Order("name").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("query muscles: %w", err)
	}

	muscles := make([]application.Muscle, 0, len(records))
	for _, record := range records {
		muscles = append(muscles, application.Muscle{
			ID:         record.ID,
			Name:       record.Name,
			BodyPartID: record.BodyPartID,
		})
	}

	return muscles, nil
}

func queryTags(ctx context.Context, db *gorm.DB) ([]application.Tag, error) {
	var records []tagRecord
	if err := db.WithContext(ctx).Order("name").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}

	tags := make([]application.Tag, 0, len(records))
	for _, record := range records {
		tags = append(tags, application.Tag{ID: record.ID, Name: record.Name})
	}

	return tags, nil
}
