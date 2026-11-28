package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ port.OutboxLogRepository = (*OutboxLogRepository)(nil)

const defaultProcessingTimeout = 3 * time.Minute

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
) (*port.OutboxLogRecord, error) {
	if eventID == "" {
		return &port.OutboxLogRecord{
			EventID:      eventID,
			EventType:    eventType,
			PartitionKey: partitionKey,
			Payload:      payload,
			Status:       "PROCESSING",
			ProcessedAt:  time.Now().UTC(),
		}, nil
	}

	now := time.Now().UTC()
	zombieThreshold := now.Add(-defaultProcessingTimeout)

	model := &OutboxLogModel{
		ID:           uuid.New().String(),
		EventID:      eventID,
		EventType:    eventType,
		Payload:      payload,
		PartitionKey: partitionKey,
		ProcessedAt:  now,
		Status:       "PROCESSING",
		ErrorMessage: "",
	}

	res := GetDB(ctx, r.db).Clauses(clause.Returning{}, clause.OnConflict{
		Columns: []clause.Column{{Name: "event_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"status":        "PROCESSING",
			"processed_at":  now,
			"error_message": "",
		}),
		Where: clause.Where{
			Exprs: []clause.Expression{
				clause.Expr{
					SQL:  "outbox_log.status = ? OR (outbox_log.status = ? AND outbox_log.processed_at < ?)",
					Vars: []interface{}{"FAILED", "PROCESSING", zombieThreshold},
				},
			},
		},
	}).Create(model)
	if res.Error != nil {
		return nil, fmt.Errorf("upsert notification outbox_log claim lock: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, nil
	}

	return &port.OutboxLogRecord{
		ID:           model.ID,
		EventID:      model.EventID,
		EventType:    model.EventType,
		Payload:      model.Payload,
		PartitionKey: model.PartitionKey,
		ProcessedAt:  model.ProcessedAt,
		Status:       model.Status,
		ErrorMessage: model.ErrorMessage,
		RetryCount:   model.RetryCount,
		NextRetryAt:  model.NextRetryAt,
		Metadata:     model.Metadata,
	}, nil
}

func (r *OutboxLogRepository) SaveLog(ctx context.Context, record *port.OutboxLogRecord) error {
	if record == nil || record.EventID == "" {
		return nil
	}

	processedAt := record.ProcessedAt
	if processedAt.IsZero() {
		processedAt = time.Now().UTC()
	}

	updates := map[string]interface{}{
		"status":        record.Status,
		"error_message": record.ErrorMessage,
		"processed_at":  processedAt,
	}
	if record.RetryCount > 0 {
		updates["retry_count"] = record.RetryCount
	}
	if !record.NextRetryAt.IsZero() {
		updates["next_retry_at"] = record.NextRetryAt
	}
	if len(record.Metadata) > 0 {
		updates["metadata"] = record.Metadata
	}

	if err := GetDB(ctx, r.db).
		Model(&OutboxLogModel{}).
		Where("event_id = ?", record.EventID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("update notification outbox_log: %w", err)
	}

	return nil
}

func (r *OutboxLogRepository) FetchFailedLogs(ctx context.Context, limit int) ([]*port.OutboxLogRecord, error) {
	if limit <= 0 {
		limit = 50
	}

	var models []OutboxLogModel
	now := time.Now().UTC()
	zombieThreshold := now.Add(-defaultProcessingTimeout)

	err := GetDB(ctx, r.db).
		Where(
			"(status = ? AND retry_count < ? AND (next_retry_at IS NULL OR next_retry_at <= ?)) OR (status = ? AND retry_count < ? AND processed_at < ?)",
			"FAILED", 3, now, "PROCESSING", 3, zombieThreshold,
		).
		Order("processed_at ASC").
		Limit(limit).
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("query failed or zombie notification outbox logs: %w", err)
	}

	records := make([]*port.OutboxLogRecord, 0, len(models))
	for i := range models {
		records = append(records, &port.OutboxLogRecord{
			ID:           models[i].ID,
			EventID:      models[i].EventID,
			EventType:    models[i].EventType,
			Payload:      models[i].Payload,
			PartitionKey: models[i].PartitionKey,
			ProcessedAt:  models[i].ProcessedAt,
			Status:       models[i].Status,
			ErrorMessage: models[i].ErrorMessage,
			RetryCount:   models[i].RetryCount,
			NextRetryAt:  models[i].NextRetryAt,
			Metadata:     models[i].Metadata,
		})
	}

	return records, nil
}

func (r *OutboxLogRepository) UpdateLogStatus(ctx context.Context, id, status, errMsg string) error {
	err := GetDB(ctx, r.db).Model(&OutboxLogModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        status,
			"error_message": errMsg,
			"processed_at":  time.Now().UTC(),
		}).Error
	if err != nil {
		return fmt.Errorf("update notification outbox_log status: %w", err)
	}
	return nil
}

func (r *OutboxLogRepository) UpdateRetryLog(ctx context.Context, id string, retryCount int, nextRetryAt time.Time, status, errMsg string) error {
	err := GetDB(ctx, r.db).Model(&OutboxLogModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"retry_count":   retryCount,
			"next_retry_at": nextRetryAt,
			"status":        status,
			"error_message": errMsg,
			"processed_at":  time.Now().UTC(),
		}).Error
	if err != nil {
		return fmt.Errorf("update notification outbox_log retry status: %w", err)
	}
	return nil
}
