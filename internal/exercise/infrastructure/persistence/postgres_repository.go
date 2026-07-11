// Package persistence contains storage adapters for Exercise.
package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/exercise/application"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
)

type PostgresRepository struct {
	db *sql.DB
}

var _ application.Repository = (*PostgresRepository)(nil)

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Save(
	ctx context.Context,
	exercise *domain.Exercise,
	event *domain.Event,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer rollback(tx)

	if err := saveExercise(ctx, tx, exercise.Info()); err != nil {
		return err
	}
	if err := replaceSecondaryMuscles(ctx, tx, exercise.Info()); err != nil {
		return err
	}
	if err := replaceTags(ctx, tx, exercise.Info()); err != nil {
		return err
	}
	if event != nil {
		if err := insertOutbox(ctx, tx, event); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r *PostgresRepository) FindByID(
	ctx context.Context,
	id string,
) (*domain.Exercise, error) {
	info, err := scanExercise(
		ctx,
		r.db,
		`SELECT id, name, body_part_id, equipment_id, target_muscle_id,
			instructions, thumbnail_url, media_url, video_url, difficulty,
			default_rest_seconds, status, archived_at, created_at, updated_at
		FROM exercise.exercises
		WHERE id = $1`,
		id,
	)
	if err != nil {
		return nil, err
	}

	if err := loadRelations(ctx, r.db, &info); err != nil {
		return nil, err
	}

	exercise, err := domain.RehydrateExercise(info)
	if err != nil {
		return nil, err
	}

	return exercise, nil
}

func (r *PostgresRepository) SearchActive(
	ctx context.Context,
	filters application.SearchFilters,
) ([]*domain.Exercise, error) {
	query, args := buildSearchQuery(filters)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query active exercises: %w", err)
	}
	defer rows.Close()

	var exercises []*domain.Exercise
	for rows.Next() {
		info, err := scanExerciseRow(rows)
		if err != nil {
			return nil, err
		}
		if err := loadRelations(ctx, r.db, &info); err != nil {
			return nil, err
		}

		exercise, err := domain.RehydrateExercise(info)
		if err != nil {
			return nil, err
		}
		exercises = append(exercises, exercise)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active exercises: %w", err)
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

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type exerciseScanner interface {
	Scan(dest ...any) error
}

func saveExercise(ctx context.Context, tx *sql.Tx, info domain.Info) error {
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO exercise.exercises (
			id, name, body_part_id, equipment_id, target_muscle_id,
			instructions, thumbnail_url, media_url, video_url, difficulty,
			default_rest_seconds, status, archived_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			body_part_id = EXCLUDED.body_part_id,
			equipment_id = EXCLUDED.equipment_id,
			target_muscle_id = EXCLUDED.target_muscle_id,
			instructions = EXCLUDED.instructions,
			thumbnail_url = EXCLUDED.thumbnail_url,
			media_url = EXCLUDED.media_url,
			video_url = EXCLUDED.video_url,
			difficulty = EXCLUDED.difficulty,
			default_rest_seconds = EXCLUDED.default_rest_seconds,
			status = EXCLUDED.status,
			archived_at = EXCLUDED.archived_at,
			updated_at = EXCLUDED.updated_at`,
		info.ID,
		info.Name,
		info.BodyPartID,
		info.EquipmentID,
		info.TargetMuscleID,
		info.Instructions,
		nullString(info.ThumbnailURL),
		nullString(info.MediaURL),
		nullString(info.VideoURL),
		info.Difficulty,
		info.DefaultRestSeconds,
		string(info.Status),
		info.ArchivedAt,
		info.CreatedAt,
		info.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert exercise: %w", err)
	}

	return nil
}

func replaceSecondaryMuscles(ctx context.Context, tx *sql.Tx, info domain.Info) error {
	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM exercise.exercise_secondary_muscles WHERE exercise_id = $1`,
		info.ID,
	); err != nil {
		return fmt.Errorf("delete secondary muscles: %w", err)
	}

	for _, muscleID := range info.SecondaryMuscleIDs {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO exercise.exercise_secondary_muscles (exercise_id, muscle_id)
			VALUES ($1, $2)`,
			info.ID,
			muscleID,
		); err != nil {
			return fmt.Errorf("insert secondary muscle: %w", err)
		}
	}

	return nil
}

func replaceTags(ctx context.Context, tx *sql.Tx, info domain.Info) error {
	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM exercise.exercise_tags WHERE exercise_id = $1`,
		info.ID,
	); err != nil {
		return fmt.Errorf("delete exercise tags: %w", err)
	}

	for _, tagID := range info.TagIDs {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO exercise.exercise_tags (exercise_id, tag_id)
			VALUES ($1, $2)`,
			info.ID,
			tagID,
		); err != nil {
			return fmt.Errorf("insert exercise tag: %w", err)
		}
	}

	return nil
}

func insertOutbox(ctx context.Context, tx *sql.Tx, event *domain.Event) error {
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO exercise.outbox (
			id, event_id, event_type, payload, partition_key, created_at
		) VALUES ($1, $2, $3, $4, $5, $6)`,
		event.ID,
		event.ID,
		event.Type,
		event.Payload,
		event.PartitionKey,
		event.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}

	return nil
}

func scanExercise(
	ctx context.Context,
	q queryer,
	query string,
	args ...any,
) (domain.Info, error) {
	info, err := scanExerciseScanner(q.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Info{}, domain.ErrExerciseNotFound
	}
	if err != nil {
		return domain.Info{}, err
	}

	return info, nil
}

func scanExerciseRow(rows *sql.Rows) (domain.Info, error) {
	return scanExerciseScanner(rows)
}

func scanExerciseScanner(scanner exerciseScanner) (domain.Info, error) {
	var (
		info         domain.Info
		status       string
		instructions sql.NullString
		thumbnailURL sql.NullString
		mediaURL     sql.NullString
		videoURL     sql.NullString
		difficulty   sql.NullString
		archivedAt   sql.NullTime
		createdAt    sql.NullTime
		updatedAt    sql.NullTime
	)

	err := scanner.Scan(
		&info.ID,
		&info.Name,
		&info.BodyPartID,
		&info.EquipmentID,
		&info.TargetMuscleID,
		&instructions,
		&thumbnailURL,
		&mediaURL,
		&videoURL,
		&difficulty,
		&info.DefaultRestSeconds,
		&status,
		&archivedAt,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return domain.Info{}, err
	}

	info.Instructions = instructions.String
	info.ThumbnailURL = thumbnailURL.String
	info.MediaURL = mediaURL.String
	info.VideoURL = videoURL.String
	info.Difficulty = difficulty.String
	info.Status = domain.Status(status)
	if archivedAt.Valid {
		info.ArchivedAt = &archivedAt.Time
	}
	if createdAt.Valid {
		info.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		info.UpdatedAt = updatedAt.Time
	}

	return info, nil
}

func loadRelations(ctx context.Context, q queryer, info *domain.Info) error {
	secondaryMuscleIDs, err := queryStrings(
		ctx,
		q,
		`SELECT muscle_id
		FROM exercise.exercise_secondary_muscles
		WHERE exercise_id = $1
		ORDER BY muscle_id`,
		info.ID,
	)
	if err != nil {
		return fmt.Errorf("load secondary muscles: %w", err)
	}

	tagIDs, err := queryStrings(
		ctx,
		q,
		`SELECT tag_id
		FROM exercise.exercise_tags
		WHERE exercise_id = $1
		ORDER BY tag_id`,
		info.ID,
	)
	if err != nil {
		return fmt.Errorf("load tags: %w", err)
	}

	info.SecondaryMuscleIDs = secondaryMuscleIDs
	info.TagIDs = tagIDs

	return nil
}

func buildSearchQuery(filters application.SearchFilters) (string, []any) {
	var (
		conditions = []string{"e.status = $1"}
		args       = []any{string(domain.StatusActive)}
	)

	addCondition := func(condition string, value any) {
		args = append(args, value)
		placeholder := fmt.Sprintf("$%d", len(args))
		conditions = append(conditions, fmt.Sprintf(condition, placeholder))
	}

	if filters.BodyPartID != "" {
		addCondition("e.body_part_id = %s", filters.BodyPartID)
	}
	if filters.EquipmentID != "" {
		addCondition("e.equipment_id = %s", filters.EquipmentID)
	}
	if filters.TargetMuscleID != "" {
		addCondition("e.target_muscle_id = %s", filters.TargetMuscleID)
	}
	if filters.Keyword != "" {
		addCondition("e.name ILIKE '%%' || %s || '%%'", filters.Keyword)
	}
	if filters.Difficulty != "" {
		addCondition("e.difficulty = %s", filters.Difficulty)
	}
	for _, muscleID := range filters.SecondaryMuscleIDs {
		addCondition(
			`EXISTS (
				SELECT 1 FROM exercise.exercise_secondary_muscles esm
				WHERE esm.exercise_id = e.id AND esm.muscle_id = %s
			)`,
			muscleID,
		)
	}
	for _, tagID := range filters.TagIDs {
		addCondition(
			`EXISTS (
				SELECT 1 FROM exercise.exercise_tags et
				WHERE et.exercise_id = e.id AND et.tag_id = %s
			)`,
			tagID,
		)
	}

	query := `SELECT e.id, e.name, e.body_part_id, e.equipment_id, e.target_muscle_id,
		e.instructions, e.thumbnail_url, e.media_url, e.video_url, e.difficulty,
		e.default_rest_seconds, e.status, e.archived_at, e.created_at, e.updated_at
		FROM exercise.exercises e
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY e.name`

	limit := filters.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	args = append(args, limit)
	query += fmt.Sprintf(" LIMIT $%d", len(args))

	if filters.Offset > 0 {
		args = append(args, filters.Offset)
		query += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	return query, args
}

func queryStrings(ctx context.Context, q queryer, query string, args ...any) ([]string, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return values, nil
}

func queryBodyParts(ctx context.Context, q queryer) ([]application.BodyPart, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, name FROM exercise.body_parts ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query body parts: %w", err)
	}
	defer rows.Close()

	var values []application.BodyPart
	for rows.Next() {
		var value application.BodyPart
		if err := rows.Scan(&value.ID, &value.Name); err != nil {
			return nil, fmt.Errorf("scan body part: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate body parts: %w", err)
	}

	return values, nil
}

func queryEquipments(ctx context.Context, q queryer) ([]application.Equipment, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, name FROM exercise.equipments ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query equipments: %w", err)
	}
	defer rows.Close()

	var values []application.Equipment
	for rows.Next() {
		var value application.Equipment
		if err := rows.Scan(&value.ID, &value.Name); err != nil {
			return nil, fmt.Errorf("scan equipment: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate equipments: %w", err)
	}

	return values, nil
}

func queryMuscles(ctx context.Context, q queryer) ([]application.Muscle, error) {
	rows, err := q.QueryContext(
		ctx,
		`SELECT id, name, body_part_id FROM exercise.muscles ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("query muscles: %w", err)
	}
	defer rows.Close()

	var values []application.Muscle
	for rows.Next() {
		var value application.Muscle
		if err := rows.Scan(&value.ID, &value.Name, &value.BodyPartID); err != nil {
			return nil, fmt.Errorf("scan muscle: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate muscles: %w", err)
	}

	return values, nil
}

func queryTags(ctx context.Context, q queryer) ([]application.Tag, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, name FROM exercise.tags ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer rows.Close()

	var values []application.Tag
	for rows.Next() {
		var value application.Tag
		if err := rows.Scan(&value.ID, &value.Name); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tags: %w", err)
	}

	return values, nil
}

func nullString(value string) sql.NullString {
	return sql.NullString{
		String: value,
		Valid:  value != "",
	}
}

func rollback(tx *sql.Tx) {
	_ = tx.Rollback()
}
