package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"gorm.io/gorm"
)

var _ port.OutboxLogRepository = (*OutboxLogRepository)(nil)

type OutboxLogRepository struct {
	db *gorm.DB
}

func NewOutboxLogRepository(db *gorm.DB) *OutboxLogRepository {
	return &OutboxLogRepository{db: db}
}

func (r *OutboxLogRepository) LogProcessed(
	ctx context.Context,
	eventID, eventType, partitionKey string,
	payload []byte,
	status, errMsg string,
) (bool, error) {
	parsedEventUUID, err := uuid.Parse(eventID)
	if err != nil {
		parsedEventUUID = uuid.New()
	}

	id := uuid.New()
	query := `
		INSERT INTO notification.outbox_log (
			id, event_id, event_type, payload, partition_key, processed_at, status, error_message
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	if status == "" {
		status = "SUCCESS"
	}

	res := GetDB(ctx, r.db).Exec(
		query,
		id,
		parsedEventUUID,
		eventType,
		payload,
		partitionKey,
		time.Now().UTC(),
		status,
		errMsg,
	)
	if res.Error != nil {
		return false, fmt.Errorf("insert notification outbox_log: %w", res.Error)
	}

	return res.RowsAffected > 0, nil
}

func (r *OutboxLogRepository) SaveLog(ctx context.Context, record *port.OutboxLogRecord) error {
	if record == nil {
		return nil
	}

	id := record.ID
	if id == "" {
		id = uuid.New().String()
	}
	eventID := record.EventID
	if eventID == "" {
		eventID = uuid.New().String()
	}

	parsedEventUUID, err := uuid.Parse(eventID)
	if err != nil {
		parsedEventUUID = uuid.New()
	}

	query := `
		INSERT INTO notification.outbox_log (
			id, event_id, event_type, payload, partition_key, processed_at, status, error_message
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	processedAt := record.ProcessedAt
	if processedAt.IsZero() {
		processedAt = time.Now().UTC()
	}

	err = GetDB(ctx, r.db).Exec(
		query,
		id,
		parsedEventUUID,
		record.EventType,
		record.Payload,
		record.PartitionKey,
		processedAt,
		record.Status,
		record.ErrorMessage,
	).Error
	if err != nil {
		return fmt.Errorf("save outbound log in notification.outbox_log: %w", err)
	}
	return nil
}

func (r *OutboxLogRepository) FetchFailedLogs(ctx context.Context, limit int) ([]*port.OutboxLogRecord, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, event_id, event_type, payload, partition_key, processed_at, status, COALESCE(error_message, '')
		FROM notification.outbox_log
		WHERE status = 'FAILED'
		ORDER BY processed_at ASC
		LIMIT $1
	`
	rows, err := GetDB(ctx, r.db).Raw(query, limit).Rows()
	if err != nil {
		return nil, fmt.Errorf("query failed notification outbox logs: %w", err)
	}
	defer rows.Close()

	var records []*port.OutboxLogRecord
	for rows.Next() {
		var rec port.OutboxLogRecord
		if err := rows.Scan(
			&rec.ID,
			&rec.EventID,
			&rec.EventType,
			&rec.Payload,
			&rec.PartitionKey,
			&rec.ProcessedAt,
			&rec.Status,
			&rec.ErrorMessage,
		); err != nil {
			return nil, fmt.Errorf("scan failed outbox log row: %w", err)
		}
		records = append(records, &rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate failed outbox log rows: %w", err)
	}

	return records, nil
}

func (r *OutboxLogRepository) UpdateLogStatus(ctx context.Context, id, status, errMsg string) error {
	query := `
		UPDATE notification.outbox_log
		SET status = $1, error_message = $2, processed_at = $3
		WHERE id = $4
	`
	err := GetDB(ctx, r.db).Exec(query, status, errMsg, time.Now().UTC(), id).Error
	if err != nil {
		return fmt.Errorf("update notification outbox_log status: %w", err)
	}
	return nil
}
