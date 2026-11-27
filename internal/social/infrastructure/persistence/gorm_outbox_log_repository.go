package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/social/application/port"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GormOutboxLogRepository struct {
	db *gorm.DB
}

var _ port.OutboxLogRepository = (*GormOutboxLogRepository)(nil)

func NewGormOutboxLogRepository(db *gorm.DB) *GormOutboxLogRepository {
	return &GormOutboxLogRepository{db: db}
}

func (r *GormOutboxLogRepository) Save(
	ctx context.Context,
	eventID, eventType string,
	payload []byte,
	partitionKey, status string,
	errMsg string,
) error {
	if status == "" {
		status = "SUCCESS"
	}

	m := &OutboxLogModel{
		ID:           uuid.New().String(),
		EventID:      eventID,
		EventType:    eventType,
		Payload:      payload,
		PartitionKey: partitionKey,
		ProcessedAt:  time.Now().UTC(),
		Status:       status,
		ErrorMessage: errMsg,
	}

	// Idempotent: cập nhật status, error_message và processed_at khi conflict event_id (hỗ trợ retry)
	err := GetDB(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "event_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"status", "error_message", "processed_at"}),
	}).Create(m).Error
	if err != nil {
		return fmt.Errorf("create outbox log: %w", err)
	}
	return nil
}

func (r *GormOutboxLogRepository) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	var count int64
	err := GetDB(ctx, r.db).Model(&OutboxLogModel{}).
		Where("event_id = ? AND status IN ('PROCESSED', 'SUCCESS')", eventID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check event processed: %w", err)
	}
	return count > 0, nil
}
