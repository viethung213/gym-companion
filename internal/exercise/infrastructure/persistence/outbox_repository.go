package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func (r *OutboxRepository) Save(ctx context.Context, rec *port.OutboxRecord) error {
	obr := outboxRecord{
		ID:           rec.ID,
		EventID:      rec.EventID,
		EventType:    rec.EventType,
		Payload:      rec.Payload,
		PartitionKey: rec.PartitionKey,
		CreatedAt:    time.Now(),
		Published:    false,
	}
	return r.getDB(ctx).Create(&obr).Error
}

func (r *OutboxRepository) FetchUnpublished(
	ctx context.Context,
	limit int,
) ([]*port.OutboxRecord, error) {
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
	for i := range records {
		rec := &records[i]
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
			"status":       "PUBLISHED",
		}).
		Error

	if err != nil {
		return fmt.Errorf("gorm mark outbox events published: %w", err)
	}
	return nil
}

func (r *OutboxRepository) ClaimBatch(
	ctx context.Context,
	limit int,
	lockDuration time.Duration,
) ([]*port.OutboxRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if lockDuration <= 0 {
		lockDuration = 30 * time.Second
	}

	var results []*port.OutboxRecord
	now := time.Now()
	lockedUntil := now.Add(lockDuration)

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var records []outboxRecord
		err := tx.
			Clauses(clause.Locking{
				Strength: "UPDATE",
				Options:  "SKIP LOCKED",
			}).
			Where("published = ? AND (status = ? OR locked_until IS NULL OR locked_until < ?)", false, "PENDING", now).
			Order("created_at ASC").
			Limit(limit).
			Find(&records).
			Error

		if err != nil {
			return fmt.Errorf("gorm fetch outbox for update skip locked: %w", err)
		}

		if len(records) == 0 {
			return nil
		}

		results = make([]*port.OutboxRecord, len(records))
		ids := make([]string, len(records))
		for i := range records {
			rec := &records[i]
			results[i] = &port.OutboxRecord{
				ID:           rec.ID,
				EventID:      rec.EventID,
				EventType:    rec.EventType,
				Payload:      rec.Payload,
				PartitionKey: rec.PartitionKey,
			}
			ids[i] = rec.ID
		}

		err = tx.Model(&outboxRecord{}).
			Where("id IN ?", ids).
			Updates(map[string]interface{}{
				"status":       "PROCESSING",
				"locked_until": lockedUntil,
			}).
			Error

		if err != nil {
			return fmt.Errorf("gorm claim outbox events: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return results, nil
}

func (r *OutboxRepository) ProcessBatch(
	ctx context.Context,
	limit int,
	publishFn func(ctx context.Context, records []*port.OutboxRecord) error,
) error {
	records, err := r.ClaimBatch(ctx, limit, 30*time.Second)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}

	if pubErr := publishFn(ctx, records); pubErr != nil {
		return fmt.Errorf("publish outbox batch: %w", pubErr)
	}

	ids := make([]string, len(records))
	for i, rec := range records {
		ids[i] = rec.ID
	}

	return r.MarkPublished(ctx, ids)
}
