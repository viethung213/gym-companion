package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PostgresOutboxLogRepository implements port.OutboxLogRepository for consumer idempotency tracking.
type PostgresOutboxLogRepository struct {
	db *gorm.DB
}

var _ port.OutboxLogRepository = (*PostgresOutboxLogRepository)(nil)

// NewPostgresOutboxLogRepository constructs repository.
func NewPostgresOutboxLogRepository(db *gorm.DB) *PostgresOutboxLogRepository {
	return &PostgresOutboxLogRepository{db: db}
}

func (r *PostgresOutboxLogRepository) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	if eventID == "" {
		return false, nil
	}
	db := GetDB(ctx, r.db)
	var count int64
	err := db.Model(&OutboxLogModel{}).
		Where("event_id = ? AND status = ?", eventID, "PROCESSED").
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check outbox_log idempotency: %w", err)
	}
	return count > 0, nil
}

func (r *PostgresOutboxLogRepository) SaveLog(ctx context.Context, record *port.OutboxLogRecord) error {
	if record == nil {
		return nil
	}
	db := GetDB(ctx, r.db)

	id := record.ID
	if id == "" {
		id = record.EventID
	}
	if id == "" {
		id = uuid.New().String()
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

	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(model).Error; err != nil {
		return fmt.Errorf("failed to save outbox_log record: %w", err)
	}
	return nil
}
