package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/application/port"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PostgresOutboxLogRepository struct {
	db *gorm.DB
}

var _ port.OutboxLogRepository = (*PostgresOutboxLogRepository)(nil)

func NewPostgresOutboxLogRepository(db *gorm.DB) *PostgresOutboxLogRepository {
	return &PostgresOutboxLogRepository{db: db}
}

func (r *PostgresOutboxLogRepository) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	if eventID == "" {
		return false, nil
	}
	db := getDB(ctx, r.db)
	var logModel GormOutboxLog
	err := db.Where("event_id = ? AND status = ?", eventID, "PROCESSED").First(&logModel).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("query outbox log by event_id: %w", err)
	}
	return true, nil
}

func (r *PostgresOutboxLogRepository) SaveLog(ctx context.Context, logRecord *port.OutboxLogRecord) error {
	m := &GormOutboxLog{
		ID:           logRecord.ID,
		EventID:      logRecord.EventID,
		EventType:    logRecord.EventType,
		Payload:      logRecord.Payload,
		PartitionKey: logRecord.PartitionKey,
		ProcessedAt:  time.Now(),
		Status:       logRecord.Status,
		ErrorMessage: logRecord.ErrorMessage,
	}
	db := getDB(ctx, r.db)
	if err := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(m).Error; err != nil {
		return fmt.Errorf("save outbox log model: %w", err)
	}
	return nil
}
