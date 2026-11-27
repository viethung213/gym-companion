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
	status, errMsg string,
) (bool, error) {
	if status == "" {
		status = "PROCESSING"
	}

	if eventID != "" {
		now := time.Now().UTC()
		zombieThreshold := now.Add(-defaultProcessingTimeout)

		// 1. Atomic Compare-And-Swap: Cho phép claim lock nếu:
		//   a) Trạng thái hiện tại là FAILED (retry sau lỗi)
		//   b) Trạng thái hiện tại là PROCESSING nhưng đã quá thời hạn timeout (Zombie lock do server crash)
		// Chỉ duy nhất 1 worker có RowsAffected > 0 để claim quyền xử lý, loại bỏ hoàn toàn race condition.
		res := GetDB(ctx, r.db).Model(&OutboxLogModel{}).
			Where("event_id = ? AND (status = ? OR (status = ? AND processed_at < ?))",
				eventID, "FAILED", "PROCESSING", zombieThreshold).
			Updates(map[string]interface{}{
				"status":        status,
				"processed_at":  now,
				"error_message": errMsg,
			})
		if res.Error != nil {
			return false, fmt.Errorf("atomic update retry or zombie status in notification outbox_log: %w", res.Error)
		}
		if res.RowsAffected > 0 {
			return true, nil
		}

		// 2. Nếu không cập nhật được (RowsAffected == 0), kiểm tra xem bản ghi đã tồn tại chưa
		var existing OutboxLogModel
		err := GetDB(ctx, r.db).
			Where("event_id = ?", eventID).
			First(&existing).Error
		if err == nil {
			// Đã tồn tại với trạng thái SUCCESS/PROCESSED hoặc đang PROCESSING còn trong hạn timeout (< 5 phút)
			// -> Duplicate, bỏ qua an toàn.
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
