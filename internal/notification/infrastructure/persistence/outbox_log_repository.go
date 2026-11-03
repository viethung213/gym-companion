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
	if status == "" {
		status = "SUCCESS"
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

	res := GetDB(ctx, r.db).Create(model)
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

	processedAt := record.ProcessedAt
	if processedAt.IsZero() {
		processedAt = time.Now().UTC()
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
	}

	err := GetDB(ctx, r.db).Create(model).Error
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
	err := GetDB(ctx, r.db).
		Where("status = ?", "FAILED").
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
