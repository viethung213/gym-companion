package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GormOutboxLogRepository implements port.OutboxLogRepository using GORM.
type GormOutboxLogRepository struct {
	db *gorm.DB
}

// Compile-time interface check.
var _ port.OutboxLogRepository = (*GormOutboxLogRepository)(nil)

// NewGormOutboxLogRepository creates a new GormOutboxLogRepository.
func NewGormOutboxLogRepository(db *gorm.DB) *GormOutboxLogRepository {
	return &GormOutboxLogRepository{db: db}
}

// IsProcessed checks whether an event has already been successfully processed.
func (r *GormOutboxLogRepository) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	if eventID == "" {
		return false, nil
	}

	db := GetDB(ctx, r.db)
	var logModel OutboxLogModel
	err := db.Where("event_id = ? AND status = ?", eventID, "PROCESSED").First(&logModel).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("check outbox log processed: %w", err)
	}

	return true, nil
}

// SaveLog stores or updates an event consumption log entry.
func (r *GormOutboxLogRepository) SaveLog(ctx context.Context, record *port.OutboxLogRecord) error {
	if record == nil {
		return errors.New("nil outbox log record")
	}

	id := record.ID
	if id == "" {
		id = uuid.NewString()
	}

	model := &OutboxLogModel{
		ID:           id,
		EventID:      record.EventID,
		EventType:    record.EventType,
		Payload:      record.Payload,
		PartitionKey: record.PartitionKey,
		ProcessedAt:  time.Now().UTC(),
		Status:       record.Status,
		ErrorMessage: record.ErrorMessage,
	}

	db := GetDB(ctx, r.db)
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "event_id"}},
		UpdateAll: true,
	}).Create(model).Error; err != nil {
		return fmt.Errorf("save outbox log: %w", err)
	}

	return nil
}
