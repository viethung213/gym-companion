package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/viethung213/gym-companion/internal/profile/application/port"
)

type GormOutboxLogRepository struct {
	db *gorm.DB
}

var _ port.OutboxLogRepository = (*GormOutboxLogRepository)(nil)

func NewGormOutboxLogRepository(db *gorm.DB) *GormOutboxLogRepository {
	return &GormOutboxLogRepository{db: db}
}

func (r *GormOutboxLogRepository) getDB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return r.db.WithContext(ctx)
}

func (r *GormOutboxLogRepository) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	if eventID == "" {
		return false, nil
	}
	db := r.getDB(ctx)
	var logModel OutboxLogModel
	err := db.Where("event_id = ? AND status = ?", eventID, "PROCESSED").First(&logModel).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("query outbox log by event_id: %w", err)
	}
	return true, nil
}

func (r *GormOutboxLogRepository) SaveLog(ctx context.Context, logRecord *port.OutboxLogRecord) error {
	m := &OutboxLogModel{
		ID:           logRecord.ID,
		EventID:      logRecord.EventID,
		EventType:    logRecord.EventType,
		Payload:      logRecord.Payload,
		PartitionKey: logRecord.PartitionKey,
		ProcessedAt:  time.Now(),
		Status:       logRecord.Status,
		ErrorMessage: logRecord.ErrorMessage,
	}
	db := r.getDB(ctx)
	if err := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(m).Error; err != nil {
		return fmt.Errorf("save outbox log model: %w", err)
	}
	return nil
}
