package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"gorm.io/gorm"
)

type OutboxRepository struct {
	db *gorm.DB
}

var _ port.OutboxRepository = (*OutboxRepository)(nil)

func NewOutboxRepository(db *gorm.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) getDB(ctx context.Context) *gorm.DB {
	if tx := GetTx(ctx); tx != nil {
		return tx
	}
	return r.db.WithContext(ctx)
}

func (r *OutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]*port.OutboxRecord, error) {
	var records []outboxRecord
	err := r.getDB(ctx).
		Where("published = ?", false).
		Order("created_at ASC").
		Limit(limit).
		Find(&records).
		Error

	if err != nil {
		return nil, fmt.Errorf("gorm fetch unpublished outbox events: %w", err)
	}

	results := make([]*port.OutboxRecord, len(records))
	for i, rec := range records {
		results[i] = &port.OutboxRecord{
			ID:           rec.ID,
			EventID:      rec.EventID,
			EventType:    rec.EventType,
			Payload:      rec.Payload,
			PartitionKey: rec.PartitionKey,
		}
	}
	return results, nil
}

func (r *OutboxRepository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	err := r.getDB(ctx).
		Model(&outboxRecord{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"published":    true,
			"published_at": time.Now(),
		}).
		Error

	if err != nil {
		return fmt.Errorf("gorm mark outbox events published: %w", err)
	}
	return nil
}

func (r *OutboxRepository) ExecuteInLock(
	ctx context.Context,
	lockID int64,
	fn func(ctx context.Context) error,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var acquired bool
		err := tx.Raw("SELECT pg_try_advisory_xact_lock(?)", lockID).Scan(&acquired).Error
		if err != nil {
			return fmt.Errorf("try advisory lock: %w", err)
		}
		if !acquired {
			return nil
		}

		txCtx := WithTx(ctx, tx)
		return fn(txCtx)
	})
}
