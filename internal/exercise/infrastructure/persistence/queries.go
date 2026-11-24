package persistence

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"gorm.io/gorm"
)

const (
	defaultSearchLimit = 50
	maxSearchLimit     = 2000
)

func (r *PostgresRepository) buildSearchQuery(
	ctx context.Context,
	filters *port.SearchFilters,
) *gorm.DB {
	query := r.db.WithContext(ctx).Table("exercise.exercises AS e")

	if !filters.IncludeAllStatuses {
		if filters.Status != "" {
			query = query.Where("e.status = ?", string(filters.Status))
		} else {
			query = query.Where("e.status = ?", string(domain.StatusActive))
		}
	}

	if len(filters.PrimaryMuscles) > 0 {
		query = query.Where("e.primary_muscle IN ?", codes(filters.PrimaryMuscles))
	}
	if len(filters.SecondaryMuscles) > 0 {
		query = query.Where("e.secondary_muscles && ?::text[]", toTextArray(filters.SecondaryMuscles))
	}
	if len(filters.Equipment) > 0 {
		query = query.Where("e.equipment IN ?", codes(filters.Equipment))
	}
	if len(filters.Categories) > 0 {
		query = query.Where("e.category IN ?", codes(filters.Categories))
	}
	if len(filters.Levels) > 0 {
		query = query.Where("e.level IN ?", codes(filters.Levels))
	}
	if filters.Force != "" {
		query = query.Where("e.force = ?", string(filters.Force))
	}
	if filters.Mechanic != "" {
		query = query.Where("e.mechanic = ?", string(filters.Mechanic))
	}
	if len(filters.ExcludeMuscles) > 0 {
		excluded := toTextArray(filters.ExcludeMuscles)
		query = query.Where(
			"e.primary_muscle <> ALL(?::text[]) AND NOT (e.secondary_muscles && ?::text[])",
			excluded, excluded,
		)
	}
	if filters.HasAISupported != nil {
		query = query.Where("e.has_ai_supported = ?", *filters.HasAISupported)
	}
	if filters.Keyword != "" {
		query = query.Where("e.name ILIKE '%' || ? || '%'", filters.Keyword)
	}
	if filters.MeasurementType != "" {
		query = query.Where("e.measurement_type = ?", filters.MeasurementType)
	}
	if filters.HasMotionSpec != nil {
		query = query.Where(specExistsClause(*filters.HasMotionSpec, "local_rules_url"))
	}
	if filters.HasVoiceSpec != nil {
		query = query.Where(specExistsClause(*filters.HasVoiceSpec, "dialogue_engine_url"))
	}

	return query
}

// specExistsClause filters on a non-blank motion specification URL column.
func specExistsClause(exists bool, column string) string {
	operator := "EXISTS"
	if !exists {
		operator = "NOT EXISTS"
	}

	return operator + ` (
		SELECT 1 FROM workout_execution.motion_specifications ms
		WHERE ms.exercise_id = e.id AND COALESCE(BTRIM(ms.` + column + `), '') <> ''
	)`
}

func (r *PostgresRepository) searchActiveRecords(
	ctx context.Context,
	filters *port.SearchFilters,
) ([]exerciseRecord, int32, error) {
	countQuery := r.buildSearchQuery(ctx, filters)
	var totalCount int64
	if err := countQuery.Count(&totalCount).Error; err != nil {
		return nil, 0, fmt.Errorf("count active exercises: %w", err)
	}

	limit := filters.Limit
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}

	dataQuery := r.buildSearchQuery(ctx, filters).
		Select("e.*").
		Order("e.name").
		Limit(int(limit))
	if filters.Offset > 0 {
		dataQuery = dataQuery.Offset(int(filters.Offset))
	}

	var records []exerciseRecord
	if err := dataQuery.Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("query active exercises: %w", err)
	}

	return records, int32(totalCount), nil
}

func codes[T ~string](values []T) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = string(v)
	}

	return out
}
