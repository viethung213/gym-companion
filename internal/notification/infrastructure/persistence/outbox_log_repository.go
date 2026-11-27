package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	if status == "" {
		status = "PROCESSING"
	}

	if eventID != "" {
		var existing OutboxLogModel
		err := GetDB(ctx, r.db).
			Where("event_id = ?", eventID).
			First(&existing).Error
		if err == nil {
			// 1. Nếu trạng thái trước là FAILED -> Đây là RETRY! Cho phép tiếp tục xử lý.
			if existing.Status == "FAILED" {
				updates := map[string]interface{}{
					"status":        status,
					"processed_at":  time.Now().UTC(),
					"error_message": errMsg,
				}
				if updErr := GetDB(ctx, r.db).Model(&existing).Updates(updates).Error; updErr != nil {
					return false, fmt.Errorf("update retry status in notification outbox_log: %w", updErr)
				}
				return true, nil
			}

			// 2. Nếu đã từng SUCCESS/PROCESSED hoặc đang PROCESSING -> Duplicate, bỏ qua.
			return false, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return false, fmt.Errorf("check duplicate event in notification outbox_log: %w", err)
		}
	}

	model := &OutboxLogModel{
		ID:           uuid.New().String(),
		EventID:      eventID,
		EventType:    eventType,
		Payload:      payload,
		PartitionKey: partitionKey,
		ProcessedAt:  time.Now().UTC(),
		Status:       status,
		ErrorMessage: errMsg,
	}

	res := GetDB(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "event_id"}},
		DoNothing: true,
	}).Create(model)
	if res.Error != nil {
		return false, fmt.Errorf("insert notification outbox_log: %w", res.Error)
	}

	return res.RowsAffected > 0, nil
}

func (r *OutboxLogRepository) SaveLog(ctx context.Context, record *port.OutboxLogRecord) error {
	if record == nil {
		return nil
	}

	processedAt := record.ProcessedAt
	if processedAt.IsZero() {
		processedAt = time.Now().UTC()
	}

	if record.EventID != "" {
		var existing OutboxLogModel
		err := GetDB(ctx, r.db).
			Where("event_id = ?", record.EventID).
			First(&existing).Error
		if err == nil {
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
			if err := GetDB(ctx, r.db).Model(&existing).Updates(updates).Error; err != nil {
				return fmt.Errorf("update existing notification outbox_log: %w", err)
			}
			return nil
		}
	}

	id := record.ID
	if id == "" {
		id = uuid.New().String()
	}
	eventID := record.EventID
	if eventID == "" {
		eventID = uuid.New().String()
	}

	model := &OutboxLogModel{
		ID:           id,
		EventID:      eventID,
		EventType:    record.EventType,
		Payload:      record.Payload,
		PartitionKey: record.PartitionKey,
		ProcessedAt:  processedAt,
		Status:       record.Status,
		ErrorMessage: record.ErrorMessage,
		RetryCount:   record.RetryCount,
		NextRetryAt:  record.NextRetryAt,
	}

	err := GetDB(ctx, r.db).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "event_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"status", "error_message", "processed_at", "retry_count", "next_retry_at",
		}),
	}).Create(model).Error
	if err != nil {
		return fmt.Errorf("save outbound log in notification.outbox_log: %w", err)
	}
	return nil
}

func (r *OutboxLogRepository) FetchFailedLogs(ctx context.Context, limit int) ([]*port.OutboxLogRecord, error) {
	if limit <= 0 {
		limit = 50
	}

	var models []OutboxLogModel
	now := time.Now().UTC()
	err := GetDB(ctx, r.db).
		Where("status = ? AND retry_count < ? AND (next_retry_at IS NULL OR next_retry_at <= ?)", "FAILED", 3, now).
		Order("processed_at ASC").
		Limit(limit).
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("query failed notification outbox logs: %w", err)
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
