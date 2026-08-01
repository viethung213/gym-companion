package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/profile/application/port"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GormOutboxRepository struct {
	db *gorm.DB
}

var _ port.OutboxRepository = (*GormOutboxRepository)(nil)

func NewGormOutboxRepository(db *gorm.DB) *GormOutboxRepository {
	return &GormOutboxRepository{db: db}
}

func (r *GormOutboxRepository) getDB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return r.db.WithContext(ctx)
}

func (r *GormOutboxRepository) Save(ctx context.Context, record *port.OutboxRecord) error {
	m := &OutboxModel{
		ID:           record.ID,
		EventID:      record.EventID,
		EventType:    record.EventType,
		Payload:      record.Payload,
		PartitionKey: record.PartitionKey,
		CreatedAt:    time.Now(),
		Published:    false,
	}
	db := r.getDB(ctx)
	if err := db.Create(m).Error; err != nil {
		return fmt.Errorf("create outbox record: %w", err)
	}
	return nil
}

func (r *GormOutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]*port.OutboxRecord, error) {
	db := r.getDB(ctx)
	var models []*OutboxModel
	if err := db.Where("published = ?", false).Order("created_at ASC").Limit(limit).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("fetch unpublished outbox events: %w", err)
	}

	records := make([]*port.OutboxRecord, 0, len(models))
	for _, m := range models {
		records = append(records, &port.OutboxRecord{
			ID:           m.ID,
			EventID:      m.EventID,
			EventType:    m.EventType,
			Payload:      m.Payload,
			PartitionKey: m.PartitionKey,
		})
	}

	return records, nil
}

func (r *GormOutboxRepository) MarkAsPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	db := r.getDB(ctx)
	now := time.Now()
	err := db.Model(&OutboxModel{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"published":    true,
			"published_at": sql.NullTime{Time: now, Valid: true},
		}).Error
	if err != nil {
		return fmt.Errorf("mark outbox records as published: %w", err)
	}
	return nil
}

func (r *GormOutboxRepository) ProcessBatch(
	ctx context.Context,
	limit int,
	publishFn func(ctx context.Context, records []*port.OutboxRecord) error,
) error {
	if limit <= 0 {
		limit = 100
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var models []*OutboxModel
		err := tx.
			Clauses(clause.Locking{
				Strength: "UPDATE",
				Options:  "SKIP LOCKED",
			}).
			Where("published = ?", false).
			Order("created_at ASC").
			Limit(limit).
			Find(&models).
			Error

		if err != nil {
			return fmt.Errorf("fetch unpublished outbox events for update: %w", err)
		}

		if len(models) == 0 {
			return nil
		}

		records := make([]*port.OutboxRecord, 0, len(models))
		ids := make([]string, 0, len(models))
		for _, m := range models {
			records = append(records, &port.OutboxRecord{
				ID:           m.ID,
				EventID:      m.EventID,
				EventType:    m.EventType,
				Payload:      m.Payload,
				PartitionKey: m.PartitionKey,
			})
			ids = append(ids, m.ID)
		}

		txCtx := context.WithValue(ctx, txKey{}, tx)
		if pubErr := publishFn(txCtx, records); pubErr != nil {
			return fmt.Errorf("publish outbox batch: %w", pubErr)
		}

		now := time.Now()
		err = tx.Model(&OutboxModel{}).
			Where("id IN ?", ids).
			Updates(map[string]interface{}{
				"published":    true,
				"published_at": sql.NullTime{Time: now, Valid: true},
			}).
			Error

		if err != nil {
			return fmt.Errorf("mark outbox records as published: %w", err)
		}

		return nil
	})
}
