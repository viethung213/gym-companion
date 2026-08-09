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

// exerciseFullRow represents joined DB columns for exercise catalog query.
type exerciseFullRow struct {
	ID               string `gorm:"column:id"`
	Name             string `gorm:"column:name"`
	BodyPartID       string `gorm:"column:body_part_id"`
	BodyPartName     string `gorm:"column:body_part_name"`
	EquipmentID      string `gorm:"column:equipment_id"`
	EquipmentName    string `gorm:"column:equipment_name"`
	TargetMuscleID   string `gorm:"column:target_muscle_id"`
	TargetMuscleName string `gorm:"column:target_muscle_name"`
	Difficulty       string `gorm:"column:difficulty"`
	Status           string `gorm:"column:status"`
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

	query := r.db.WithContext(ctx).Table("exercise.exercises e").
		Select(`e.id, e.name, e.body_part_id, bp.name AS body_part_name,
		        e.equipment_id, eq.name AS equipment_name,
		        e.target_muscle_id, m.name AS target_muscle_name,
		        e.difficulty, e.status`).
		Joins("LEFT JOIN exercise.muscles m ON m.id = e.target_muscle_id").
		Joins("LEFT JOIN exercise.body_parts bp ON bp.id = e.body_part_id").
		Joins("LEFT JOIN exercise.equipments eq ON eq.id = e.equipment_id").
		Where("e.status != 'ARCHIVED'")

	if filter != nil {
		targetMuscle := strings.TrimSpace(filter.TargetMuscleID)
		if targetMuscle == "" {
			targetMuscle = strings.TrimSpace(filter.BodyPartID)
		}

		if targetMuscle != "" {
			pattern := "%" + targetMuscle + "%"
			query = query.Where(`(
				e.target_muscle_id = ? OR e.body_part_id = ?
				OR LOWER(m.name) LIKE LOWER(?) OR LOWER(bp.name) LIKE LOWER(?) OR m.id = ? OR bp.id = ?
				OR EXISTS (
					SELECT 1 FROM exercise.exercise_secondary_muscles esm
					JOIN exercise.muscles sm ON sm.id = esm.muscle_id
					WHERE esm.exercise_id = e.id AND (LOWER(sm.name) LIKE LOWER(?) OR sm.id = ?)
				)
			)`, targetMuscle, targetMuscle, pattern, pattern, targetMuscle, targetMuscle, pattern, targetMuscle)
		}

		if len(filter.SecondaryMuscleIDs) > 0 {
			var secConds []string
			var secArgs []interface{}
			for _, sec := range filter.SecondaryMuscleIDs {
				secTrim := strings.TrimSpace(sec)
				if secTrim != "" {
					secPattern := "%" + secTrim + "%"
					secConds = append(secConds, "(LOWER(sm.name) LIKE LOWER(?) OR sm.id = ?)")
					secArgs = append(secArgs, secPattern, secTrim)
				}
			}
			if len(secConds) > 0 {
				query = query.Where(fmt.Sprintf(`EXISTS (
					SELECT 1 FROM exercise.exercise_secondary_muscles esm
					JOIN exercise.muscles sm ON sm.id = esm.muscle_id
					WHERE esm.exercise_id = e.id AND (%s)
				)`, strings.Join(secConds, " OR ")), secArgs...)
			}
		}

		if len(filter.EquipmentIDs) > 0 {
			var eqConds []string
			var eqArgs []interface{}
			for _, eqVal := range filter.EquipmentIDs {
				eqTrim := strings.TrimSpace(eqVal)
				if eqTrim != "" {
					eqPattern := "%" + eqTrim + "%"
					eqConds = append(eqConds, "e.equipment_id = ? OR LOWER(eq.name) LIKE LOWER(?) OR eq.id = ?")
					eqArgs = append(eqArgs, eqTrim, eqPattern, eqTrim)
				}
			}
			if len(eqConds) > 0 {
				query = query.Where(fmt.Sprintf("(%s)", strings.Join(eqConds, " OR ")), eqArgs...)
			}
		}

		if len(filter.AvoidInjuryAreas) > 0 {
			for _, avoid := range filter.AvoidInjuryAreas {
				avoidTrim := strings.TrimSpace(avoid)
				if avoidTrim != "" {
					avoidPattern := "%" + avoidTrim + "%"
					query = query.Where(`NOT (
						LOWER(m.name) LIKE LOWER(?) OR LOWER(bp.name) LIKE LOWER(?)
						OR EXISTS (
							SELECT 1 FROM exercise.exercise_secondary_muscles esm
							JOIN exercise.muscles sm ON sm.id = esm.muscle_id
							WHERE esm.exercise_id = e.id AND LOWER(sm.name) LIKE LOWER(?)
						)
					)`, avoidPattern, avoidPattern, avoidPattern)
				}
			}
		}

		if len(filter.TagIDs) > 0 {
			var tagConds []string
			var tagArgs []interface{}
			for _, tagVal := range filter.TagIDs {
				tagTrim := strings.TrimSpace(tagVal)
				if tagTrim != "" {
					tagPattern := "%" + tagTrim + "%"
					tagConds = append(tagConds, "(LOWER(t.name) LIKE LOWER(?) OR t.id = ?)")
					tagArgs = append(tagArgs, tagPattern, tagTrim)
				}
			}
			if len(tagConds) > 0 {
				query = query.Where(fmt.Sprintf(`EXISTS (
					SELECT 1 FROM exercise.exercise_tags et
					JOIN exercise.tags t ON t.id = et.tag_id
					WHERE et.exercise_id = e.id AND (%s)
				)`, strings.Join(tagConds, " OR ")), tagArgs...)
			}
		}

		if filter.Difficulty != "" {
			query = query.Where("LOWER(e.difficulty) LIKE LOWER(?)", filter.Difficulty)
		}

		if filter.Keyword != "" {
			pattern := "%" + strings.TrimSpace(filter.Keyword) + "%"
			query = query.Where("LOWER(e.name) LIKE LOWER(?) OR LOWER(e.id) LIKE LOWER(?) OR LOWER(m.name) LIKE LOWER(?) OR LOWER(bp.name) LIKE LOWER(?)", pattern, pattern, pattern, pattern)
		}

		if filter.Limit > 0 {
			query = query.Limit(filter.Limit)
		}
		if filter.Offset > 0 {
			query = query.Offset(filter.Offset)
		}
	}

	var rows []exerciseFullRow
	err := query.Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("search exercises in DB: %w", err)
	}

	result := make([]port.Exercise, 0, len(rows))
	for i := range rows {
		result = append(result, mapFullRowToExercise(&rows[i]))
	}

	return result, nil
}

