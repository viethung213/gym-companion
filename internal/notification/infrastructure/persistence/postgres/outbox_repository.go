package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
)

var _ port.OutboxRepository = (*OutboxRepository)(nil)

type OutboxRepository struct {
	db *sql.DB
}

func NewOutboxRepository(db *sql.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) Save(ctx context.Context, record *port.OutboxRecord) error {
	if record == nil {
		return errors.New("nil outbox record")
	}

	id := record.ID
	if id == "" {
		id = uuid.New().String()
	}
	eventID := record.EventID
	if eventID == "" {
		eventID = uuid.New().String()
	}

	query := `
		INSERT INTO notification.outbox (
			id, event_id, event_type, payload, partition_key, created_at, published, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (event_id) DO NOTHING
	`
	status := "PENDING"
	if record.Published {
		status = "PUBLISHED"
	}

	executor := r.getExecutor(ctx)
	_, err := executor.ExecContext(
		ctx,
		query,
		id,
		eventID,
		record.EventType,
		record.Payload,
		record.PartitionKey,
		record.CreatedAt,
		record.Published,
		status,
	)
	if err != nil {
		return fmt.Errorf("insert notification outbox: %w", err)
	}
	return nil
}

func (r *OutboxRepository) getExecutor(ctx context.Context) DBExecutor {
	if tx := GetTx(ctx); tx != nil {
		return tx
	}
	return r.db
}

func (r *OutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]*port.OutboxRecord, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, event_id, event_type, payload, partition_key, created_at, published, published_at
		FROM notification.outbox
		WHERE published = FALSE
		ORDER BY created_at ASC
		LIMIT $1
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch unpublished notification outbox: %w", err)
	}
	defer rows.Close()

	var records []*port.OutboxRecord
	for rows.Next() {
		var rec port.OutboxRecord
		var pubAt sql.NullTime
		if err := rows.Scan(
			&rec.ID,
			&rec.EventID,
			&rec.EventType,
			&rec.Payload,
			&rec.PartitionKey,
			&rec.CreatedAt,
			&rec.Published,
			&pubAt,
		); err != nil {
			return nil, fmt.Errorf("scan notification outbox row: %w", err)
		}
		if pubAt.Valid {
			rec.PublishedAt = &pubAt.Time
		}
		records = append(records, &rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notification outbox rows: %w", err)
	}

	return records, nil
}

func (r *OutboxRepository) ClaimBatch(ctx context.Context, limit int, lockDuration time.Duration) ([]*port.OutboxRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if lockDuration <= 0 {
		lockDuration = 30 * time.Second
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin claim batch tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()
	lockedUntil := now.Add(lockDuration)

	query := `
		SELECT id, event_id, event_type, payload, partition_key, created_at, published, published_at
		FROM notification.outbox
		WHERE published = FALSE AND (status = 'PENDING' OR locked_until IS NULL OR locked_until < $1)
		ORDER BY created_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`
	rows, err := tx.QueryContext(ctx, query, now, limit)
	if err != nil {
		// Fallback without FOR UPDATE SKIP LOCKED for SQLite unit tests
		fallbackQuery := `
			SELECT id, event_id, event_type, payload, partition_key, created_at, published, published_at
			FROM notification.outbox
			WHERE published = FALSE AND (status = 'PENDING' OR locked_until IS NULL OR locked_until < $1)
			ORDER BY created_at ASC
			LIMIT $2
		`
		rows, err = tx.QueryContext(ctx, fallbackQuery, now, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("select outbox for update: %w", err)
	}

	var records []*port.OutboxRecord
	var ids []string
	for rows.Next() {
		var rec port.OutboxRecord
		var pubAt sql.NullTime
		if err := rows.Scan(
			&rec.ID,
			&rec.EventID,
			&rec.EventType,
			&rec.Payload,
			&rec.PartitionKey,
			&rec.CreatedAt,
			&rec.Published,
			&pubAt,
		); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan outbox claim row: %w", err)
		}
		if pubAt.Valid {
			rec.PublishedAt = &pubAt.Time
		}
		records = append(records, &rec)
		ids = append(ids, rec.ID)
	}
	rows.Close()

	if len(ids) > 0 {
		placeholders := make([]string, len(ids))
		args := make([]interface{}, len(ids)+1)
		args[0] = lockedUntil
		for i, id := range ids {
			placeholders[i] = fmt.Sprintf("$%d", i+2)
			args[i+1] = id
		}
		updateQuery := fmt.Sprintf(`
			UPDATE notification.outbox
			SET status = 'PROCESSING', locked_until = $1
			WHERE id IN (%s)
		`, strings.Join(placeholders, ","))
		if _, err := tx.ExecContext(ctx, updateQuery, args...); err != nil {
			return nil, fmt.Errorf("update claimed outbox status: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit claim batch tx: %w", err)
	}

	return records, nil
}

func (r *OutboxRepository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids)+1)
	args[0] = time.Now().UTC()
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args[i+1] = id
	}
	query := fmt.Sprintf(`
		UPDATE notification.outbox
		SET published = TRUE, published_at = $1, status = 'PUBLISHED'
		WHERE id IN (%s)
	`, strings.Join(placeholders, ","))

	executor := r.getExecutor(ctx)
	if _, err := executor.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("mark notification outbox published: %w", err)
	}
	return nil
}

func (r *OutboxRepository) ProcessBatch(
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
		return fmt.Errorf("publish notification outbox batch: %w", pubErr)
	}

	ids := make([]string, len(records))
	for i, rec := range records {
		ids[i] = rec.ID
	}

	return r.MarkPublished(ctx, ids)
}
