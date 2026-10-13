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

// GormOutboxRepository implements port.OutboxRepository using GORM.
type GormOutboxRepository struct {
	db *gorm.DB
}

// Compile-time interface check.
var _ port.OutboxRepository = (*GormOutboxRepository)(nil)

// NewGormOutboxRepository creates a new GormOutboxRepository.
func NewGormOutboxRepository(db *gorm.DB) *GormOutboxRepository {
	return &GormOutboxRepository{db: db}
}

// Save stores a new OutboxRecord into the outbox table.
func (r *GormOutboxRepository) Save(ctx context.Context, record *port.OutboxRecord) error {
	if record == nil {
		return errors.New("nil outbox record")
	}

	id := record.ID
	if id == "" {
		id = uuid.NewString()
	}
	eventID := record.EventID
	if eventID == "" {
		eventID = uuid.NewString()
	}

	model := &OutboxModel{
		ID:            id,
		EventID:       eventID,
		AggregateType: record.AggregateType,
		AggregateID:   record.AggregateID,
		EventType:     record.EventType,
		Payload:       record.Payload,
		PartitionKey:  record.PartitionKey,
		Published:     false,
		CreatedAt:     time.Now().UTC(),
		Status:        "PENDING",
		MaxRetries:    5,
	}

	db := GetDB(ctx, r.db)
	err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "event_id"}},
		DoNothing: true,
	}).Create(model).Error
	if err != nil {
		return fmt.Errorf("create outbox record: %w", err)
	}

	return nil
}

// ClaimBatch selects unpublished records using SELECT ... FOR UPDATE SKIP LOCKED
// and sets their status to PROCESSING with a locked_until lease.
func (r *GormOutboxRepository) ClaimBatch(
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

	db := GetDB(ctx, r.db)
	now := time.Now().UTC()
	lockedUntil := now.Add(lockDuration)

	var claimedModels []*OutboxModel
	err := db.Transaction(func(tx *gorm.DB) error {
		var models []*OutboxModel

		// SELECT ... FOR UPDATE SKIP LOCKED prevents lock contention across multiple workers.
		err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("published = ? AND (status = 'PENDING' OR locked_until IS NULL OR locked_until < ?)", false, now).
			Order("created_at ASC").
			Limit(limit).
			Find(&models).Error
		if err != nil {
			return err
		}

		if len(models) == 0 {
			return nil
		}

		ids := make([]string, len(models))
		for i, m := range models {
			ids[i] = m.ID
		}

		if err := tx.Model(&OutboxModel{}).
			Where("id IN ?", ids).
			Updates(map[string]any{
				"status":       "PROCESSING",
				"locked_until": lockedUntil,
			}).Error; err != nil {
			return err
		}

		claimedModels = models
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("claim batch outbox events: %w", err)
	}

	records := make([]*port.OutboxRecord, 0, len(claimedModels))
	for _, m := range claimedModels {
		records = append(records, &port.OutboxRecord{
			ID:            m.ID,
			EventID:       m.EventID,
			AggregateType: m.AggregateType,
			AggregateID:   m.AggregateID,
			EventType:     m.EventType,
			Payload:       m.Payload,
			PartitionKey:  m.PartitionKey,
			Published:     m.Published,
			CreatedAt:     m.CreatedAt,
			PublishedAt:   m.PublishedAt,
		})
	}

	return records, nil
}

// MarkAsPublished marks records as PUBLISHED.
func (r *GormOutboxRepository) MarkAsPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	now := time.Now().UTC()
	db := GetDB(ctx, r.db)
	err := db.Model(&OutboxModel{}).
		Where("id IN ?", ids).
		Updates(map[string]any{
			"published":    true,
			"published_at": &now,
			"status":       "PUBLISHED",
			"locked_until": nil,
		}).Error
	if err != nil {
		return fmt.Errorf("mark outbox records as published: %w", err)
	}

	return nil
}

// MarkAsFailed increments retry_count, records last_error, resets locked_until,
// and moves records to DEAD_LETTER if retry_count >= max_retries (Poison Pill Guard).
func (r *GormOutboxRepository) MarkAsFailed(ctx context.Context, ids []string, lastErr error) error {
	if len(ids) == 0 {
		return nil
	}

	errMsg := ""
	if lastErr != nil {
		errMsg = lastErr.Error()
	}

	db := GetDB(ctx, r.db)
	err := db.Model(&OutboxModel{}).
		Where("id IN ?", ids).
		Updates(map[string]any{
			"retry_count":  gorm.Expr("retry_count + 1"),
			"last_error":   errMsg,
			"locked_until": nil,
			"status": gorm.Expr(
				"CASE WHEN retry_count + 1 >= max_retries THEN 'DEAD_LETTER' ELSE 'PENDING' END",
			),
		}).Error
	if err != nil {
		return fmt.Errorf("mark outbox records as failed: %w", err)
	}

	return nil
}

// ProcessBatch claims a batch, passes it to publishFn, and marks claimed records as published or failed.
func (r *GormOutboxRepository) ProcessBatch(
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

	ids := make([]string, len(records))
	for i, rec := range records {
		ids[i] = rec.ID
	}

	if pubErr := publishFn(ctx, records); pubErr != nil {
		if failErr := r.MarkAsFailed(ctx, ids, pubErr); failErr != nil {
			return fmt.Errorf("publish outbox batch failed: %w; also failed to mark failed records: %v", pubErr, failErr)
		}
		return fmt.Errorf("publish outbox batch: %w", pubErr)
	}

	return r.MarkAsPublished(ctx, ids)
}
