package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/viethung213/gym-companion/internal/profile/application/port"
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
