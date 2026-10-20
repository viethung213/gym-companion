package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GORMOutboxLogRepository struct {
	db *gorm.DB
}

var _ port.OutboxLogRepository = (*GORMOutboxLogRepository)(nil)

func NewGORMOutboxLogRepository(db *gorm.DB) *GORMOutboxLogRepository {
	return &GORMOutboxLogRepository{db: db}
}

// Save ghi nhận kết quả xử lý của consumer vào solo_match.outbox_log
func (r *GORMOutboxLogRepository) Save(
	ctx context.Context,
	eventID, eventType string,
	payload []byte,
	partitionKey, status, errMsg string,
) error {
	if status == "" {
		status = "SUCCESS"
	}

	m := OutboxLogModel{
		ID:           uuid.New().String(),
		EventID:      eventID,
		EventType:    eventType,
		Payload:      payload,
		PartitionKey: partitionKey,
		ProcessedAt:  time.Now().UTC(),
		Status:       status,
		ErrorMessage: errMsg,
	}

	// Idempotent: Bỏ qua nếu event_id này đã được log với status trùng khớp
	err := GetDB(ctx, r.db).Clauses(clause.OnConflict{DoNothing: true}).Create(&m).Error
	if err != nil {
		return fmt.Errorf("create outbox log: %w", err)
	}
	return nil
}

// IsProcessed kiểm tra xem eventID này đã được xử lý thành công chưa (Idempotency check)
func (r *GORMOutboxLogRepository) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	if eventID == "" {
		return false, nil
	}

	var count int64
	err := GetDB(ctx, r.db).Model(&OutboxLogModel{}).
		Where("event_id = ? AND status IN ('PROCESSED', 'SUCCESS')", eventID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check event processed: %w", err)
	}
	return count > 0, nil
}