// GetByID returns an exercise by ID or port.ErrExerciseNotFound if not found.
func (r *PostgresExerciseCatalogReader) GetByID(ctx context.Context, exerciseID string) (port.Exercise, error) {
	if r.db == nil {
		var mockReader MockExerciseCatalogReader
		return mockReader.GetByID(ctx, exerciseID)
	}

	var row exerciseFullRow
	err := r.db.WithContext(ctx).Table("exercise.exercises e").
		Select(`e.id, e.name, e.body_part_id, bp.name AS body_part_name,
		        e.equipment_id, eq.name AS equipment_name,
		        e.target_muscle_id, m.name AS target_muscle_name,
		        e.difficulty, e.status`).
		Joins("LEFT JOIN exercise.muscles m ON m.id = e.target_muscle_id").
		Joins("LEFT JOIN exercise.body_parts bp ON bp.id = e.body_part_id").
		Joins("LEFT JOIN exercise.equipments eq ON eq.id = e.equipment_id").
		Where("e.id = ? AND e.status != 'ARCHIVED'", exerciseID).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return port.Exercise{}, fmt.Errorf("%w: %s", port.ErrExerciseNotFound, exerciseID)
		}
		return port.Exercise{}, fmt.Errorf("get exercise by ID from DB: %w", err)
	}

	return mapFullRowToExercise(&row), nil
}

func mapFullRowToExercise(row *exerciseFullRow) port.Exercise {
	muscleGroup := row.TargetMuscleName
	if muscleGroup == "" {
		muscleGroup = row.BodyPartName
	}
	if muscleGroup == "" {
		muscleGroup = row.TargetMuscleID
	}
	if muscleGroup == "" {
		muscleGroup = row.BodyPartID
	}

	eqName := row.EquipmentName
	if eqName == "" {
		eqName = row.EquipmentID
	}

	eqLower := strings.ToLower(eqName)
	isBodyweight := eqLower == "bodyweight" || eqLower == "body weight" || eqLower == "none" || eqLower == "" || eqLower == "pull-up-bar" || eqLower == "body_weight"
	isMachineOrCable := strings.Contains(eqLower, "cable") || strings.Contains(eqLower, "machine") || strings.Contains(eqLower, "pulldown") || strings.Contains(eqLower, "lever")

	return port.Exercise{
		ExerciseID:       row.ID,
		Name:             row.Name,
		MuscleGroup:      muscleGroup,
		Equipment:        eqName,
		Difficulty:       row.Difficulty,
		IsBodyweight:     isBodyweight,
		IsMachineOrCable: isMachineOrCable,
	}
}
