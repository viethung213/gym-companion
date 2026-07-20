package persistence

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"gorm.io/gorm"
)

func replaceSecondaryMuscles(tx *gorm.DB, info *domain.Info) error {
	var currentIDs []string
	err := tx.Model(&exerciseSecondaryMuscleRecord{}).
		Where("exercise_id = ?", info.ID).
		Pluck("muscle_id", &currentIDs).
		Error
	if err != nil {
		return fmt.Errorf("load current secondary muscles for diff: %w", err)
	}

	currentMap := make(map[string]bool, len(currentIDs))
	for _, id := range currentIDs {
		currentMap[id] = true
	}

	newMap := make(map[string]bool, len(info.SecondaryMuscleIDs))
	for _, id := range info.SecondaryMuscleIDs {
		newMap[id] = true
	}

	var toAdd []string
	for _, id := range info.SecondaryMuscleIDs {
		if !currentMap[id] {
			toAdd = append(toAdd, id)
		}
	}

	var toDelete []string
	for _, id := range currentIDs {
		if !newMap[id] {
			toDelete = append(toDelete, id)
		}
	}

	if len(toDelete) > 0 {
		err := tx.Where("exercise_id = ? AND muscle_id IN ?", info.ID, toDelete).
			Delete(&exerciseSecondaryMuscleRecord{}).
			Error
		if err != nil {
			return fmt.Errorf("delete secondary muscles: %w", err)
		}
	}

	if len(toAdd) > 0 {
		records := newSecondaryMuscleRecords(info.ID, toAdd)
		if err := tx.Create(&records).Error; err != nil {
			return fmt.Errorf("insert secondary muscles: %w", err)
		}
	}

	return nil
}

func replaceTags(tx *gorm.DB, info *domain.Info) error {
	var currentIDs []string
	err := tx.Model(&exerciseTagRecord{}).
		Where("exercise_id = ?", info.ID).
		Pluck("tag_id", &currentIDs).
		Error
	if err != nil {
		return fmt.Errorf("load current exercise tags for diff: %w", err)
	}

	currentMap := make(map[string]bool, len(currentIDs))
	for _, id := range currentIDs {
		currentMap[id] = true
	}

	newMap := make(map[string]bool, len(info.TagIDs))
	for _, id := range info.TagIDs {
		newMap[id] = true
	}

	var toAdd []string
	for _, id := range info.TagIDs {
		if !currentMap[id] {
			toAdd = append(toAdd, id)
		}
	}

	var toDelete []string
	for _, id := range currentIDs {
		if !newMap[id] {
			toDelete = append(toDelete, id)
		}
	}

	if len(toDelete) > 0 {
		err := tx.Where("exercise_id = ? AND tag_id IN ?", info.ID, toDelete).
			Delete(&exerciseTagRecord{}).
			Error
		if err != nil {
			return fmt.Errorf("delete exercise tags: %w", err)
		}
	}

	if len(toAdd) > 0 {
		records := newTagRecords(info.ID, toAdd)
		if err := tx.Create(&records).Error; err != nil {
			return fmt.Errorf("insert exercise tags: %w", err)
		}
	}

	return nil
}

func (r *PostgresRepository) searchActiveRecords(
	ctx context.Context,
	filters *port.SearchFilters,
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
	if len(filters.TargetMuscleIDs) > 0 {
		query = query.Where("LOWER(e.target_muscle_id) IN ?", lowerStrings(filters.TargetMuscleIDs))
	}
	if len(filters.AvoidMuscleIDs) > 0 {
		avoided := lowerStrings(filters.AvoidMuscleIDs)
		query = query.
			Where("LOWER(e.target_muscle_id) NOT IN ?", avoided).
			Where(
				`NOT EXISTS (
					SELECT 1 FROM exercise.exercise_secondary_muscles esm
					WHERE esm.exercise_id = e.id AND LOWER(esm.muscle_id) IN ?
				)`,
				avoided,
			)
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

func lowerStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			result = append(result, value)
		}
	}
	return result
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

func queryBodyParts(ctx context.Context, db *gorm.DB) ([]port.BodyPart, error) {
	var records []bodyPartRecord
	if err := db.WithContext(ctx).Order("name").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("query body parts: %w", err)
	}

	bodyParts := make([]port.BodyPart, 0, len(records))
	for _, record := range records {
		bodyParts = append(bodyParts, port.BodyPart{ID: record.ID, Name: record.Name})
	}

	return bodyParts, nil
}

func queryEquipments(ctx context.Context, db *gorm.DB) ([]port.Equipment, error) {
	var records []equipmentRecord
	if err := db.WithContext(ctx).Order("name").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("query equipments: %w", err)
	}

	equipments := make([]port.Equipment, 0, len(records))
	for _, record := range records {
		equipments = append(equipments, port.Equipment{ID: record.ID, Name: record.Name})
	}

	return equipments, nil
}

func queryMuscles(ctx context.Context, db *gorm.DB) ([]port.Muscle, error) {
	var records []muscleRecord
	if err := db.WithContext(ctx).Order("name").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("query muscles: %w", err)
	}

	muscles := make([]port.Muscle, 0, len(records))
	for _, record := range records {
		muscles = append(muscles, port.Muscle{
			ID:         record.ID,
			Name:       record.Name,
			BodyPartID: record.BodyPartID,
		})
	}

	return muscles, nil
}

func queryTags(ctx context.Context, db *gorm.DB) ([]port.Tag, error) {
	var records []tagRecord
	if err := db.WithContext(ctx).Order("name").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}

	tags := make([]port.Tag, 0, len(records))
	for _, record := range records {
		tags = append(tags, port.Tag{ID: record.ID, Name: record.Name})
	}

	return tags, nil
}
