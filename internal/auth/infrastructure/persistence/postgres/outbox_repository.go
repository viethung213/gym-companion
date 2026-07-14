package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"gorm.io/gorm"
)

// OutboxRepository implements port.OutboxRepository using GORM over PostgreSQL.
type OutboxRepository struct {
	db *gorm.DB
}

// Compile-time interface verification
var _ port.OutboxRepository = (*OutboxRepository)(nil)

// NewOutboxRepository creates a new instance of OutboxRepository.
func NewOutboxRepository(db *gorm.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) getDB(ctx context.Context) *gorm.DB {
	if tx := GetTx(ctx); tx != nil {
		return tx
	}
	return r.db.WithContext(ctx)
}

// SaveEvent inserts a new event log entry directly into the auth.outbox table.
func (r *OutboxRepository) SaveEvent(ctx context.Context, eventID string, eventType string, payload []byte, partitionKey string) error {
	dbOutbox := &OutboxModel{
		ID:           uuid.New().String(),
		EventID:      eventID,
		EventType:    eventType,
		Payload:      payload,
		PartitionKey: partitionKey,
		CreatedAt:    time.Now(),
		Published:    false,
	}
	err := r.getDB(ctx).Create(dbOutbox).Error
	if err != nil {
		return fmt.Errorf("gorm save outbox event: %w", err)
	}
	return nil
}

// FetchUnpublished queries all unpublished outbox entries up to the given limit.
func (r *OutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]*port.OutboxRecord, error) {
	var dbOutboxes []OutboxModel
	err := r.getDB(ctx).
		Where("published = ?", false).
		Order("created_at ASC").
		Limit(limit).
		Find(&dbOutboxes).
		Error

	if err != nil {
		return nil, fmt.Errorf("gorm fetch unpublished outbox events: %w", err)
	}

	records := make([]*port.OutboxRecord, 0, len(dbOutboxes))
	for _, o := range dbOutboxes {
		records = append(records, o.ToRepositoryRecord())
	}
	return records, nil
}

// MarkPublished flags the given events as published with target timestamp in a single bulk update.
func (r *OutboxRepository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	err := r.getDB(ctx).
		Model(&OutboxModel{}).
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

// ExecuteInLock executes the provided function within a database transaction
// guarded by a PostgreSQL Advisory Lock.
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
			// Lock not acquired, silently return nil to skip this processing iteration
			return nil
		}

		txCtx := WithTx(ctx, tx)
		return fn(txCtx)
	})
}
