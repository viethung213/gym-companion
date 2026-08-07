package adapters

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"gorm.io/gorm"
)

// exerciseDBModel represents the database model for exercise.exercises table.
type exerciseDBModel struct {
	ID             string `gorm:"column:id;primaryKey"`
	Name           string `gorm:"column:name"`
	BodyPartID     string `gorm:"column:body_part_id"`
	EquipmentID    string `gorm:"column:equipment_id"`
	TargetMuscleID string `gorm:"column:target_muscle_id"`
	Difficulty     string `gorm:"column:difficulty"`
	Status         string `gorm:"column:status"`
}

func (exerciseDBModel) TableName() string {
	return "exercise.exercises"
}

// PostgresExerciseCatalogReader implements port.ExerciseCatalogReader querying exercise schema in PostgreSQL.
type PostgresExerciseCatalogReader struct {
	db *gorm.DB
}

var _ port.ExerciseCatalogReader = (*PostgresExerciseCatalogReader)(nil)

// NewPostgresExerciseCatalogReader creates a new PostgresExerciseCatalogReader.
func NewPostgresExerciseCatalogReader(db *gorm.DB) *PostgresExerciseCatalogReader {
	return &PostgresExerciseCatalogReader{db: db}
}

// SearchByFilter queries active exercises matching the filter options.
func (r *PostgresExerciseCatalogReader) SearchByFilter(ctx context.Context, filter *port.ExerciseFilter) ([]port.Exercise, error) {
	if r.db == nil {
		log.Printf("[PostgresExerciseCatalogReader] DB pool is nil, using mock exercise catalog fallback")
		var mockReader MockExerciseCatalogReader
		return mockReader.SearchByFilter(ctx, filter)
	}

	query := r.db.WithContext(ctx).Model(&exerciseDBModel{})

	// Status filter: ignore archived exercises
	query = query.Where("status != 'ARCHIVED'")

	if filter != nil {
		targetMuscle := filter.TargetMuscleID
		if targetMuscle == "" {
			targetMuscle = filter.BodyPartID
		}

		if targetMuscle != "" {
			query = query.Where("target_muscle_id = ? OR body_part_id = ?", targetMuscle, targetMuscle)
		}

		if len(filter.EquipmentIDs) > 0 {
			query = query.Where("equipment_id IN ?", filter.EquipmentIDs)
		}

		if filter.Difficulty != "" {
			query = query.Where("difficulty ILIKE ?", filter.Difficulty)
		}

		if filter.Keyword != "" {
			pattern := "%" + filter.Keyword + "%"
			query = query.Where("name ILIKE ? OR id ILIKE ?", pattern, pattern)
		}

		if filter.Limit > 0 {
			query = query.Limit(filter.Limit)
		}
		if filter.Offset > 0 {
			query = query.Offset(filter.Offset)
		}
	}

	var records []exerciseDBModel
	err := query.Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("search exercises in DB: %w", err)
	}

	if len(records) == 0 {
		// Fallback to mock catalog if database tables are unseeded/empty
		var mockReader MockExerciseCatalogReader
		return mockReader.SearchByFilter(ctx, filter)
	}

	result := make([]port.Exercise, 0, len(records))
	for _, rec := range records {
		result = append(result, mapDBRecordToExercise(rec))
	}

	return result, nil
}

// GetByID returns an exercise by ID or port.ErrExerciseNotFound if not found.
func (r *PostgresExerciseCatalogReader) GetByID(ctx context.Context, exerciseID string) (port.Exercise, error) {
	if r.db == nil {
		var mockReader MockExerciseCatalogReader
		return mockReader.GetByID(ctx, exerciseID)
	}

	var rec exerciseDBModel
	err := r.db.WithContext(ctx).Where("id = ? AND status != 'ARCHIVED'", exerciseID).First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Fallback check in mock catalog before returning ErrExerciseNotFound
			var mockReader MockExerciseCatalogReader
			if mockEx, mockErr := mockReader.GetByID(ctx, exerciseID); mockErr == nil {
				return mockEx, nil
			}
			return port.Exercise{}, fmt.Errorf("%w: %s", port.ErrExerciseNotFound, exerciseID)
		}
		return port.Exercise{}, fmt.Errorf("get exercise by ID from DB: %w", err)
	}

	return mapDBRecordToExercise(rec), nil
}

func mapDBRecordToExercise(rec exerciseDBModel) port.Exercise {
	muscleGroup := rec.TargetMuscleID
	if muscleGroup == "" {
		muscleGroup = rec.BodyPartID
	}

	eqLower := strings.ToLower(rec.EquipmentID)
	isBodyweight := eqLower == "bodyweight" || eqLower == "none" || eqLower == "" || eqLower == "pull-up-bar" || eqLower == "body_weight"
	isMachineOrCable := strings.Contains(eqLower, "cable") || strings.Contains(eqLower, "machine") || strings.Contains(eqLower, "pulldown")

	return port.Exercise{
		ExerciseID:       rec.ID,
		Name:             rec.Name,
		MuscleGroup:      muscleGroup,
		Equipment:        rec.EquipmentID,
		Difficulty:       rec.Difficulty,
		IsBodyweight:     isBodyweight,
		IsMachineOrCable: isMachineOrCable,
	}
}
