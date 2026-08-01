package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// OutboxRepository implements port.OutboxRepository.
type OutboxRepository struct {
	db *gorm.DB
}

var _ port.OutboxRepository = (*OutboxRepository)(nil)

// NewOutboxRepository constructs the outbox adapter.
func NewOutboxRepository(db *gorm.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) getDB(ctx context.Context) *gorm.DB {
	if tx := GetTx(ctx); tx != nil {
		return tx
	}

	return r.db.WithContext(ctx)
}

// Save persists one outbox record.
func (r *OutboxRepository) Save(ctx context.Context, rec *port.OutboxRecord) error {
	if rec == nil {
		return errors.New("nil outbox record")
	}

	obr := outboxRecord{
		ID: rec.ID,

		EventID: rec.EventID,

		EventType: rec.EventType,

		Payload: rec.Payload,

		PartitionKey: rec.PartitionKey,

		CreatedAt: rec.CreatedAt,

		Published: rec.Published,
	}

	if err := r.getDB(ctx).Create(&obr).Error; err != nil {
		return fmt.Errorf("insert outbox: %w", err)
	}

	return nil
}

// FetchUnpublished returns unpublished events ordered by created_at ASC.
func (r *OutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]*port.OutboxRecord, error) {
	if limit <= 0 {
		limit = 100
	}

	var recs []outboxRecord

	if err := r.getDB(ctx).
		Where("published = ?", false).
		Order("created_at ASC").
		Limit(limit).
		Find(&recs).Error; err != nil {
		return nil, fmt.Errorf("fetch unpublished: %w", err)
	}

	out := make([]*port.OutboxRecord, 0, len(recs))

	for i := range recs {
		out = append(out, &port.OutboxRecord{
			ID:           recs[i].ID,
			EventID:      recs[i].EventID,
			EventType:    recs[i].EventType,
			Payload:      recs[i].Payload,
			PartitionKey: recs[i].PartitionKey,
			CreatedAt:    recs[i].CreatedAt,
			Published:    recs[i].Published,
		})
	}

	return out, nil
}

// MarkPublished flips published=true for the given ids.
func (r *OutboxRepository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	err := r.getDB(ctx).
		Model(&outboxRecord{}).
		Where("id IN ?", ids).
		Updates(map[string]any{
			"published":    true,
			"published_at": time.Now(),
			"status":       "PUBLISHED",
		}).Error

	if err != nil {
		return fmt.Errorf("mark published: %w", err)
	}

	return nil
}

// ClaimBatch claims unpublished outbox events using FOR UPDATE SKIP LOCKED,
// updates status='PROCESSING' and locked_until=NOW()+lockDuration in a short transaction,
// and returns the claimed records.
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

	var records []*port.OutboxRecord
	now := time.Now()
	lockedUntil := now.Add(lockDuration)

	err := r.getDB(ctx).Transaction(func(tx *gorm.DB) error {
		var dbOutboxes []outboxRecord
		err := tx.
			Clauses(clause.Locking{
				Strength: "UPDATE",
				Options:  "SKIP LOCKED",
			}).
			Where("published = ? AND (status = ? OR locked_until IS NULL OR locked_until < ?)", false, "PENDING", now).
			Order("created_at ASC").
			Limit(limit).
			Find(&dbOutboxes).
			Error

		if err != nil {
			return fmt.Errorf("fetch outbox for update skip locked: %w", err)
		}

		if len(dbOutboxes) == 0 {
			return nil
		}

		records = make([]*port.OutboxRecord, len(dbOutboxes))
		ids := make([]string, len(dbOutboxes))
		for i := range dbOutboxes {
			rec := &dbOutboxes[i]
			var pubAt *time.Time
			if rec.PublishedAt.Valid {
				pubAt = &rec.PublishedAt.Time
			}
			records[i] = &port.OutboxRecord{
				ID:           rec.ID,
				EventID:      rec.EventID,
				EventType:    rec.EventType,
				Payload:      rec.Payload,
				PartitionKey: rec.PartitionKey,
				CreatedAt:    rec.CreatedAt,
				Published:    rec.Published,
				PublishedAt:  pubAt,
			}
			ids[i] = rec.ID
		}

		err = tx.Model(&outboxRecord{}).
			Where("id IN ?", ids).
			Updates(map[string]any{
				"status":       "PROCESSING",
				"locked_until": lockedUntil,
			}).
			Error

		if err != nil {
			return fmt.Errorf("claim outbox events: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return records, nil
}

// ProcessBatch claims unpublished outbox events in a short transaction,
// publishes them via publishFn OUTSIDE the database transaction,
// and marks them as published upon success.
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

// LogProcessed records an inbound event in coaching.outbox_log. Returns
// (fresh=true, nil) on first record, (fresh=false, nil) on duplicate event_id (D9).
func (r *OutboxRepository) LogProcessed(ctx context.Context, eventID, eventType, partitionKey string, payload []byte) (bool, error) {
	rec := outboxLogRecord{
		ID:           eventID, // Use event_id as PK to make retries idempotent.
		EventID:      eventID,
		EventType:    eventType,
		Payload:      payload,
		PartitionKey: partitionKey,
		ProcessedAt:  time.Now(),
		Status:       "SUCCESS",
	}

	res := r.getDB(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "event_id"}},
		DoNothing: true,
	}).Create(&rec)

	if err := res.Error; err != nil {
		return false, fmt.Errorf("log processed: %w", err)
	}

	return res.RowsAffected > 0, nil
}
